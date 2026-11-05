package cli

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/r4r00t-sh/petti/internal/runtime"
	"github.com/r4r00t-sh/petti/internal/seccomp"
)

func parseInitPids(s string) (int64, error) {
	n, err := strconv.ParseInt(s, 10, 64)
	if err != nil || n < 0 {
		return 0, fmt.Errorf("invalid --pids-limit %q", s)
	}
	return n, nil
}

func parseSeccompFlag(s string) (seccomp.Mode, error) {
	return seccomp.ParseMode(s)
}

// initCommand is the hidden re-exec entrypoint. Not shown in help.
func initCommand(args []string, streams IO) int {
	cfg, err := parseInitArgs(args)
	if err != nil {
		_, _ = fmt.Fprintf(streams.Err, "petti __init: %v\n", err)
		return ExitInternal
	}
	if err := runtime.Init(cfg); err != nil {
		_, _ = fmt.Fprintf(streams.Err, "petti __init: %v\n", err)
		return ExitInternal
	}
	return ExitOK
}

func parseInitArgs(args []string) (runtime.Config, error) {
	var cfg runtime.Config
	cfg.SyncFD = -1
	i := 0
	for i < len(args) {
		a := args[i]
		switch {
		case a == "--rootfs":
			if i+1 >= len(args) {
				return cfg, fmt.Errorf("--rootfs requires a value")
			}
			cfg.Rootfs = args[i+1]
			i += 2
		case strings.HasPrefix(a, "--rootfs="):
			cfg.Rootfs = strings.TrimPrefix(a, "--rootfs=")
			i++
		case a == "--hostname":
			if i+1 >= len(args) {
				return cfg, fmt.Errorf("--hostname requires a value")
			}
			cfg.Hostname = args[i+1]
			i += 2
		case strings.HasPrefix(a, "--hostname="):
			cfg.Hostname = strings.TrimPrefix(a, "--hostname=")
			i++
		case a == "--workdir":
			if i+1 >= len(args) {
				return cfg, fmt.Errorf("--workdir requires a value")
			}
			cfg.Workdir = args[i+1]
			i += 2
		case strings.HasPrefix(a, "--workdir="):
			cfg.Workdir = strings.TrimPrefix(a, "--workdir=")
			i++
		case a == "--env":
			if i+1 >= len(args) {
				return cfg, fmt.Errorf("--env requires KEY=VAL")
			}
			cfg.Env = append(cfg.Env, args[i+1])
			i += 2
		case strings.HasPrefix(a, "--env="):
			cfg.Env = append(cfg.Env, strings.TrimPrefix(a, "--env="))
			i++
		case a == "--user":
			if i+1 >= len(args) {
				return cfg, fmt.Errorf("--user requires uid[:gid]")
			}
			u, err := runtime.ParseUser(args[i+1])
			if err != nil {
				return cfg, err
			}
			cfg.User = u
			i += 2
		case strings.HasPrefix(a, "--user="):
			u, err := runtime.ParseUser(strings.TrimPrefix(a, "--user="))
			if err != nil {
				return cfg, err
			}
			cfg.User = u
			i++
		case a == "--mount":
			if i+1 >= len(args) {
				return cfg, fmt.Errorf("--mount requires host:dest:mode")
			}
			v, err := parseMountFlag(args[i+1])
			if err != nil {
				return cfg, err
			}
			cfg.Volumes = append(cfg.Volumes, v)
			i += 2
		case strings.HasPrefix(a, "--mount="):
			v, err := parseMountFlag(strings.TrimPrefix(a, "--mount="))
			if err != nil {
				return cfg, err
			}
			cfg.Volumes = append(cfg.Volumes, v)
			i++
		case a == "--sync-fd":
			if i+1 >= len(args) {
				return cfg, fmt.Errorf("--sync-fd requires a value")
			}
			fd, err := runtime.ParseSyncFD(args[i+1])
			if err != nil {
				return cfg, err
			}
			cfg.SyncFD = fd
			i += 2
		case strings.HasPrefix(a, "--sync-fd="):
			fd, err := runtime.ParseSyncFD(strings.TrimPrefix(a, "--sync-fd="))
			if err != nil {
				return cfg, err
			}
			cfg.SyncFD = fd
			i++
		case a == "--cap-add":
			if i+1 >= len(args) {
				return cfg, fmt.Errorf("--cap-add requires a value")
			}
			cfg.CapAdd = append(cfg.CapAdd, args[i+1])
			i += 2
		case strings.HasPrefix(a, "--cap-add="):
			cfg.CapAdd = append(cfg.CapAdd, strings.TrimPrefix(a, "--cap-add="))
			i++
		case a == "--cap-drop":
			if i+1 >= len(args) {
				return cfg, fmt.Errorf("--cap-drop requires a value")
			}
			cfg.CapDrop = append(cfg.CapDrop, args[i+1])
			i += 2
		case strings.HasPrefix(a, "--cap-drop="):
			cfg.CapDrop = append(cfg.CapDrop, strings.TrimPrefix(a, "--cap-drop="))
			i++
		case a == "--seccomp":
			if i+1 >= len(args) {
				return cfg, fmt.Errorf("--seccomp requires a value")
			}
			m, err := parseSeccompFlag(args[i+1])
			if err != nil {
				return cfg, err
			}
			cfg.Seccomp = m
			i += 2
		case strings.HasPrefix(a, "--seccomp="):
			m, err := parseSeccompFlag(strings.TrimPrefix(a, "--seccomp="))
			if err != nil {
				return cfg, err
			}
			cfg.Seccomp = m
			i++
		case a == "--read-only":
			cfg.ReadOnly = true
			i++
		case a == "--network":
			if i+1 >= len(args) {
				return cfg, fmt.Errorf("--network requires a value")
			}
			cfg.NetworkMode = args[i+1]
			i += 2
		case strings.HasPrefix(a, "--network="):
			cfg.NetworkMode = strings.TrimPrefix(a, "--network=")
			i++
		case a == "--cgroup-path":
			if i+1 >= len(args) {
				return cfg, fmt.Errorf("--cgroup-path requires a value")
			}
			cfg.CgroupPath = args[i+1]
			i += 2
		case strings.HasPrefix(a, "--cgroup-path="):
			cfg.CgroupPath = strings.TrimPrefix(a, "--cgroup-path=")
			i++
		case a == "--pids-limit":
			if i+1 >= len(args) {
				return cfg, fmt.Errorf("--pids-limit requires a value")
			}
			n, err := parseInitPids(args[i+1])
			if err != nil {
				return cfg, err
			}
			cfg.PidsLimit = n
			i += 2
		case strings.HasPrefix(a, "--pids-limit="):
			n, err := parseInitPids(strings.TrimPrefix(a, "--pids-limit="))
			if err != nil {
				return cfg, err
			}
			cfg.PidsLimit = n
			i++
		case a == "--":
			cfg.Args = args[i+1:]
			return validateInit(cfg)
		default:
			if strings.HasPrefix(a, "-") {
				return cfg, fmt.Errorf("unknown flag: %s", a)
			}
			cfg.Args = args[i:]
			return validateInit(cfg)
		}
	}
	return validateInit(cfg)
}

func parseMountFlag(s string) (runtime.VolumeSpec, error) {
	parts := strings.Split(s, ":")
	if len(parts) < 2 {
		return runtime.VolumeSpec{}, fmt.Errorf("invalid --mount %q", s)
	}
	v := runtime.VolumeSpec{Source: parts[0], Target: parts[1]}
	if len(parts) >= 3 && parts[2] == "ro" {
		v.ReadOnly = true
	}
	return v, nil
}

func validateInit(cfg runtime.Config) (runtime.Config, error) {
	if cfg.Rootfs == "" {
		return cfg, fmt.Errorf("--rootfs is required")
	}
	if cfg.Hostname == "" {
		return cfg, fmt.Errorf("--hostname is required")
	}
	if cfg.SyncFD < 0 {
		return cfg, fmt.Errorf("--sync-fd is required")
	}
	if len(cfg.Args) == 0 {
		return cfg, fmt.Errorf("command is required")
	}
	if cfg.Workdir == "" {
		cfg.Workdir = "/"
	}
	return cfg, nil
}
