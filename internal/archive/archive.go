package archive

import (
	"archive/tar"
	"compress/gzip"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/r4r00t-sh/petti/internal/image"
	"github.com/r4r00t-sh/petti/internal/state"
)

const formatVersion = 1

// Manifest describes images inside a petti archive.
type Manifest struct {
	Version int             `json:"version"`
	Images  []ManifestImage `json:"images"`
}

// ManifestImage is one tagged image in the archive.
type ManifestImage struct {
	Repo   string       `json:"repo"`
	Tag    string       `json:"tag"`
	Record image.Record `json:"record"`
	Blobs  []string     `json:"blobs"` // sha256 hex digests to include
}

// Save writes one or more local images to a gzipped tar at destPath.
func Save(store *image.Store, names []string, destPath string) error {
	var images []ManifestImage
	blobSet := map[string]struct{}{}
	for _, name := range names {
		rec, err := store.Resolve(name)
		if err != nil {
			return err
		}
		mi := ManifestImage{Repo: rec.Repo, Tag: rec.Tag, Record: rec}
		cfgHex := strings.TrimPrefix(rec.ConfigDigest, "sha256:")
		if cfgHex != "" {
			blobSet[cfgHex] = struct{}{}
			mi.Blobs = append(mi.Blobs, cfgHex)
		}
		for _, l := range rec.Layers {
			hex := strings.TrimPrefix(l.Digest, "sha256:")
			blobSet[hex] = struct{}{}
			mi.Blobs = append(mi.Blobs, hex)
		}
		images = append(images, mi)
	}
	man := Manifest{Version: formatVersion, Images: images}
	manBytes, err := json.MarshalIndent(man, "", "  ")
	if err != nil {
		return err
	}

	f, err := os.Create(destPath)
	if err != nil {
		return err
	}
	defer f.Close()
	gz := gzip.NewWriter(f)
	defer gz.Close()
	tw := tar.NewWriter(gz)
	defer tw.Close()

	if err := writeBytes(tw, "manifest.json", manBytes); err != nil {
		return err
	}
	for hex := range blobSet {
		path := store.Layout.BlobPath(hex)
		if err := writeFile(tw, "blobs/sha256/"+hex, path); err != nil {
			return fmt.Errorf("blob %s: %w", hex, err)
		}
	}
	return nil
}

// Load imports images from a petti archive into the store.
func Load(store *image.Store, srcPath string) ([]image.Record, error) {
	f, err := os.Open(srcPath)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	gz, err := gzip.NewReader(f)
	if err != nil {
		return nil, fmt.Errorf("gzip: %w", err)
	}
	defer gz.Close()
	tr := tar.NewReader(gz)

	tmp, err := os.MkdirTemp("", "petti-load-*")
	if err != nil {
		return nil, err
	}
	defer os.RemoveAll(tmp)

	var man Manifest
	for {
		hdr, err := tr.Next()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return nil, err
		}
		if hdr.Typeflag != tar.TypeReg {
			continue
		}
		target := filepath.Join(tmp, hdr.Name)
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			return nil, err
		}
		out, err := os.Create(target)
		if err != nil {
			return nil, err
		}
		if _, err := io.Copy(out, tr); err != nil {
			_ = out.Close()
			return nil, err
		}
		_ = out.Close()
		if hdr.Name == "manifest.json" {
			data, err := os.ReadFile(target)
			if err != nil {
				return nil, err
			}
			if err := json.Unmarshal(data, &man); err != nil {
				return nil, err
			}
		}
	}
	if man.Version != formatVersion {
		return nil, fmt.Errorf("unsupported archive version %d", man.Version)
	}

	var loaded []image.Record
	for _, mi := range man.Images {
		for _, hex := range mi.Blobs {
			src := filepath.Join(tmp, "blobs", "sha256", hex)
			dst := store.Layout.BlobPath(hex)
			if _, err := os.Stat(dst); err == nil {
				continue
			}
			if err := copyFile(src, dst); err != nil {
				return nil, fmt.Errorf("install blob %s: %w", hex, err)
			}
		}
		// Unpack layers if missing.
		for _, l := range mi.Record.Layers {
			diffHex := strings.TrimPrefix(l.DiffID, "sha256:")
			layerDir := store.Layout.LayerPath(diffHex)
			marker := filepath.Join(layerDir, ".petti-unpacked")
			if _, err := os.Stat(marker); err == nil {
				continue
			}
			blobHex := strings.TrimPrefix(l.Digest, "sha256:")
			if err := unpackLayer(store.Layout.BlobPath(blobHex), layerDir, l.DiffID); err != nil {
				return nil, err
			}
			_ = os.WriteFile(marker, []byte(l.DiffID+"\n"), 0o644)
		}
		mi.Record.PulledAt = time.Now().UTC()
		if err := store.SaveRecord(mi.Record); err != nil {
			return nil, err
		}
		loaded = append(loaded, mi.Record)
	}
	return loaded, nil
}

func unpackLayer(blobPath, layerDir, diffID string) error {
	// Reuse image package unpack via a thin local gzip+tar extract with diff check.
	return imageUnpack(blobPath, layerDir, diffID)
}

// imageUnpack is set in unpack_bridge.go to call image internals without cycles —
// we duplicate the call by importing image's exported helper.
func imageUnpack(blobPath, layerDir, diffID string) error {
	return image.UnpackGzipLayerForArchive(blobPath, layerDir, diffID)
}

func writeBytes(tw *tar.Writer, name string, data []byte) error {
	hdr := &tar.Header{Name: name, Mode: 0o644, Size: int64(len(data)), ModTime: time.Now()}
	if err := tw.WriteHeader(hdr); err != nil {
		return err
	}
	_, err := tw.Write(data)
	return err
}

func writeFile(tw *tar.Writer, name, path string) error {
	st, err := os.Stat(path)
	if err != nil {
		return err
	}
	hdr, err := tar.FileInfoHeader(st, "")
	if err != nil {
		return err
	}
	hdr.Name = name
	if err := tw.WriteHeader(hdr); err != nil {
		return err
	}
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()
	_, err = io.Copy(tw, f)
	return err
}

func copyFile(src, dst string) error {
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return err
	}
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	data, err := io.ReadAll(in)
	if err != nil {
		return err
	}
	return state.WriteFileAtomic(dst, data, 0o644)
}
