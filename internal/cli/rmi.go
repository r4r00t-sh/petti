package cli

import (
	"fmt"
	"strings"

	"github.com/r4r00t-sh/petti/internal/image"
)

func rmiCommand(args []string, streams IO) int {
	var positional []string
	for _, a := range args {
		if a == "-h" || a == "--help" {
			_, _ = fmt.Fprint(streams.Out, "Usage: petti rmi <image[:tag]>\n\nRemove a local image tag (blobs retained until prune).\n")
			return ExitOK
		}
		if strings.HasPrefix(a, "-") {
			_, _ = fmt.Fprintf(streams.Err, "petti rmi: unknown flag: %s\n", a)
			return ExitUsage
		}
		positional = append(positional, a)
	}
	if len(positional) != 1 {
		_, _ = fmt.Fprintf(streams.Err, "petti rmi: image required\n")
		return ExitUsage
	}
	store, err := image.OpenStore("")
	if err != nil {
		_, _ = fmt.Fprintf(streams.Err, "petti rmi: %v\n", err)
		return ExitRuntime
	}
	if err := store.Remove(positional[0]); err != nil {
		_, _ = fmt.Fprintf(streams.Err, "petti rmi: %v\n", err)
		return ExitRuntime
	}
	_, _ = fmt.Fprintf(streams.Out, "Untagged: %s\n", positional[0])
	return ExitOK
}
