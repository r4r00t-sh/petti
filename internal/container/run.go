package container

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/r4r00t-sh/petti/internal/cgroups"
	"github.com/r4r00t-sh/petti/internal/image"
	"github.com/r4r00t-sh/petti/internal/network"
	"github.com/r4r00t-sh/petti/internal/rootfs"
	"github.com/r4r00t-sh/petti/internal/rootless"
	"github.com/r4r00t-sh/petti/internal/runtime"
	"github.com/r4r00t-sh/petti/internal/seccomp"
)

// RunOptions configures `petti run`.
type RunOptions struct {
	Rootfs      string
	Image       string
	Name        string
	Hostname    string
	Workdir     string
	Env         []string
	User        string
	Entrypoint  []string
	Args        []string
	Volumes     []Volume
	Memory      string
	CPUs        string
	PidsLimit   string
	Detach      bool
	AutoRemove  bool
	Tty         bool
	Interactive bool
	Network     string
	Ports       []network.PortMapping
	Rootless    bool // force rootless even as root; auto when non-root
	CapAdd      []string
	CapDrop     []string
	Seccomp     seccomp.Mode
	ReadOnly    bool
	Stdin       io.Reader
	Stdout      io.Writer
	Stderr      io.Writer
}

// RunResult is returned after a successful start (including detach).
type RunResult struct {
	ID   string
	Name string
}

// Run starts a container. For detach, returns after the process is running.
func Run(opts RunOptions) (RunResult, error) {
	var out RunResult
	if opts.Rootfs == "" && opts.Image == "" {
		return out, fmt.Errorf("provide an image name or --rootfs")
	}
	if opts.Rootfs != "" && opts.Image != "" {
		return out, fmt.Errorf("use either an image name or --rootfs, not both")
	}

	isRootless := rootless.Enabled(opts.Rootless)
	warnOut := opts.Stderr
	if warnOut == nil {
		warnOut = os.Stderr
	}

	store, err := OpenStore("")
	if err != nil {
		return out, err
	}

	id, err := GenerateID()
	if err != nil {
		return out, err
	}
	out.ID = id
	out.Name = opts.Name

	if opts.Name != "" {
		if err := store.bindName(opts.Name, id); err != nil {
			return out, err
		}
	}

	bundle := store.BundleDir(id)
	if err := os.MkdirAll(bundle, 0o755); err != nil {
		return out, err
	}

	var (
		rootfsPath = opts.Rootfs
		workdir    = opts.Workdir
		userStr    = opts.User
		env        = append([]string(nil), opts.Env...)
		entrypoint = append([]string(nil), opts.Entrypoint...)
		args       = append([]string(nil), opts.Args...)
		overlay    *rootfs.Overlay
	)

	if opts.Image != "" {
		imgStore, err := image.OpenStore(store.Layout.Root)
		if err != nil {
			return out, err
		}
		rec, err := imgStore.Resolve(opts.Image)
		if err != nil {
			return out, err
		}
		var lowers []string
		for _, layer := range rec.Layers {
			hex := strings.TrimPrefix(layer.DiffID, "sha256:")
			lowers = append(lowers, imgStore.Layout.LayerPath(hex))
		}
		ovDir := filepath.Join(bundle, "rootfs")
		overlay, err = mountRootfs(ovDir, lowers, isRootless, warnOut)
		if err != nil {
			_ = store.unbindName(opts.Name)
			return out, fmt.Errorf("rootfs: %w", err)
		}
		rootfsPath = overlay.Merged

		cfg := rec.Config.Config
		if len(entrypoint) == 0 && len(cfg.Entrypoint) > 0 {
			entrypoint = append([]string{}, cfg.Entrypoint...)
		}
		if len(args) == 0 && len(cfg.Cmd) > 0 {
			args = append([]string{}, cfg.Cmd...)
		}
		if workdir == "" && cfg.WorkingDir != "" {
			workdir = cfg.WorkingDir
		}
		if userStr == "" && cfg.User != "" {
			userStr = cfg.User
		}
		env = append(append([]string{}, cfg.Env...), opts.Env...)
	}

	finalArgs := runtime.MergeEntrypoint(entrypoint, args)
	if len(finalArgs) == 0 {
		cleanupOverlay(overlay)
		_ = store.unbindName(opts.Name)
		return out, fmt.Errorf("command is required (image has no Cmd/Entrypoint)")
	}

	user, err := runtime.ParseUser(userStr)
	if err != nil {
		cleanupOverlay(overlay)
		_ = store.unbindName(opts.Name)
		return out, fmt.Errorf("--user: %w (numeric uid[:gid] required)", err)
	}

	lim, err := parseLimits(opts.Memory, opts.CPUs, opts.PidsLimit)
	if err != nil {
		cleanupOverlay(overlay)
		_ = store.unbindName(opts.Name)
		return out, err
	}

	hostname := opts.Hostname
	if hostname == "" {
		hostname = HostnamePrefix(id)
	}
	if workdir == "" {
		workdir = "/"
	}

	logPath := filepath.Join(bundle, "container.log")
	stdin, stdout, stderr := opts.Stdin, opts.Stdout, opts.Stderr
	if stdin == nil {
		stdin = os.Stdin
	}
	if stdout == nil {
		stdout = os.Stdout
	}
	if stderr == nil {
		stderr = os.Stderr
	}

	var logFile *os.File
	if opts.Detach {
		logFile, err = os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
		if err != nil {
			cleanupOverlay(overlay)
			return out, err
		}
		// Must pass *os.File so exec does not spawn a copy goroutine that dies on detach.
		stdout = logFile
		stderr = logFile
		devnull, err := os.OpenFile(os.DevNull, os.O_RDONLY, 0)
		if err != nil {
			_ = logFile.Close()
			cleanupOverlay(overlay)
			return out, err
		}
		stdin = devnull
		defer func() { _ = devnull.Close() }()
	}

	vols := make([]runtime.VolumeSpec, 0, len(opts.Volumes))
	for _, v := range opts.Volumes {
		vols = append(vols, runtime.VolumeSpec{Source: v.Source, Target: v.Target, ReadOnly: v.ReadOnly})
	}

	netArg := opts.Network
	if isRootless && netArg == "" {
		netArg = network.ModeHost // bridge needs CAP_NET_ADMIN on the host
	}
	netMode, err := network.ParseMode(netArg)
	if err != nil {
		cleanupOverlay(overlay)
		_ = store.unbindName(opts.Name)
		return out, err
	}
	if isRootless && netMode == network.ModeBridge {
		cleanupOverlay(overlay)
		_ = store.unbindName(opts.Name)
		return out, fmt.Errorf("rootless mode does not support --network bridge (use host or none)")
	}
	if netMode != network.ModeBridge && len(opts.Ports) > 0 {
		cleanupOverlay(overlay)
		_ = store.unbindName(opts.Name)
		return out, fmt.Errorf("-p requires --network bridge")
	}

	// Pre-allocate IP and bind-mount DNS files so we never mutate a shared --rootfs.
	var allocatedIP string
	switch netMode {
	case network.ModeBridge:
		ip, err := network.AllocateIP(store.Layout.Root, id)
		if err != nil {
			cleanupOverlay(overlay)
			_ = store.unbindName(opts.Name)
			return out, err
		}
		allocatedIP = ip.String()
		hosts, resolv, err := network.WriteDNSFiles(filepath.Join(bundle, "dns"), hostname, allocatedIP)
		if err != nil {
			_ = network.ReleaseIP(store.Layout.Root, id)
			cleanupOverlay(overlay)
			_ = store.unbindName(opts.Name)
			return out, err
		}
		vols = append(vols,
			runtime.VolumeSpec{Source: hosts, Target: "/etc/hosts", ReadOnly: true},
			runtime.VolumeSpec{Source: resolv, Target: "/etc/resolv.conf", ReadOnly: true},
		)
	case network.ModeNone:
		hosts, resolv, err := network.WriteDNSFiles(filepath.Join(bundle, "dns"), hostname, "127.0.0.1")
		if err == nil {
			vols = append(vols,
				runtime.VolumeSpec{Source: hosts, Target: "/etc/hosts", ReadOnly: true},
				runtime.VolumeSpec{Source: resolv, Target: "/etc/resolv.conf", ReadOnly: true},
			)
		}
	}

	now := time.Now().UTC()
	rec := Record{
		ID:          id,
		Name:        opts.Name,
		Image:       opts.Image,
		Status:      StatusCreated,
		CreatedAt:   now,
		Hostname:    hostname,
		Workdir:     workdir,
		Env:         env,
		User:        userStr,
		Command:     finalArgs,
		Volumes:     opts.Volumes,
		Detach:      opts.Detach,
		AutoRemove:  opts.AutoRemove,
		Tty:         opts.Tty,
		Memory:      opts.Memory,
		CPUs:        opts.CPUs,
		PidsLimit:   opts.PidsLimit,
		Rootfs:      rootfsPath,
		BundleDir:   bundle,
		LogPath:     logPath,
		NetworkMode: netMode,
		Ports:       opts.Ports,
		IP:          allocatedIP,
		Gateway:     "",
		Rootless:    isRootless,
	}
	if allocatedIP != "" {
		rec.Gateway = network.BridgeIP
	}
	if err := store.Save(rec); err != nil {
		cleanupOverlay(overlay)
		if logFile != nil {
			_ = logFile.Close()
		}
		return out, err
	}

	cfg := runtime.Config{
		ID:          id,
		Rootfs:      rootfsPath,
		Hostname:    hostname,
		Workdir:     workdir,
		Env:         env,
		User:        user,
		Args:        finalArgs,
		Volumes:     vols,
		Resources:   lim,
		Detach:      opts.Detach,
		NetworkMode: netMode,
		NetworkRoot: store.Layout.Root,
		Ports:       opts.Ports,
		Rootless:    isRootless,
		CapAdd:      opts.CapAdd,
		CapDrop:     opts.CapDrop,
		Seccomp:     opts.Seccomp,
		ReadOnly:    opts.ReadOnly,
		Stdin:       stdin,
		Stdout:      stdout,
		Stderr:      stderr,
	}

	proc, err := runtime.Start(cfg)
	if logFile != nil {
		_ = logFile.Close()
	}
	if err != nil {
		rec.Status = StatusExited
		var ee *runtime.ExitError
		if errors.As(err, &ee) {
			rec.ExitCode = ee.Code
		}
		_ = store.Save(rec)
		if allocatedIP != "" {
			_ = network.ReleaseIP(store.Layout.Root, id)
		}
		cleanupOverlay(overlay)
		if opts.AutoRemove {
			_ = store.Delete(id)
		}
		return out, err
	}

	rec.Status = StatusRunning
	rec.Pid = proc.Pid
	rec.StartedAt = time.Now().UTC()
	if proc.Cgroup != nil {
		rec.CgroupPath = proc.Cgroup.Path
	}
	if proc.Network != nil {
		rec.IP = proc.Network.IP
		rec.Gateway = proc.Network.Gateway
		rec.HostVeth = proc.Network.HostVeth
		rec.NetworkMode = proc.Network.Mode
		rec.Ports = proc.Network.Ports
	}
	_ = store.Save(rec)

	if opts.Detach {
		return out, nil
	}

	waitErr := proc.Wait()
	rec.Status = StatusExited
	rec.FinishedAt = time.Now().UTC()
	var ee *runtime.ExitError
	if errors.As(waitErr, &ee) {
		rec.ExitCode = ee.Code
	} else if waitErr == nil {
		rec.ExitCode = 0
	}
	_ = store.Save(rec)

	cleanupOverlay(overlay)
	if opts.AutoRemove {
		_ = store.Delete(id)
	}
	return out, waitErr
}

func cleanupOverlay(o *rootfs.Overlay) {
	if o == nil {
		return
	}
	_ = o.Unmount()
}

// mountRootfs mounts overlay (privileged or userxattr) or copy-merges layers.
func mountRootfs(ovDir string, lowers []string, isRootless bool, warnOut io.Writer) (*rootfs.Overlay, error) {
	if !isRootless {
		return rootfs.MountOverlay(ovDir, lowers)
	}
	o, err := rootfs.MountOverlayUserxattr(ovDir, lowers)
	if err == nil {
		return o, nil
	}
	if warnOut != nil {
		_, _ = fmt.Fprintf(warnOut, "petti: rootless overlay unavailable (%v); copying layers\n", err)
	}
	o, err = rootfs.NewCopiedRootfs(ovDir)
	if err != nil {
		return nil, err
	}
	if err := image.MergeLayers(lowers, o.Merged); err != nil {
		return nil, fmt.Errorf("copy rootfs: %w", err)
	}
	return o, nil
}

func parseLimits(memory, cpus, pids string) (cgroups.Limits, error) {
	var lim cgroups.Limits
	if memory != "" {
		b, err := cgroups.ParseBytes(memory)
		if err != nil {
			return lim, fmt.Errorf("--memory: %w", err)
		}
		lim.MemoryBytes = b
	}
	if cpus != "" {
		v, err := cgroups.ParseCPUs(cpus)
		if err != nil {
			return lim, fmt.Errorf("--cpus: %w", err)
		}
		lim.CPUs = v
	}
	if pids != "" {
		v, err := cgroups.ParsePids(pids)
		if err != nil {
			return lim, fmt.Errorf("--pids-limit: %w", err)
		}
		lim.Pids = v
	}
	return lim, nil
}
