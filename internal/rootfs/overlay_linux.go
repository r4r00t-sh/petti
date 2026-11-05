//go:build linux

// Package rootfs mounts container filesystems (overlayfs).
package rootfs

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"golang.org/x/sys/unix"
)

// Overlay is a mounted overlayfs instance (or a plain copied rootfs when Copied).
type Overlay struct {
	Merged string
	Upper  string
	Work   string
	Lower  []string
	Copied bool // true when Merged is a directory copy, not an overlay mount
}

// MountOverlay mounts an overlayfs at dir/merged using lowerdirs (bottom→top order
// in the slice: index 0 is the bottom-most layer).
//
// lowerdir page-size limit: we pass paths as /proc/self/fd/N so the mount option
// string stays short regardless of how deep the real layer paths are (see
// docs/DESIGN-DECISIONS.md).
func MountOverlay(dir string, lowerDirs []string) (*Overlay, error) {
	return mountOverlay(dir, lowerDirs, false)
}

// MountOverlayUserxattr is the rootless overlay mount (kernel 5.11+ userxattr).
// Whiteouts use user.overlay.opaque; char-device whiteouts still need CAP_MKNOD
// in the user namespace — callers should fall back to a layer copy on error.
func MountOverlayUserxattr(dir string, lowerDirs []string) (*Overlay, error) {
	return mountOverlay(dir, lowerDirs, true)
}

func mountOverlay(dir string, lowerDirs []string, userxattr bool) (*Overlay, error) {
	if len(lowerDirs) == 0 {
		return nil, fmt.Errorf("overlay: no lower directories")
	}
	o := &Overlay{
		Merged: filepath.Join(dir, "merged"),
		Upper:  filepath.Join(dir, "upper"),
		Work:   filepath.Join(dir, "work"),
		Lower:  append([]string(nil), lowerDirs...),
	}
	for _, d := range []string{o.Merged, o.Upper, o.Work} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			return nil, err
		}
	}

	for _, low := range lowerDirs {
		var err error
		if userxattr {
			err = MaterializeOverlayWhiteoutsRootless(low)
		} else {
			err = MaterializeOverlayWhiteouts(low)
		}
		if err != nil {
			return nil, fmt.Errorf("whiteouts %s: %w", low, err)
		}
	}

	var fds []*os.File
	defer func() {
		for _, f := range fds {
			_ = f.Close()
		}
	}()
	parts := make([]string, 0, len(lowerDirs))
	for i := len(lowerDirs) - 1; i >= 0; i-- {
		f, err := os.Open(lowerDirs[i])
		if err != nil {
			return nil, err
		}
		fds = append(fds, f)
		parts = append(parts, fmt.Sprintf("/proc/self/fd/%d", f.Fd()))
	}
	opts := fmt.Sprintf("lowerdir=%s,upperdir=%s,workdir=%s",
		strings.Join(parts, ":"), o.Upper, o.Work)
	if userxattr {
		opts += ",userxattr"
	}
	if err := unix.Mount("overlay", o.Merged, "overlay", 0, opts); err != nil {
		return nil, fmt.Errorf("mount overlay: %w (opts len=%d)", err, len(opts))
	}
	return o, nil
}

// NewCopiedRootfs marks dir/merged as a non-mounted rootfs (copy-merge fallback).
func NewCopiedRootfs(dir string) (*Overlay, error) {
	merged := filepath.Join(dir, "merged")
	if err := os.MkdirAll(merged, 0o755); err != nil {
		return nil, err
	}
	return &Overlay{Merged: merged, Copied: true}, nil
}

// Unmount tears down the overlay mount (no-op for Copied rootfs).
func (o *Overlay) Unmount() error {
	if o == nil || o.Copied {
		return nil
	}
	return UnmountPath(o.Merged)
}

// UnmountPath detaches a mount at path (best-effort).
func UnmountPath(path string) error {
	if path == "" {
		return nil
	}
	if err := unix.Unmount(path, unix.MNT_DETACH); err != nil && !os.IsNotExist(err) && err != unix.EINVAL {
		return fmt.Errorf("unmount %s: %w", path, err)
	}
	return nil
}

// MaterializeOverlayWhiteouts converts OCI .wh.* markers into overlayfs whiteouts
// (char devices 0:0) and opaque xattrs. Safe to call multiple times.
func MaterializeOverlayWhiteouts(layerDir string) error {
	return materializeWhiteouts(layerDir, "trusted.overlay.opaque", ".petti-overlay-whiteouts")
}

// MaterializeOverlayWhiteoutsRootless uses user.overlay.opaque (needs userxattr mount).
func MaterializeOverlayWhiteoutsRootless(layerDir string) error {
	return materializeWhiteouts(layerDir, "user.overlay.opaque", ".petti-overlay-whiteouts-user")
}

func materializeWhiteouts(layerDir, opaqueAttr, markerName string) error {
	marker := filepath.Join(layerDir, markerName)
	if _, err := os.Stat(marker); err == nil {
		return nil
	}
	err := filepath.Walk(layerDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		base := filepath.Base(path)
		if base == ".petti-unpacked" || strings.HasPrefix(base, ".petti-overlay-whiteouts") {
			return nil
		}
		if info.IsDir() {
			return nil
		}
		parent := filepath.Dir(path)
		if base == ".wh..wh..opq" {
			if err := unix.Setxattr(parent, opaqueAttr, []byte("y"), 0); err != nil {
				return fmt.Errorf("set opaque on %s: %w", parent, err)
			}
			return os.Remove(path)
		}
		if strings.HasPrefix(base, ".wh.") {
			name := strings.TrimPrefix(base, ".wh.")
			target := filepath.Join(parent, name)
			_ = os.RemoveAll(target)
			if err := unix.Mknod(target, unix.S_IFCHR, int(unix.Mkdev(0, 0))); err != nil {
				return fmt.Errorf("whiteout mknod %s: %w", target, err)
			}
			return os.Remove(path)
		}
		return nil
	})
	if err != nil {
		return err
	}
	return os.WriteFile(marker, []byte("1\n"), 0o644)
}

// BindMount mounts source at target (bind). If readOnly, remounts MS_RDONLY.
func BindMount(source, target string, readOnly bool) error {
	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		return err
	}
	// Ensure target exists (file vs dir).
	st, err := os.Stat(source)
	if err != nil {
		return fmt.Errorf("volume source %s: %w", source, err)
	}
	if st.IsDir() {
		if err := os.MkdirAll(target, 0o755); err != nil {
			return err
		}
	} else {
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			return err
		}
		f, err := os.OpenFile(target, os.O_CREATE, 0o644)
		if err != nil {
			return err
		}
		_ = f.Close()
	}
	if err := unix.Mount(source, target, "", unix.MS_BIND|unix.MS_REC, ""); err != nil {
		return fmt.Errorf("bind %s -> %s: %w", source, target, err)
	}
	if readOnly {
		if err := unix.Mount("", target, "", unix.MS_BIND|unix.MS_REMOUNT|unix.MS_RDONLY|unix.MS_REC, ""); err != nil {
			return fmt.Errorf("remount ro %s: %w", target, err)
		}
	}
	return nil
}
