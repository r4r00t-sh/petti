package cli

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"strings"

	"github.com/r4r00t-sh/petti/internal/image"
)

const pullUsage = `Usage:
  petti pull [flags] <image[:tag|@digest]>

Download an image from a registry (anonymous pull).

Flags:
  --platform os/arch   Platform to pull (default: host, e.g. linux/amd64)
  -h, --help           Show this help

Examples:
  petti pull alpine
  petti pull library/alpine:3.20
  petti pull --platform linux/arm64 alpine
  petti pull ghcr.io/OWNER/IMAGE:tag
`

func pullCommand(args []string, streams IO) int {
	platform := ""
	var positional []string
	for i := 0; i < len(args); i++ {
		a := args[i]
		switch {
		case a == "-h" || a == "--help":
			_, _ = fmt.Fprint(streams.Out, pullUsage)
			return ExitOK
		case a == "--platform":
			if i+1 >= len(args) {
				_, _ = fmt.Fprintf(streams.Err, "petti pull: --platform requires os/arch\n")
				return ExitUsage
			}
			platform = args[i+1]
			i++
		case strings.HasPrefix(a, "--platform="):
			platform = strings.TrimPrefix(a, "--platform=")
		case strings.HasPrefix(a, "-"):
			_, _ = fmt.Fprintf(streams.Err, "petti pull: unknown flag: %s\n\n%s", a, pullUsage)
			return ExitUsage
		default:
			positional = append(positional, a)
		}
	}
	if len(positional) != 1 {
		_, _ = fmt.Fprintf(streams.Err, "petti pull: image reference required\n\n%s", pullUsage)
		return ExitUsage
	}

	store, err := image.OpenStore("")
	if err != nil {
		_, _ = fmt.Fprintf(streams.Err, "petti pull: %v\n", err)
		return ExitRuntime
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	rec, err := store.Pull(ctx, image.PullOptions{
		Ref:      positional[0],
		Platform: platform,
		Stderr:   streams.Err,
	})
	if err != nil {
		_, _ = fmt.Fprintf(streams.Err, "petti pull: %v\n", err)
		return ExitRuntime
	}
	_, _ = fmt.Fprintf(streams.Out, "%s:%s\n", rec.Repo, rec.Tag)
	return ExitOK
}
