package image

import (
	"archive/tar"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// PackDirAsGzipLayer tars dir (as layer root), gzip-compresses it into blobPath,
// unpacks to layerDir, and returns compressed digest, diffID, and size.
func PackDirAsGzipLayer(dir, blobPath, layerDir string) (digest, diffID string, size int64, err error) {
	if err := os.MkdirAll(filepath.Dir(blobPath), 0o755); err != nil {
		return "", "", 0, err
	}
	tmp := blobPath + ".partial"
	f, err := os.Create(tmp)
	if err != nil {
		return "", "", 0, err
	}
	diffHash := sha256.New()
	gz := gzip.NewWriter(f)
	tw := tar.NewWriter(io.MultiWriter(gz, diffHash))

	err = filepath.Walk(dir, func(path string, info os.FileInfo, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		rel, err := filepath.Rel(dir, path)
		if err != nil {
			return err
		}
		if rel == "." {
			return nil
		}
		// Skip petti markers.
		base := filepath.Base(path)
		if strings.HasPrefix(base, ".petti-") {
			return nil
		}
		hdr, err := tar.FileInfoHeader(info, "")
		if err != nil {
			return err
		}
		hdr.Name = filepath.ToSlash(rel)
		if info.Mode()&os.ModeSymlink != 0 {
			link, err := os.Readlink(path)
			if err != nil {
				return err
			}
			hdr.Typeflag = tar.TypeSymlink
			hdr.Linkname = link
		}
		if err := tw.WriteHeader(hdr); err != nil {
			return err
		}
		if info.Mode().IsRegular() {
			rf, err := os.Open(path)
			if err != nil {
				return err
			}
			_, copyErr := io.Copy(tw, rf)
			_ = rf.Close()
			if copyErr != nil {
				return copyErr
			}
		}
		return nil
	})
	if err != nil {
		_ = tw.Close()
		_ = gz.Close()
		_ = f.Close()
		_ = os.Remove(tmp)
		return "", "", 0, err
	}
	if err := tw.Close(); err != nil {
		_ = gz.Close()
		_ = f.Close()
		_ = os.Remove(tmp)
		return "", "", 0, err
	}
	if err := gz.Close(); err != nil {
		_ = f.Close()
		_ = os.Remove(tmp)
		return "", "", 0, err
	}
	if err := f.Close(); err != nil {
		_ = os.Remove(tmp)
		return "", "", 0, err
	}
	st, err := os.Stat(tmp)
	if err != nil {
		_ = os.Remove(tmp)
		return "", "", 0, err
	}
	// Compressed digest.
	sum, err := fileSHA256(tmp)
	if err != nil {
		_ = os.Remove(tmp)
		return "", "", 0, err
	}
	digest = "sha256:" + sum
	diffID = "sha256:" + hex.EncodeToString(diffHash.Sum(nil))
	if err := os.Rename(tmp, blobPath); err != nil {
		_ = os.Remove(tmp)
		return "", "", 0, err
	}
	// Unpack for local use.
	_ = os.RemoveAll(layerDir)
	if err := unpackGzipLayer(blobPath, layerDir, diffID); err != nil {
		return "", "", 0, fmt.Errorf("unpack packed layer: %w", err)
	}
	if err := os.WriteFile(filepath.Join(layerDir, ".petti-unpacked"), []byte(diffID+"\n"), 0o644); err != nil {
		return "", "", 0, err
	}
	return digest, diffID, st.Size(), nil
}

func fileSHA256(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}
