//go:build !linux

package rootfs

import "fmt"

// Overlay is a stub on non-Linux.
type Overlay struct {
	Merged string
	Copied bool
}

// MountOverlay requires Linux.
func MountOverlay(dir string, lowerDirs []string) (*Overlay, error) {
	return nil, fmt.Errorf("Petti requires Linux")
}

// MountOverlayUserxattr requires Linux.
func MountOverlayUserxattr(dir string, lowerDirs []string) (*Overlay, error) {
	return nil, fmt.Errorf("Petti requires Linux")
}

// NewCopiedRootfs requires Linux.
func NewCopiedRootfs(dir string) (*Overlay, error) {
	return nil, fmt.Errorf("Petti requires Linux")
}

// Unmount is a no-op stub.
func (o *Overlay) Unmount() error { return nil }

// UnmountPath is a no-op stub.
func UnmountPath(path string) error { return nil }

// BindMount requires Linux.
func BindMount(source, target string, readOnly bool) error {
	return fmt.Errorf("Petti requires Linux")
}

// MaterializeOverlayWhiteouts requires Linux.
func MaterializeOverlayWhiteouts(layerDir string) error {
	return fmt.Errorf("Petti requires Linux")
}
