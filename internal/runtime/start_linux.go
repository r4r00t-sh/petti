//go:build linux

package runtime

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"syscall"
	"testing"

	"golang.org/x/sys/unix"

	"github.com/r4r00t-sh/petti/internal/cgroups"
	"github.com/r4r00t-sh/petti/internal/network"
	"github.com/r4r00t-sh/petti/internal/rootfs"
	"github.com/r4r00t-sh/petti/internal/rootless"
	"github.com/r4r00t-sh/petti/internal/seccomp"
)

// baseNamespaceFlags: PID, mount, UTS, IPC. CLONE_NEWNET added for bridge/none.
const baseNamespaceFlags = unix.CLONE_NEWNS | unix.CLONE_NEWUTS | unix.CLONE_NEWPID | unix.CLONE_NEWIPC

// Process is a started container init.
type Process struct {
	Pid     int
	Cmd     *exec.Cmd
	Cgroup  *cgroups.Manager
	Network *network.Endpoint
	cfg     Config
}

// Start re-execs the current binary as "__init" inside new namespaces and
// returns a Process. The caller must Wait (or manage Detach lifetime) and owns
// cgroup/network cleanup via Wait.
func Start(cfg Config) (*Process, error) {
	cfg.DefaultIO()

	if cfg.Rootfs == "" {
		return nil, fmt.Errorf("rootfs is required")
	}
	rootfsPath, err := filepath.Abs(cfg.Rootfs)
	if err != nil {
		return nil, fmt.Errorf("resolve rootfs: %w", err)
	}
	info, err := os.Stat(rootfsPath)
	if err != nil {
		return nil, fmt.Errorf("rootfs %q: %w", rootfsPath, err)
	}
	if !info.IsDir() {
		return nil, fmt.Errorf("rootfs %q is not a directory", rootfsPath)
	}
	cfg.Rootfs = rootfsPath

	if len(cfg.Args) == 0 {
		return nil, fmt.Errorf("command is required")
	}
	if cfg.Hostname == "" {
		return nil, fmt.Errorf("hostname is required")
	}
	if cfg.Workdir == "" {
		cfg.Workdir = "/"
	}

	self, err := resolveInitBinary()
	if err != nil {
		return nil, err
	}

	var cg *cgroups.Manager
	var idMap rootless.Mapping
	// Defer pids.max until __init is about to exec — the Go runtime needs
	// several OS threads and would SIGABRT under a tight pids limit.
	startLim := cfg.Resources
	deferredPids := startLim.Pids
	startLim.Pids = 0
	if cfg.Rootless {
		idMap, err = rootless.BuildMapping()
		if err != nil {
			return nil, fmt.Errorf("rootless id map: %w", err)
		}
		var warn string
		cg, warn, err = cgroups.CreateRootless(cfg.ID, startLim)
		if err != nil {
			return nil, fmt.Errorf("cgroup: %w", err)
		}
		if warn != "" && cfg.Stderr != nil {
			_, _ = fmt.Fprintf(cfg.Stderr, "petti: %s\n", warn)
		}
	} else if startLim.Set() || deferredPids > 0 {
		cg, err = cgroups.Create(cfg.CgroupMount, cfg.ID, startLim)
		if err != nil {
			return nil, fmt.Errorf("cgroup: %w", err)
		}
	}
	if cg != nil {
		cg.Lim.Pids = deferredPids
	}

	r, w, err := os.Pipe()
	if err != nil {
		if cg != nil {
			_ = cg.Destroy()
		}
		return nil, fmt.Errorf("sync pipe: %w", err)
	}

	initArgs := []string{
		"__init",
		"--rootfs", cfg.Rootfs,
		"--hostname", cfg.Hostname,
		"--workdir", cfg.Workdir,
		"--sync-fd", "3",
	}
	for _, e := range cfg.Env {
		initArgs = append(initArgs, "--env", e)
	}
	if cfg.User.Set {
		initArgs = append(initArgs, "--user", fmt.Sprintf("%d:%d", cfg.User.UID, cfg.User.GID))
	}
	for _, v := range cfg.Volumes {
		mode := "rw"
		if v.ReadOnly {
			mode = "ro"
		}
		initArgs = append(initArgs, "--mount", fmt.Sprintf("%s:%s:%s", v.Source, v.Target, mode))
	}
	for _, c := range cfg.CapAdd {
		initArgs = append(initArgs, "--cap-add", c)
	}
	for _, c := range cfg.CapDrop {
		initArgs = append(initArgs, "--cap-drop", c)
	}
	secMode := cfg.Seccomp
	if secMode == "" {
		secMode = seccomp.ModeDefault
	}
	initArgs = append(initArgs, "--seccomp", string(secMode))
	if cfg.ReadOnly {
		initArgs = append(initArgs, "--read-only")
	}

	netMode, err := network.ParseMode(cfg.NetworkMode)
	if err != nil {
		if cg != nil {
			_ = cg.Destroy()
		}
		_ = r.Close()
		_ = w.Close()
		return nil, err
	}
	cfg.NetworkMode = netMode
	initArgs = append(initArgs, "--network", netMode)
	if deferredPids > 0 && cg != nil {
		initArgs = append(initArgs,
			"--cgroup-path", cg.Path,
			"--pids-limit", strconv.FormatInt(deferredPids, 10),
		)
	}

	initArgs = append(initArgs, "--")
	initArgs = append(initArgs, cfg.Args...)

	cmd := exec.Command(self, initArgs...)
	cmd.Stdin = cfg.Stdin
	cmd.Stdout = cfg.Stdout
	cmd.Stderr = cfg.Stderr
	cmd.ExtraFiles = []*os.File{r}
	flags := uintptr(baseNamespaceFlags)
	if netMode != network.ModeHost {
		flags |= unix.CLONE_NEWNET
	}
	if cfg.Rootless {
		flags |= unix.CLONE_NEWUSER
	}
	sys := &syscall.SysProcAttr{Cloneflags: flags}
	// Pdeathsig would kill detached containers when `petti run -d` exits.
	if !cfg.Detach {
		sys.Pdeathsig = unix.SIGKILL
	}
	// Single-line maps: let Go write uid_map/gid_map before the child runs.
	// Multi-range (subuid) needs newuidmap after Start.
	if cfg.Rootless && !idMap.UseNewuidmap {
		sys.UidMappings, sys.GidMappings = idMap.SysProcMaps()
	}
	cmd.SysProcAttr = sys

	if err := cmd.Start(); err != nil {
		_ = r.Close()
		_ = w.Close()
		if cg != nil {
			_ = cg.Destroy()
		}
		if errors.Is(err, syscall.EPERM) || errors.Is(err, os.ErrPermission) || errors.Is(err, syscall.EACCES) {
			hint := "Petti run requires root for namespaces; try: sudo petti run ..."
			if cfg.Rootless {
				hint = rootlessStartHint()
			}
			return nil, fmt.Errorf("start init: %w (%s)", err, hint)
		}
		return nil, fmt.Errorf("start init: %w", err)
	}
	_ = r.Close()

	fail := func(err error) (*Process, error) {
		_ = w.Close()
		_ = cmd.Process.Kill()
		_, _ = cmd.Process.Wait()
		if cg != nil {
			_ = cg.Destroy()
		}
		return nil, err
	}

	if cfg.Rootless && idMap.UseNewuidmap {
		if err := idMap.Apply(cmd.Process.Pid); err != nil {
			return fail(fmt.Errorf("uid/gid map: %w", err))
		}
	}

	if cg != nil {
		if err := cg.AddProc(cmd.Process.Pid); err != nil {
			return fail(fmt.Errorf("cgroup attach: %w", err))
		}
	}

	ep, err := network.Setup(network.SetupOpts{
		ContainerID: cfg.ID,
		Pid:         cmd.Process.Pid,
		Hostname:    cfg.Hostname,
		Mode:        netMode,
		Ports:       cfg.Ports,
		DataRoot:    cfg.NetworkRoot,
	})
	if err != nil {
		return fail(fmt.Errorf("network: %w", err))
	}

	if _, err := w.Write([]byte{1}); err != nil {
		_ = network.Teardown(cfg.NetworkRoot, *ep)
		return fail(fmt.Errorf("sync signal: %w", err))
	}
	_ = w.Close()

	return &Process{Pid: cmd.Process.Pid, Cmd: cmd, Cgroup: cg, Network: ep, cfg: cfg}, nil
}

// Wait blocks until the process exits and destroys the cgroup / network endpoint.
func (p *Process) Wait() error {
	if p == nil || p.Cmd == nil {
		return nil
	}
	waitErr := p.Cmd.Wait()
	oom := false
	if p.Cgroup != nil {
		if killed, err := p.Cgroup.OOMKilled(); err == nil {
			oom = killed
		}
		_ = p.Cgroup.Destroy()
		p.Cgroup = nil
	}
	if p.Network != nil {
		_ = network.Teardown(p.cfg.NetworkRoot, *p.Network)
		p.Network = nil
	}
	if waitErr == nil {
		if oom {
			return &ExitError{Code: 137, OOM: true}
		}
		return nil
	}
	var ee *exec.ExitError
	if errors.As(waitErr, &ee) {
		code := ee.ExitCode()
		if status, ok := ee.Sys().(syscall.WaitStatus); ok {
			if status.Signaled() {
				code = 128 + int(status.Signal())
			} else {
				code = status.ExitStatus()
			}
		}
		if oom {
			code = 137
		}
		return &ExitError{Code: code, OOM: oom}
	}
	return fmt.Errorf("wait init: %w", waitErr)
}

// Init is the __init entrypoint. It runs inside the new namespaces.
func Init(cfg Config) error {
	runtimeLockOSThread()

	if err := waitSync(cfg.SyncFD); err != nil {
		return fmt.Errorf("sync: %w", err)
	}

	// ModeNone skips parent setns; bring lo up here. ModeBridge is already configured.
	if cfg.NetworkMode == network.ModeNone {
		if err := network.SetLoopbackUp(); err != nil {
			return fmt.Errorf("loopback: %w", err)
		}
	}

	if err := unix.Sethostname([]byte(cfg.Hostname)); err != nil {
		return fmt.Errorf("set hostname %q: %w", cfg.Hostname, err)
	}

	if err := setupMounts(cfg.Rootfs); err != nil {
		return fmt.Errorf("mounts: %w", err)
	}

	// Bind volumes onto the rootfs *before* pivot_root so host paths are visible.
	for _, v := range cfg.Volumes {
		target := filepath.Join(cfg.Rootfs, filepath.Clean("/"+v.Target))
		src, err := filepath.Abs(v.Source)
		if err != nil {
			return fmt.Errorf("volume source: %w", err)
		}
		if err := rootfs.BindMount(src, target, v.ReadOnly); err != nil {
			return fmt.Errorf("volume %s:%s: %w", v.Source, v.Target, err)
		}
	}

	// Host cgroupfs is still visible before pivot_root. Deferring until here
	// lets the Go __init keep its threads; forks after exec are limited.
	if cfg.PidsLimit > 0 {
		if err := cgroups.ApplyPidsMaxAt(cfg.CgroupPath, cfg.PidsLimit); err != nil {
			return fmt.Errorf("pids.max: %w", err)
		}
	}

	if err := pivotIntoRootfs(cfg.Rootfs); err != nil {
		return err
	}

	if cfg.ReadOnly {
		if err := remountRootReadOnly(); err != nil {
			return err
		}
	}

	if err := setNoNewPrivs(); err != nil {
		return err
	}
	keep := cfg.KeepCaps
	if keep == nil {
		var err error
		keep, err = ResolveCaps(cfg.CapAdd, cfg.CapDrop)
		if err != nil {
			return err
		}
	}
	if err := dropCapabilities(keep); err != nil {
		return fmt.Errorf("capabilities: %w", err)
	}
	secMode := cfg.Seccomp
	if secMode == "" {
		secMode = seccomp.ModeDefault
	}
	if err := seccomp.Apply(secMode); err != nil {
		return fmt.Errorf("seccomp: %w", err)
	}
	if err := applyUser(cfg.User); err != nil {
		return err
	}

	wd := cfg.Workdir
	if wd == "" {
		wd = "/"
	}
	if err := unix.Chdir(wd); err != nil {
		return fmt.Errorf("workdir %q: %w", wd, err)
	}

	if len(cfg.Args) == 0 {
		return fmt.Errorf("no command")
	}
	argv0, err := resolveArgv0(cfg.Args[0])
	if err != nil {
		return err
	}
	cfg.Args[0] = argv0

	env, err := MergeEnv(DefaultEnv(cfg.User), cfg.Env)
	if err != nil {
		return err
	}
	if err := unix.Exec(argv0, cfg.Args, env); err != nil {
		return fmt.Errorf("exec %s: %w", argv0, err)
	}
	return nil
}

func waitSync(fd int) error {
	if fd < 0 {
		return fmt.Errorf("invalid sync fd %d", fd)
	}
	f := os.NewFile(uintptr(fd), "sync")
	if f == nil {
		return fmt.Errorf("sync fd %d", fd)
	}
	defer f.Close()
	buf := make([]byte, 1)
	n, err := f.Read(buf)
	if err != nil {
		return err
	}
	if n != 1 {
		return fmt.Errorf("short sync read")
	}
	return nil
}

// ParseSyncFD converts the --sync-fd flag value.
func ParseSyncFD(s string) (int, error) {
	fd, err := strconv.Atoi(s)
	if err != nil {
		return -1, fmt.Errorf("sync-fd: %w", err)
	}
	return fd, nil
}

// resolveInitBinary returns the petti executable used for the __init re-exec.
// Under `go test`, /proc/self/exe is the test binary (no __init command), so we
// require PETTI_INIT or a `petti` binary on PATH.
func resolveInitBinary() (string, error) {
	if v := os.Getenv("PETTI_INIT"); v != "" {
		if _, err := os.Stat(v); err != nil {
			return "", fmt.Errorf("PETTI_INIT=%q: %w", v, err)
		}
		return v, nil
	}
	if testing.Testing() {
		if p, err := exec.LookPath("petti"); err == nil {
			return p, nil
		}
		return "", fmt.Errorf("running under go test: set PETTI_INIT to the petti binary (e.g. make integration)")
	}
	self, err := os.Executable()
	if err != nil {
		return "", fmt.Errorf("resolve executable: %w", err)
	}
	// Prefer the live inode so a replaced binary mid-flight still works.
	if _, err := os.Stat("/proc/self/exe"); err == nil {
		return "/proc/self/exe", nil
	}
	return self, nil
}
