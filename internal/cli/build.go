package cli

import (
	"context"
	"fmt"
	"path/filepath"

	"github.com/r4r00t-sh/petti/internal/build"
)

const buildUsage = `Usage:
  petti build [flags] <context>

Flags:
  -f, --file <path>   Petfile path (default: Petfile)
  -t, --tag <name>    Name:tag (stored as localhost/<name>:<tag>)
  --no-cache          Do not use build cache
  -h, --help          Show this help

Petfile instructions: FROM, RUN, COPY, ENV, WORKDIR, CMD, ENTRYPOINT
RUN requires root. COPY/config steps work unprivileged after a local base image exists.
`

func buildCommand(args []string, streams IO) int {
	var (
		file    = "Petfile"
		tag     string
		noCache bool
		help    bool
		pos     []string
	)
	for i := 0; i < len(args); i++ {
		a := args[i]
		switch {
		case a == "-h" || a == "--help":
			help = true
		case a == "-f" || a == "--file":
			if i+1 >= len(args) {
				_, _ = fmt.Fprintf(streams.Err, "petti build: %s requires a value\n", a)
				return ExitUsage
			}
			file = args[i+1]
			i++
		case len(a) > 7 && a[:7] == "--file=":
			file = a[7:]
		case a == "-t" || a == "--tag":
			if i+1 >= len(args) {
				_, _ = fmt.Fprintf(streams.Err, "petti build: %s requires a value\n", a)
				return ExitUsage
			}
			tag = args[i+1]
			i++
		case len(a) > 6 && a[:6] == "--tag=":
			tag = a[6:]
		case a == "--no-cache":
			noCache = true
		case len(a) > 0 && a[0] == '-':
			_, _ = fmt.Fprintf(streams.Err, "petti build: unknown flag %s\n", a)
			return ExitUsage
		default:
			pos = append(pos, a)
		}
	}
	if help {
		_, _ = fmt.Fprint(streams.Out, buildUsage)
		return ExitOK
	}
	ctxDir := "."
	if len(pos) == 1 {
		ctxDir = pos[0]
	} else if len(pos) > 1 {
		_, _ = fmt.Fprintf(streams.Err, "petti build: unexpected args\n\n%s", buildUsage)
		return ExitUsage
	}
	abs, err := filepath.Abs(ctxDir)
	if err != nil {
		_, _ = fmt.Fprintf(streams.Err, "petti build: %v\n", err)
		return ExitRuntime
	}
	b, err := build.New("")
	if err != nil {
		_, _ = fmt.Fprintf(streams.Err, "petti build: %v\n", err)
		return ExitRuntime
	}
	res, err := b.Build(context.Background(), build.Options{
		ContextDir: abs,
		Petfile:    file,
		Tag:        tag,
		NoCache:    noCache,
		Stdout:     streams.Out,
		Stderr:     streams.Err,
	})
	if err != nil {
		_, _ = fmt.Fprintf(streams.Err, "petti build: %v\n", err)
		return ExitRuntime
	}
	_, _ = fmt.Fprintln(streams.Out, res.Record.ID)
	return ExitOK
}
