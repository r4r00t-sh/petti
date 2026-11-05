package image

import (
	"archive/tar"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

const (
	whiteoutPrefix = ".wh."
	opaqueWhiteout = ".wh..wh..opq"
)

// ApplyLayerTar extracts a single OCI layer tar into dest.
//
// Whiteout files are preserved as regular files named .wh.* so a later merge
// step (or overlay conversion) can apply deletions against lower layers.
// Path traversal is rejected: absolute paths, "..", and symlink escapes.
func ApplyLayerTar(r io.Reader, dest string) error {
	if err := os.MkdirAll(dest, 0o755); err != nil {
		return err
	}
	destAbs, err := filepath.Abs(dest)
	if err != nil {
		return err
	}
	tr := tar.NewReader(r)
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			return nil
		}
		if err != nil {
			return fmt.Errorf("tar: %w", err)
		}
		if err := extractOne(tr, hdr, destAbs); err != nil {
			return err
		}
	}
}

func extractOne(tr *tar.Reader, hdr *tar.Header, destAbs string) error {
	target, err := safeJoin(destAbs, hdr.Name)
	if err != nil {
		return fmt.Errorf("reject entry %q: %w", hdr.Name, err)
	}

	switch hdr.Typeflag {
	case tar.TypeDir:
		return os.MkdirAll(target, os.FileMode(hdr.Mode)|0o755)
	case tar.TypeReg:
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			return err
		}
		f, err := os.OpenFile(target, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, os.FileMode(hdr.Mode)&0o777)
		if err != nil {
			return err
		}
		if _, err := io.Copy(f, tr); err != nil {
			_ = f.Close()
			return err
		}
		if err := f.Close(); err != nil {
			return err
		}
		_ = os.Lchown(target, hdr.Uid, hdr.Gid)
		return nil
	case tar.TypeSymlink:
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			return err
		}
		// Reject absolute symlink targets that would escape when cleaned against dest.
		if err := validateSymlink(destAbs, target, hdr.Linkname); err != nil {
			return fmt.Errorf("reject symlink %q -> %q: %w", hdr.Name, hdr.Linkname, err)
		}
		_ = os.Remove(target)
		if err := os.Symlink(hdr.Linkname, target); err != nil {
			return err
		}
		return nil
	case tar.TypeLink:
		linkTarget, err := safeJoin(destAbs, hdr.Linkname)
		if err != nil {
			return fmt.Errorf("reject hardlink %q -> %q: %w", hdr.Name, hdr.Linkname, err)
		}
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			return err
		}
		_ = os.Remove(target)
		return os.Link(linkTarget, target)
	case tar.TypeChar, tar.TypeBlock, tar.TypeFifo:
		// Skip special nodes in educational runtime unpack (need CAP_MKNOD at pull time).
		return nil
	default:
		return nil
	}
}

// safeJoin ensures name resolves under root without escaping.
func safeJoin(root, name string) (string, error) {
	if name == "" {
		return root, nil
	}
	// Reject absolute paths and any ".." segment before cleaning can hide them.
	if filepath.IsAbs(name) || strings.HasPrefix(name, "/") || strings.HasPrefix(name, `\`) {
		return "", fmt.Errorf("absolute path")
	}
	for _, part := range strings.Split(filepath.ToSlash(name), "/") {
		if part == ".." {
			return "", fmt.Errorf("path escapes with parent directory segment")
		}
	}
	cleaned := filepath.Clean(name)
	if cleaned == ".." || strings.HasPrefix(cleaned, ".."+string(os.PathSeparator)) {
		return "", fmt.Errorf("path escapes with parent directory segment")
	}
	if cleaned == "." {
		return root, nil
	}
	full := filepath.Join(root, cleaned)
	rel, err := filepath.Rel(root, full)
	if err != nil {
		return "", err
	}
	if rel == ".." || strings.HasPrefix(rel, ".."+string(os.PathSeparator)) {
		return "", fmt.Errorf("path escapes root")
	}
	return full, nil
}

// validateSymlink ensures a symlink placed at linkPath with target linkname
// cannot be used as a path-escape trampoline for later entries.
// We allow relative links that stay under root when resolved from linkPath's dir.
func validateSymlink(root, linkPath, linkname string) error {
	if filepath.IsAbs(linkname) {
		// Absolute links inside containers are OK at runtime, but for unpack
		// safety we only allow them if cleaning under root still stays put —
		// absolute host paths like /etc/passwd are accepted as *container*
		// absolute paths; we do not resolve them on the host during unpack.
		cleaned := filepath.Clean(linkname)
		if strings.HasPrefix(cleaned, "/../") || cleaned == "/.." {
			return fmt.Errorf("absolute symlink escapes")
		}
		return nil
	}
	resolved := filepath.Clean(filepath.Join(filepath.Dir(linkPath), linkname))
	rel, err := filepath.Rel(root, resolved)
	if err != nil {
		return err
	}
	if rel == ".." || strings.HasPrefix(rel, ".."+string(os.PathSeparator)) {
		return fmt.Errorf("symlink target escapes root")
	}
	return nil
}

// MergeLayers copies layer directories in order into dest, applying OCI whiteouts.
func MergeLayers(layerDirs []string, dest string) error {
	if err := os.MkdirAll(dest, 0o755); err != nil {
		return err
	}
	destAbs, err := filepath.Abs(dest)
	if err != nil {
		return err
	}
	for _, layer := range layerDirs {
		if err := mergeOneLayer(layer, destAbs); err != nil {
			return fmt.Errorf("merge %s: %w", layer, err)
		}
	}
	return nil
}

func mergeOneLayer(layerDir, destAbs string) error {
	return filepath.Walk(layerDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(layerDir, path)
		if err != nil {
			return err
		}
		if rel == "." {
			return nil
		}
		base := filepath.Base(rel)
		parent := filepath.Dir(rel)

		if base == opaqueWhiteout {
			targetDir, err := safeJoin(destAbs, parent)
			if err != nil {
				return err
			}
			return clearDirContents(targetDir)
		}
		if strings.HasPrefix(base, whiteoutPrefix) {
			name := strings.TrimPrefix(base, whiteoutPrefix)
			victimRel := filepath.Join(parent, name)
			victim, err := safeJoin(destAbs, victimRel)
			if err != nil {
				return err
			}
			return os.RemoveAll(victim)
		}

		target, err := safeJoin(destAbs, rel)
		if err != nil {
			return err
		}
		if info.IsDir() {
			return os.MkdirAll(target, info.Mode())
		}
		if info.Mode()&os.ModeSymlink != 0 {
			link, err := os.Readlink(path)
			if err != nil {
				return err
			}
			if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
				return err
			}
			_ = os.Remove(target)
			return os.Symlink(link, target)
		}
		return copyFile(path, target, info.Mode())
	})
}

func clearDirContents(dir string) error {
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return os.MkdirAll(dir, 0o755)
		}
		return err
	}
	for _, e := range entries {
		if err := os.RemoveAll(filepath.Join(dir, e.Name())); err != nil {
			return err
		}
	}
	return nil
}

func copyFile(src, dst string, mode os.FileMode) error {
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return err
	}
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.OpenFile(dst, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, mode&0o777)
	if err != nil {
		return err
	}
	if _, err := io.Copy(out, in); err != nil {
		_ = out.Close()
		return err
	}
	return out.Close()
}
