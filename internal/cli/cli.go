// Package cli implements command parsing, help, and version for the petti binary.
package cli

import (
	"fmt"
	"io"
	"log/slog"
	"os"

	"github.com/r4r00t-sh/petti/internal/version"
)

// IO bundles streams so tests can capture stdout/stderr without racing os.Stdout.
type IO struct {
	In  io.Reader
	Out io.Writer
	Err io.Writer
}

// DefaultIO uses the process standard streams.
func DefaultIO() IO {
	return IO{In: os.Stdin, Out: os.Stdout, Err: os.Stderr}
}

// Execute parses args (typically os.Args[1:]) and runs the matching command.
// It returns a process exit code; it never calls os.Exit itself.
func Execute(args []string, streams IO) int {
	if streams.Out == nil {
		streams.Out = os.Stdout
	}
	if streams.Err == nil {
		streams.Err = os.Stderr
	}
	if streams.In == nil {
		streams.In = os.Stdin
	}

	// Global flags before the command.
	debug := false
	for len(args) > 0 {
		switch args[0] {
		case "--debug":
			debug = true
			args = args[1:]
			continue
		}
		break
	}
	if debug {
		slog.SetDefault(slog.New(slog.NewTextHandler(streams.Err, &slog.HandlerOptions{Level: slog.LevelDebug})))
		slog.Debug("debug logging enabled")
	} else {
		slog.SetDefault(slog.New(slog.NewTextHandler(io.Discard, nil)))
	}

	if len(args) == 0 {
		_, _ = fmt.Fprint(streams.Out, Usage())
		return ExitOK
	}

	switch args[0] {
	case "-h", "--help", "help":
		_, _ = fmt.Fprint(streams.Out, Usage())
		return ExitOK
	case "-v", "--v", "--version", "version":
		_, _ = fmt.Fprintln(streams.Out, version.String())
		return ExitOK
	case "run":
		return runCommand(args[1:], streams)
	case "pull":
		return pullCommand(args[1:], streams)
	case "images":
		return imagesCommand(args[1:], streams)
	case "rmi":
		return rmiCommand(args[1:], streams)
	case "build":
		return buildCommand(args[1:], streams)
	case "save":
		return saveCommand(args[1:], streams)
	case "load":
		return loadCommand(args[1:], streams)
	case "compose":
		return composeCommand(args[1:], streams)
	case "system":
		return systemCommand(args[1:], streams)
	case "ps":
		return psCommand(args[1:], streams)
	case "stop":
		return stopCommand(args[1:], streams)
	case "kill":
		return killCommand(args[1:], streams)
	case "rm":
		return rmCommand(args[1:], streams)
	case "logs":
		return logsCommand(args[1:], streams)
	case "inspect":
		return inspectCommand(args[1:], streams)
	case "exec":
		return execCommand(args[1:], streams)
	case "__init":
		return initCommand(args[1:], streams)
	case "__exec":
		return execInitCommand(args[1:], streams)
	default:
		if isFlag(args[0]) {
			_, _ = fmt.Fprintf(streams.Err, "petti: unknown flag: %s\n\n%s", args[0], Usage())
			return ExitUsage
		}
		_, _ = fmt.Fprintf(streams.Err, "petti: unknown command: %s\n\n%s", args[0], Usage())
		return ExitUsage
	}
}

func isFlag(s string) bool {
	return len(s) > 0 && s[0] == '-'
}
