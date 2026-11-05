package cli

import (
	"fmt"

	"github.com/r4r00t-sh/petti/internal/compose"
)

func composeCommand(args []string, streams IO) int {
	if len(args) == 0 || args[0] == "-h" || args[0] == "--help" {
		_, _ = fmt.Fprint(streams.Out, `Usage:
  petti compose up [-f petti.yaml] [-d]
  petti compose down [-f petti.yaml]

Compose-lite reads petti.yaml services and starts/stops named containers.
`)
		return ExitOK
	}
	sub := args[0]
	rest := args[1:]
	file := "petti.yaml"
	detach := true
	var positional []string
	for i := 0; i < len(rest); i++ {
		a := rest[i]
		switch {
		case a == "-f" || a == "--file":
			if i+1 >= len(rest) {
				_, _ = fmt.Fprintf(streams.Err, "petti compose: %s requires a value\n", a)
				return ExitUsage
			}
			file = rest[i+1]
			i++
		case a == "-d" || a == "--detach":
			detach = true
		case len(a) > 0 && a[0] == '-':
			_, _ = fmt.Fprintf(streams.Err, "petti compose: unknown flag %s\n", a)
			return ExitUsage
		default:
			positional = append(positional, a)
		}
	}
	_ = positional

	switch sub {
	case "up":
		res, err := compose.Up(compose.UpOptions{File: file, Detach: detach})
		if err != nil {
			_, _ = fmt.Fprintf(streams.Err, "petti compose up: %v\n", err)
			return ExitRuntime
		}
		for _, r := range res {
			name := r.Name
			if name == "" {
				name = r.ID
			}
			_, _ = fmt.Fprintf(streams.Out, "%s\n", name)
		}
		return ExitOK
	case "down":
		if err := compose.Down(file, ""); err != nil {
			_, _ = fmt.Fprintf(streams.Err, "petti compose down: %v\n", err)
			return ExitRuntime
		}
		_, _ = fmt.Fprintln(streams.Out, "ok")
		return ExitOK
	default:
		_, _ = fmt.Fprintf(streams.Err, "petti compose: unknown subcommand %s\n", sub)
		return ExitUsage
	}
}
