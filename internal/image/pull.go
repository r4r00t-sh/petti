package image

import (
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/r4r00t-sh/petti/internal/registry"
	"github.com/r4r00t-sh/petti/internal/state"
)

// PullOptions configures an image pull.
type PullOptions struct {
	Ref      string
	Platform string // empty → host platform
	Stdout   io.Writer
	Stderr   io.Writer
}

// Pull downloads an image into the store.
func (s *Store) Pull(ctx context.Context, opts PullOptions) (Record, error) {
	ref, err := registry.ParseReference(opts.Ref)
	if err != nil {
		return Record{}, err
	}
	platform := registry.HostPlatform()
	if opts.Platform != "" {
		platform, err = registry.ParsePlatform(opts.Platform)
		if err != nil {
			return Record{}, err
		}
	}
	progress := opts.Stderr
	if progress == nil {
		progress = io.Discard
	}

	client := registry.NewClient()
	_, _ = fmt.Fprintf(progress, "Pulling %s (%s/%s)\n", ref.String(), platform.OS, platform.Architecture)

	man, manDigest, err := client.FetchManifest(ctx, ref, platform)
	if err != nil {
		return Record{}, fmt.Errorf("manifest: %w", err)
	}
	_, _ = fmt.Fprintf(progress, "Manifest %s\n", shortDigest(manDigest))

	cfgBytes, err := client.FetchBlobBytes(ctx, ref, man.Config.Digest)
	if err != nil {
		return Record{}, fmt.Errorf("config: %w", err)
	}
	cfgHex := strings.TrimPrefix(man.Config.Digest, "sha256:")
	if err := state.WriteFileAtomic(s.Layout.BlobPath(cfgHex), cfgBytes, 0o644); err != nil {
		return Record{}, err
	}
	var imgCfg registry.ImageConfig
	if err := json.Unmarshal(cfgBytes, &imgCfg); err != nil {
		return Record{}, fmt.Errorf("parse config: %w", err)
	}
	if len(imgCfg.RootFS.DiffIDs) != len(man.Layers) {
		return Record{}, fmt.Errorf("config diff_ids (%d) != manifest layers (%d)", len(imgCfg.RootFS.DiffIDs), len(man.Layers))
	}

	var layers []LayerMeta
	var total int64
	for i, desc := range man.Layers {
		diffID := imgCfg.RootFS.DiffIDs[i]
		_, _ = fmt.Fprintf(progress, "Layer %s\n", shortDigest(desc.Digest))
		blobHex := strings.TrimPrefix(desc.Digest, "sha256:")
		blobPath := s.Layout.BlobPath(blobHex)
		partial := blobPath + ".partial"
		if _, err := os.Stat(blobPath); err != nil {
			if err := client.FetchBlob(ctx, ref, desc.Digest, partial, progress); err != nil {
				return Record{}, fmt.Errorf("blob %s: %w", desc.Digest, err)
			}
			if err := os.Rename(partial, blobPath); err != nil {
				return Record{}, err
			}
			_, _ = fmt.Fprintln(progress)
		} else {
			_, _ = fmt.Fprintf(progress, "  already present\n")
		}

		diffHex := strings.TrimPrefix(diffID, "sha256:")
		layerDir := s.Layout.LayerPath(diffHex)
		marker := filepath.Join(layerDir, ".petti-unpacked")
		if _, err := os.Stat(marker); err != nil {
			if err := os.RemoveAll(layerDir); err != nil {
				return Record{}, err
			}
			if err := unpackGzipLayer(blobPath, layerDir, diffID); err != nil {
				_ = os.RemoveAll(layerDir)
				return Record{}, fmt.Errorf("unpack %s: %w", desc.Digest, err)
			}
			if err := os.WriteFile(marker, []byte(diffID+"\n"), 0o644); err != nil {
				return Record{}, err
			}
		}
		layers = append(layers, LayerMeta{Digest: desc.Digest, DiffID: diffID, Size: desc.Size})
		total += desc.Size
	}

	tag := ref.Tag
	if tag == "" {
		tag = "latest"
	}
	rec := Record{
		ID:             man.Config.Digest,
		Repo:           ref.Domain + "/" + ref.Path,
		Tag:            tag,
		ManifestDigest: manDigest,
		ConfigDigest:   man.Config.Digest,
		Created:        imgCfg.Created,
		Size:           total,
		Layers:         layers,
		Config:         imgCfg,
		PulledAt:       time.Now().UTC(),
	}
	if err := s.SaveRecord(rec); err != nil {
		return Record{}, err
	}
	_, _ = fmt.Fprintf(progress, "Pulled %s:%s (%s)\n", rec.Repo, rec.Tag, shortDigest(rec.ID))
	return rec, nil
}

// UnpackGzipLayerForArchive unpacks a gzip layer blob for petti load.
func UnpackGzipLayerForArchive(blobPath, dest, wantDiffID string) error {
	return unpackGzipLayer(blobPath, dest, wantDiffID)
}

func unpackGzipLayer(blobPath, dest, wantDiffID string) error {
	f, err := os.Open(blobPath)
	if err != nil {
		return err
	}
	defer f.Close()
	gz, err := gzip.NewReader(f)
	if err != nil {
		return fmt.Errorf("gzip: %w", err)
	}
	defer gz.Close()

	h := sha256.New()
	tr := io.TeeReader(gz, h)
	if err := ApplyLayerTar(tr, dest); err != nil {
		return err
	}
	sum := "sha256:" + hex.EncodeToString(h.Sum(nil))
	if sum != wantDiffID {
		return fmt.Errorf("diffID mismatch: got %s want %s", sum, wantDiffID)
	}
	return nil
}

func shortDigest(d string) string {
	d = strings.TrimPrefix(d, "sha256:")
	if len(d) > 12 {
		return d[:12]
	}
	return d
}
