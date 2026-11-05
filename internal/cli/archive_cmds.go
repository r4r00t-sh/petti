package cli

import (
	"fmt"

	"github.com/r4r00t-sh/petti/internal/archive"
	"github.com/r4r00t-sh/petti/internal/image"
)

func saveCommand(args []string, streams IO) int {
	if len(args) == 0 || args[0] == "-h" || args[0] == "--help" {
		_, _ = fmt.Fprint(streams.Out, "Usage: petti save -o <file.tar.gz> <image> [image…]\n")
		if len(args) > 0 && (args[0] == "-h" || args[0] == "--help") {
			return ExitOK
		}
		return ExitUsage
	}
	var out string
	var images []string
	for i := 0; i < len(args); i++ {
		a := args[i]
		switch {
		case a == "-o" || a == "--output":
			if i+1 >= len(args) {
				_, _ = fmt.Fprintf(streams.Err, "petti save: %s requires a path\n", a)
				return ExitUsage
			}
			out = args[i+1]
			i++
		case len(a) > 9 && a[:9] == "--output=":
			out = a[9:]
		case len(a) > 0 && a[0] == '-':
			_, _ = fmt.Fprintf(streams.Err, "petti save: unknown flag %s\n", a)
			return ExitUsage
		default:
			images = append(images, a)
		}
	}
	if out == "" || len(images) == 0 {
		_, _ = fmt.Fprintf(streams.Err, "petti save: -o and at least one image required\n")
		return ExitUsage
	}
	store, err := image.OpenStore("")
	if err != nil {
		_, _ = fmt.Fprintf(streams.Err, "petti save: %v\n", err)
		return ExitRuntime
	}
	if err := archive.Save(store, images, out); err != nil {
		_, _ = fmt.Fprintf(streams.Err, "petti save: %v\n", err)
		return ExitRuntime
	}
	_, _ = fmt.Fprintln(streams.Out, out)
	return ExitOK
}

func loadCommand(args []string, streams IO) int {
	if len(args) != 1 || args[0] == "-h" || args[0] == "--help" {
		_, _ = fmt.Fprint(streams.Out, "Usage: petti load <file.tar.gz>\n")
		if len(args) == 1 && (args[0] == "-h" || args[0] == "--help") {
			return ExitOK
		}
		return ExitUsage
	}
	store, err := image.OpenStore("")
	if err != nil {
		_, _ = fmt.Fprintf(streams.Err, "petti load: %v\n", err)
		return ExitRuntime
	}
	recs, err := archive.Load(store, args[0])
	if err != nil {
		_, _ = fmt.Fprintf(streams.Err, "petti load: %v\n", err)
		return ExitRuntime
	}
	for _, r := range recs {
		_, _ = fmt.Fprintf(streams.Out, "Loaded %s:%s\n", r.Repo, r.Tag)
	}
	return ExitOK
}
