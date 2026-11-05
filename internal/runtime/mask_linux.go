//go:build linux

package runtime

import (
	"fmt"
	"os"
	"path/filepath"

	"golang.org/x/sys/unix"
)

// Sensitive paths masked inside the container (Docker-like).
var maskedPaths = []string{
	"/proc/kcore",
	"/proc/sysrq-trigger",
	"/proc/timer_list",
	"/proc/acpi",
	"/sys/firmware",
}

// Paths remounted read-only under the container's /proc.
var readOnlyProcPaths = []string{
	"/proc/sys",
	"/proc/irq",
	"/proc/bus",
	"/proc/fs",
}

// maskSensitivePaths bind-mounts /dev/null (files) or empty tmpfs (dirs) over
// sensitive host interfaces, and remounts selected /proc subtrees read-only.
// Call after /proc and /sys are mounted under rootfs, before pivot_root.
func maskSensitivePaths(rootfs string) error {
	for _, p := range maskedPaths {
		target := filepath.Join(rootfs, filepath.Clean(p))
		if err := maskOne(target); err != nil {
			// Path may be absent on this kernel/image — skip.
			if os.IsNotExist(err) {
				continue
			}
			// Parent missing (e.g. /proc/acpi): skip quietly.
			if _, statErr := os.Stat(filepath.Dir(target)); os.IsNotExist(statErr) {
				continue
			}
			return fmt.Errorf("mask %s: %w", p, err)
		}
	}
	for _, p := range readOnlyProcPaths {
		target := filepath.Join(rootfs, filepath.Clean(p))
		if _, err := os.Stat(target); err != nil {
			continue
		}
		// Bind to itself then remount read-only.
		if err := unix.Mount(target, target, "", unix.MS_BIND|unix.MS_REC, ""); err != nil {
			continue
		}
		if err := unix.Mount("", target, "", unix.MS_BIND|unix.MS_REMOUNT|unix.MS_RDONLY|unix.MS_REC|unix.MS_NOSUID|unix.MS_NODEV|unix.MS_NOEXEC, ""); err != nil {
			return fmt.Errorf("remount ro %s: %w", p, err)
		}
	}
	return nil
}

func maskOne(target string) error {
	st, err := os.Lstat(target)
	if err != nil {
		return err
	}
	if st.IsDir() {
		if err := unix.Mount("tmpfs", target, "tmpfs", unix.MS_RDONLY|unix.MS_NOSUID|unix.MS_NODEV|unix.MS_NOEXEC, "size=4k,mode=555"); err != nil {
			return err
		}
		return nil
	}
	// File: bind /dev/null over it.
	if err := unix.Mount("/dev/null", target, "", unix.MS_BIND, ""); err != nil {
		return err
	}
	return nil
}

// remountRootReadOnly remounts "/" read-only after pivot_root (--read-only).
func remountRootReadOnly() error {
	if err := unix.Mount("", "/", "", unix.MS_REMOUNT|unix.MS_BIND|unix.MS_RDONLY, ""); err != nil {
		return fmt.Errorf("remount / read-only: %w", err)
	}
	return nil
}
