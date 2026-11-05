package cli

import (
	"errors"
	"fmt"
	"strings"

	"github.com/r4r00t-sh/petti/internal/container"
	"github.com/r4r00t-sh/petti/internal/network"
	"github.com/r4r00t-sh/petti/internal/runtime"
	"github.com/r4r00t-sh/petti/internal/seccomp"
)

const runUsage = `Usage:
  petti run [flags] <image> [cmd...]
  petti run [flags] --rootfs <dir> <cmd> [args...]

Flags:
  -d, --detach           Run container in background and print id
  -i                     Keep STDIN open
  -t                     Allocate a pseudo-TTY (best-effort)
  --name <name>          Assign a name
  --rm                   Remove filesystem when container exits
  --rootfs <dir>         Use a local rootfs instead of an image
  --hostname <name>      Container hostname
  -w, --workdir <dir>    Working directory
  -e, --env KEY=VAL      Set environment (repeatable)
  -u, --user uid[:gid]   Numeric user
  --entrypoint <path>    Entrypoint override
  -v, --volume host:dest[:ro]
                         Bind mount (NOTE: top-level petti -v means version)
  --memory <size>        Memory limit
  --cpus <n>             CPU quota
  --pids-limit <n>       PIDs limit
  --network <mode>       bridge (default as root) | host | none
                         (rootless default: host)
  -p, --publish hostPort:containerPort[/proto]
                         Publish a port (bridge mode; repeatable)
  --rootless             Force user-namespace mode (auto when non-root)
  --cap-add <CAP>        Add a capability (repeatable; ALL allowed)
  --cap-drop <CAP>       Drop a capability (repeatable; ALL allowed)
  --security-opt seccomp=default|unconfined
                         Seccomp profile (default: denylist)
  --read-only            Remount container rootfs read-only
  -h, --help             Show this help
`

type runFlags struct {
	Rootfs       string
	Hostname     string
	Workdir      string
	Env          []string
	User         string
	Entrypoint   string
	Memory       string
	CPUs         string
	PidsLimit    string
	Name         string
	Volumes      []string
	Network      string
	Publish      []string
	CapAdd       []string
	CapDrop      []string
	SecurityOpts []string
	ReadOnly     bool
	Detach       bool
	AutoRemove   bool
	Tty          bool
	Interactive  bool
	Rootless     bool
	Help         bool
	Args         []string
}

func parseRunArgs(args []string) (runFlags, error) {
	var f runFlags
	i := 0
	for i < len(args) {
		a := args[i]
		switch {
		case a == "-h" || a == "--help":
			f.Help = true
			return f, nil
		case a == "-d" || a == "--detach":
			f.Detach = true
			i++
		case a == "-i":
			f.Interactive = true
			i++
		case a == "-t":
			f.Tty = true
			i++
		case a == "-it" || a == "-ti":
			f.Interactive = true
			f.Tty = true
			i++
		case a == "--rm":
			f.AutoRemove = true
			i++
		case a == "--name":
			if i+1 >= len(args) {
				return f, fmt.Errorf("--name requires a value")
			}
			f.Name = args[i+1]
			i += 2
		case strings.HasPrefix(a, "--name="):
			f.Name = strings.TrimPrefix(a, "--name=")
			i++
		case a == "--rootfs":
			if i+1 >= len(args) {
				return f, fmt.Errorf("--rootfs requires a value")
			}
			f.Rootfs = args[i+1]
			i += 2
		case strings.HasPrefix(a, "--rootfs="):
			f.Rootfs = strings.TrimPrefix(a, "--rootfs=")
			i++
		case a == "--hostname":
			if i+1 >= len(args) {
				return f, fmt.Errorf("--hostname requires a value")
			}
			f.Hostname = args[i+1]
			i += 2
		case strings.HasPrefix(a, "--hostname="):
			f.Hostname = strings.TrimPrefix(a, "--hostname=")
			i++
		case a == "--workdir" || a == "-w":
			if i+1 >= len(args) {
				return f, fmt.Errorf("%s requires a value", a)
			}
			f.Workdir = args[i+1]
			i += 2
		case strings.HasPrefix(a, "--workdir="):
			f.Workdir = strings.TrimPrefix(a, "--workdir=")
			i++
		case a == "--env" || a == "-e":
			if i+1 >= len(args) {
				return f, fmt.Errorf("%s requires KEY=VAL", a)
			}
			f.Env = append(f.Env, args[i+1])
			i += 2
		case strings.HasPrefix(a, "--env="):
			f.Env = append(f.Env, strings.TrimPrefix(a, "--env="))
			i++
		case a == "--user" || a == "-u":
			if i+1 >= len(args) {
				return f, fmt.Errorf("%s requires uid[:gid]", a)
			}
			f.User = args[i+1]
			i += 2
		case strings.HasPrefix(a, "--user="):
			f.User = strings.TrimPrefix(a, "--user=")
			i++
		case a == "--entrypoint":
			if i+1 >= len(args) {
				return f, fmt.Errorf("--entrypoint requires a value")
			}
			f.Entrypoint = args[i+1]
			i += 2
		case strings.HasPrefix(a, "--entrypoint="):
			f.Entrypoint = strings.TrimPrefix(a, "--entrypoint=")
			i++
		case a == "-v" || a == "--volume":
			if i+1 >= len(args) {
				return f, fmt.Errorf("%s requires host:container[:ro]", a)
			}
			f.Volumes = append(f.Volumes, args[i+1])
			i += 2
		case strings.HasPrefix(a, "--volume="):
			f.Volumes = append(f.Volumes, strings.TrimPrefix(a, "--volume="))
			i++
		case a == "--memory":
			if i+1 >= len(args) {
				return f, fmt.Errorf("--memory requires a value")
			}
			f.Memory = args[i+1]
			i += 2
		case strings.HasPrefix(a, "--memory="):
			f.Memory = strings.TrimPrefix(a, "--memory=")
			i++
		case a == "--cpus":
			if i+1 >= len(args) {
				return f, fmt.Errorf("--cpus requires a value")
			}
			f.CPUs = args[i+1]
			i += 2
		case strings.HasPrefix(a, "--cpus="):
			f.CPUs = strings.TrimPrefix(a, "--cpus=")
			i++
		case a == "--pids-limit":
			if i+1 >= len(args) {
				return f, fmt.Errorf("--pids-limit requires a value")
			}
			f.PidsLimit = args[i+1]
			i += 2
		case strings.HasPrefix(a, "--pids-limit="):
			f.PidsLimit = strings.TrimPrefix(a, "--pids-limit=")
			i++
		case a == "--network":
			if i+1 >= len(args) {
				return f, fmt.Errorf("--network requires a value")
			}
			f.Network = args[i+1]
			i += 2
		case strings.HasPrefix(a, "--network="):
			f.Network = strings.TrimPrefix(a, "--network=")
			i++
		case a == "-p" || a == "--publish":
			if i+1 >= len(args) {
				return f, fmt.Errorf("%s requires hostPort:containerPort[/proto]", a)
			}
			f.Publish = append(f.Publish, args[i+1])
			i += 2
		case strings.HasPrefix(a, "--publish="):
			f.Publish = append(f.Publish, strings.TrimPrefix(a, "--publish="))
			i++
		case a == "--rootless":
			f.Rootless = true
			i++
		case a == "--cap-add":
			if i+1 >= len(args) {
				return f, fmt.Errorf("--cap-add requires a value")
			}
			f.CapAdd = append(f.CapAdd, args[i+1])
			i += 2
		case strings.HasPrefix(a, "--cap-add="):
			f.CapAdd = append(f.CapAdd, strings.TrimPrefix(a, "--cap-add="))
			i++
		case a == "--cap-drop":
			if i+1 >= len(args) {
				return f, fmt.Errorf("--cap-drop requires a value")
			}
			f.CapDrop = append(f.CapDrop, args[i+1])
			i += 2
		case strings.HasPrefix(a, "--cap-drop="):
			f.CapDrop = append(f.CapDrop, strings.TrimPrefix(a, "--cap-drop="))
			i++
		case a == "--security-opt":
			if i+1 >= len(args) {
				return f, fmt.Errorf("--security-opt requires key=value")
			}
			f.SecurityOpts = append(f.SecurityOpts, args[i+1])
			i += 2
		case strings.HasPrefix(a, "--security-opt="):
			f.SecurityOpts = append(f.SecurityOpts, strings.TrimPrefix(a, "--security-opt="))
			i++
		case a == "--read-only":
			f.ReadOnly = true
			i++
		case a == "--":
			f.Args = args[i+1:]
			return f, nil
		case strings.HasPrefix(a, "-"):
			return f, fmt.Errorf("unknown flag: %s", a)
		default:
			f.Args = args[i:]
			return f, nil
		}
	}
	return f, nil
}

func runCommand(args []string, streams IO) int {
	f, err := parseRunArgs(args)
	if err != nil {
		_, _ = fmt.Fprintf(streams.Err, "petti run: %v\n\n%s", err, runUsage)
		return ExitUsage
	}
	if f.Help {
		_, _ = fmt.Fprint(streams.Out, runUsage)
		return ExitOK
	}

	var image string
	var cmdArgs []string
	var entrypoint []string
	if f.Entrypoint != "" {
		entrypoint = []string{f.Entrypoint}
	}

	if f.Rootfs != "" {
		cmdArgs = f.Args
		if f.Entrypoint == "" && len(cmdArgs) == 0 {
			_, _ = fmt.Fprintf(streams.Err, "petti run: command or --entrypoint is required\n\n%s", runUsage)
			return ExitUsage
		}
	} else {
		if len(f.Args) == 0 {
			_, _ = fmt.Fprintf(streams.Err, "petti run: image name or --rootfs is required\n\n%s", runUsage)
			return ExitUsage
		}
		image = f.Args[0]
		cmdArgs = f.Args[1:]
	}

	var vols []container.Volume
	for _, s := range f.Volumes {
		v, err := container.ParseVolume(s)
		if err != nil {
			_, _ = fmt.Fprintf(streams.Err, "petti run: %v\n", err)
			return ExitUsage
		}
		vols = append(vols, v)
	}
	var ports []network.PortMapping
	for _, s := range f.Publish {
		p, err := network.ParsePort(s)
		if err != nil {
			_, _ = fmt.Fprintf(streams.Err, "petti run: %v\n", err)
			return ExitUsage
		}
		ports = append(ports, p)
	}
	secMode := seccomp.ModeDefault
	for _, opt := range f.SecurityOpts {
		k, v, err := seccomp.ParseSecurityOpt(opt)
		if err != nil {
			_, _ = fmt.Fprintf(streams.Err, "petti run: %v\n", err)
			return ExitUsage
		}
		switch k {
		case "seccomp":
			secMode, err = seccomp.ParseMode(v)
			if err != nil {
				_, _ = fmt.Fprintf(streams.Err, "petti run: %v\n", err)
				return ExitUsage
			}
		default:
			_, _ = fmt.Fprintf(streams.Err, "petti run: unsupported --security-opt %q\n", opt)
			return ExitUsage
		}
	}

	res, err := container.Run(container.RunOptions{
		Rootfs:      f.Rootfs,
		Image:       image,
		Name:        f.Name,
		Hostname:    f.Hostname,
		Workdir:     f.Workdir,
		Env:         f.Env,
		User:        f.User,
		Entrypoint:  entrypoint,
		Args:        cmdArgs,
		Volumes:     vols,
		Memory:      f.Memory,
		CPUs:        f.CPUs,
		PidsLimit:   f.PidsLimit,
		Detach:      f.Detach,
		AutoRemove:  f.AutoRemove,
		Tty:         f.Tty,
		Interactive: f.Interactive,
		Network:     f.Network,
		Ports:       ports,
		Rootless:    f.Rootless,
		CapAdd:      f.CapAdd,
		CapDrop:     f.CapDrop,
		Seccomp:     secMode,
		ReadOnly:    f.ReadOnly,
		Stdin:       streams.In,
		Stdout:      streams.Out,
		Stderr:      streams.Err,
	})
	if err == nil {
		if f.Detach {
			_, _ = fmt.Fprintln(streams.Out, res.ID)
		}
		return ExitOK
	}

	var ee *runtime.ExitError
	if errors.As(err, &ee) {
		if ee.OOM {
			_, _ = fmt.Fprintf(streams.Err, "petti: container was OOM-killed\n")
		}
		return ee.Code
	}
	_, _ = fmt.Fprintf(streams.Err, "petti run: %v\n", err)
	return ExitRuntime
}
