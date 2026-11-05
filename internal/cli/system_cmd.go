package cli

import (
	"fmt"

	"github.com/r4r00t-sh/petti/internal/system"
)

func systemCommand(args []string, streams IO) int {
	if len(args) == 0 || args[0] == "-h" || args[0] == "--help" {
		_, _ = fmt.Fprint(streams.Out, "Usage: petti system prune\n")
		return ExitOK
	}
	if args[0] != "prune" {
		_, _ = fmt.Fprintf(streams.Err, "petti system: unknown subcommand %s\n", args[0])
		return ExitUsage
	}
	if err := system.Prune(system.PruneOptions{Stdout: streams.Out}); err != nil {
		_, _ = fmt.Fprintf(streams.Err, "petti system prune: %v\n", err)
		return ExitRuntime
	}
	return ExitOK
}
