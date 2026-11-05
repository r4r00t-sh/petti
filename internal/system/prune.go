package system

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/r4r00t-sh/petti/internal/container"
	"github.com/r4r00t-sh/petti/internal/image"
	"github.com/r4r00t-sh/petti/internal/state"
)

// PruneOptions configures system prune.
type PruneOptions struct {
	Stdout io.Writer
}

// Prune removes stopped containers and unreferenced blobs/layers/build cache.
func Prune(opts PruneOptions) error {
	out := opts.Stdout
	if out == nil {
		out = io.Discard
	}
	root, err := state.DataRoot()
	if err != nil {
		return err
	}
	layout, err := state.NewLayout(root)
	if err != nil {
		return err
	}

	// Stopped / exited containers.
	cs, err := container.OpenStore(root)
	if err != nil {
		return err
	}
	list, err := cs.List()
	if err != nil {
		return err
	}
	removedC := 0
	for _, r := range list {
		if r.Status == container.StatusRunning {
			continue
		}
		if err := container.Remove(r.ID, true); err != nil {
			_, _ = fmt.Fprintf(out, "skip container %s: %v\n", r.ID[:12], err)
			continue
		}
		removedC++
	}
	_, _ = fmt.Fprintf(out, "Removed %d containers\n", removedC)

	// Referenced digests from images.
	imgs, err := image.OpenStore(root)
	if err != nil {
		return err
	}
	records, err := imgs.List()
	if err != nil {
		return err
	}
	keepBlob := map[string]struct{}{}
	keepLayer := map[string]struct{}{}
	for _, rec := range records {
		if h := strings.TrimPrefix(rec.ConfigDigest, "sha256:"); h != "" {
			keepBlob[h] = struct{}{}
		}
		for _, l := range rec.Layers {
			if h := strings.TrimPrefix(l.Digest, "sha256:"); h != "" {
				keepBlob[h] = struct{}{}
			}
			if h := strings.TrimPrefix(l.DiffID, "sha256:"); h != "" {
				keepLayer[h] = struct{}{}
			}
		}
	}

	removedB := 0
	_ = filepath.Walk(layout.Blobs, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return err
		}
		hex := info.Name()
		if _, ok := keepBlob[hex]; ok {
			return nil
		}
		if err := os.Remove(path); err == nil {
			removedB++
		}
		return nil
	})
	_, _ = fmt.Fprintf(out, "Removed %d blobs\n", removedB)

	removedL := 0
	layerRoot := filepath.Join(layout.Layers, "sha256")
	entries, _ := os.ReadDir(layerRoot)
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		if _, ok := keepLayer[e.Name()]; ok {
			continue
		}
		if err := os.RemoveAll(filepath.Join(layerRoot, e.Name())); err == nil {
			removedL++
		}
	}
	_, _ = fmt.Fprintf(out, "Removed %d layers\n", removedL)

	cache := filepath.Join(layout.Root, "buildcache")
	if err := os.RemoveAll(cache); err == nil {
		_, _ = fmt.Fprintln(out, "Cleared build cache")
	}
	return nil
}
