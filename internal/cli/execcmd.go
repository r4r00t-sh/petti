package cli

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/r4r00t-sh/petti/internal/runtime"
)

func execInitCommand(args []string, streams IO) int {
	cfg, err := parseExecArgs(args)
	if err != nil {
		_, _ = fmt.Fprintf(streams.Err, "petti __exec: %v\n", err)
		return ExitInternal
	}
	if err := runtime.ExecInNamespaces(cfg); err != nil {
		_, _ = fmt.Fprintf(streams.Err, "petti __exec: %v\n", err)
		return ExitInternal
	}
	return ExitOK
}

func parseExecArgs(args []string) (runtime.ExecConfig, error) {
	var cfg runtime.ExecConfig
	i := 0
	for i < len(args) {
		a := args[i]
		switch a {
		case "--pid":
			if i+1 >= len(args) {
				return cfg, fmt.Errorf("--pid required")
			}
			pid, err := strconv.Atoi(args[i+1])
			if err != nil {
				return cfg, err
			}
			cfg.TargetPid = pid
			i += 2
		case "--workdir":
			if i+1 >= len(args) {
				return cfg, fmt.Errorf("--workdir required")
			}
			cfg.Workdir = args[i+1]
			i += 2
		case "--":
			cfg.Args = args[i+1:]
			if cfg.TargetPid <= 0 || len(cfg.Args) == 0 {
				return cfg, fmt.Errorf("pid and command required")
			}
			return cfg, nil
		default:
			if strings.HasPrefix(a, "-") {
				return cfg, fmt.Errorf("unknown flag %s", a)
			}
			cfg.Args = args[i:]
			return cfg, nil
		}
	}
	return cfg, fmt.Errorf("pid and command required")
}
