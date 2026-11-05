package cli

import (
	"fmt"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/r4r00t-sh/petti/internal/image"
)

func imagesCommand(args []string, streams IO) int {
	for _, a := range args {
		if a == "-h" || a == "--help" {
			_, _ = fmt.Fprint(streams.Out, "Usage: petti images\n\nList locally stored images.\n")
			return ExitOK
		}
		if strings.HasPrefix(a, "-") {
			_, _ = fmt.Fprintf(streams.Err, "petti images: unknown flag: %s\n", a)
			return ExitUsage
		}
	}
	store, err := image.OpenStore("")
	if err != nil {
		_, _ = fmt.Fprintf(streams.Err, "petti images: %v\n", err)
		return ExitRuntime
	}
	list, err := store.List()
	if err != nil {
		_, _ = fmt.Fprintf(streams.Err, "petti images: %v\n", err)
		return ExitRuntime
	}
	tw := tabwriter.NewWriter(streams.Out, 0, 4, 2, ' ', 0)
	_, _ = fmt.Fprintln(tw, "REPOSITORY\tTAG\tIMAGE ID\tSIZE\tCREATED")
	for _, rec := range list {
		id := strings.TrimPrefix(rec.ID, "sha256:")
		if len(id) > 12 {
			id = id[:12]
		}
		created := rec.Created
		if created == "" {
			created = rec.PulledAt.Format(time.RFC3339)
		}
		_, _ = fmt.Fprintf(tw, "%s\t%s\t%s\t%s\t%s\n", rec.Repo, rec.Tag, id, humanSize(rec.Size), created)
	}
	_ = tw.Flush()
	return ExitOK
}

func humanSize(n int64) string {
	const (
		kb = 1024
		mb = 1024 * kb
		gb = 1024 * mb
	)
	switch {
	case n >= gb:
		return fmt.Sprintf("%.1fGB", float64(n)/float64(gb))
	case n >= mb:
		return fmt.Sprintf("%.1fMB", float64(n)/float64(mb))
	case n >= kb:
		return fmt.Sprintf("%.1fKB", float64(n)/float64(kb))
	default:
		return fmt.Sprintf("%dB", n)
	}
}
