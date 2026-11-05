//go:build linux

package runtime

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strconv"

	"golang.org/x/sys/unix"
)

// ExecConfig configures joining an existing container's namespaces.
type ExecConfig struct {
	TargetPid int
	Workdir   string
	User      UserSpec
	Env       []string
	Args      []string
}

// ExecInNamespaces is the `__exec` entrypoint: setns into target, then exec.
func ExecInNamespaces(cfg ExecConfig) error {
	runtime.LockOSThread()

	if cfg.TargetPid <= 0 {
		return fmt.Errorf("target pid required")
	}
	for _, ns := range []string{"mnt", "pid", "uts", "ipc", "net"} {
		path := filepath.Join("/proc", strconv.Itoa(cfg.TargetPid), "ns", ns)
		f, err := os.Open(path)
		if err != nil {
			// host network mode shares the host netns; still present under /proc.
			return fmt.Errorf("open %s: %w", path, err)
		}
		if err := unix.Setns(int(f.Fd()), 0); err != nil {
			_ = f.Close()
			return fmt.Errorf("setns %s: %w", ns, err)
		}
		_ = f.Close()
	}

	if err := applyUser(cfg.User); err != nil {
		return err
	}
	wd := cfg.Workdir
	if wd == "" {
		wd = "/"
	}
	if err := unix.Chdir(wd); err != nil {
		return fmt.Errorf("workdir: %w", err)
	}
	if len(cfg.Args) == 0 {
		return fmt.Errorf("command required")
	}
	argv0, err := resolveArgv0(cfg.Args[0])
	if err != nil {
		return err
	}
	env, err := MergeEnv(DefaultEnv(cfg.User), cfg.Env)
	if err != nil {
		return err
	}
	return unix.Exec(argv0, append([]string{argv0}, cfg.Args[1:]...), env)
}
