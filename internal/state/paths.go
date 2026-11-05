// Package state manages on-disk Petti metadata paths and atomic writes.
package state

import (
	"fmt"
	"os"
	"path/filepath"
)

const (
	envRoot = "PETTI_ROOT"
)

// DataRoot returns the Petti data directory.
// Override with PETTI_ROOT. Otherwise root → /var/lib/petti, else ~/.local/share/petti.
func DataRoot() (string, error) {
	if v := os.Getenv(envRoot); v != "" {
		return filepath.Abs(v)
	}
	if os.Geteuid() == 0 {
		return "/var/lib/petti", nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("home dir: %w", err)
	}
	return filepath.Join(home, ".local", "share", "petti"), nil
}

// Layout holds well-known subpaths under the data root.
type Layout struct {
	Root   string
	Blobs  string
	Layers string
	Images string // image index JSON records
}

// NewLayout resolves and creates the store directories.
func NewLayout(root string) (*Layout, error) {
	if root == "" {
		var err error
		root, err = DataRoot()
		if err != nil {
			return nil, err
		}
	}
	root, err := filepath.Abs(root)
	if err != nil {
		return nil, err
	}
	l := &Layout{
		Root:   root,
		Blobs:  filepath.Join(root, "blobs", "sha256"),
		Layers: filepath.Join(root, "layers"),
		Images: filepath.Join(root, "images"),
	}
	for _, d := range []string{l.Root, l.Blobs, l.Layers, l.Images} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			return nil, fmt.Errorf("mkdir %s: %w", d, err)
		}
	}
	return l, nil
}

// BlobPath returns the content-addressed blob path for a sha256 hex digest (no algo prefix).
func (l *Layout) BlobPath(hex string) string {
	return filepath.Join(l.Blobs, hex)
}

// LayerPath returns the unpacked layer directory for a diff ID hex digest.
func (l *Layout) LayerPath(diffHex string) string {
	return filepath.Join(l.Layers, "sha256", diffHex)
}
