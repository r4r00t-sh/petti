//go:build linux

package runtime

import (
	"fmt"
	"os"
	"path/filepath"

	"golang.org/x/sys/unix"
)

// pivotIntoRootfs switches the mount-namespace root to rootfs and detaches the
// old host root so it is no longer reachable.
//
// Why pivot_root instead of chroot:
//
//	chroot(2) only changes the *process* root directory. The old root mount
//	remains in the namespace, so a process that can obtain a file descriptor to
//	the old root (classic "chroot escape": mkdir + chroot + chdir("..") loops,
//	or /proc/self/cwd tricks) can walk back onto the host filesystem.
//
//	pivot_root(2) swaps the root mount for the calling mount namespace and lets
//	us unmount the old root (MNT_DETACH). After that there is no path from
//	inside the container to the host's filesystem tree. That is why runtimes
//	use pivot_root, not chroot alone.
func pivotIntoRootfs(rootfs string) error {
	putOld := filepath.Join(rootfs, ".oldroot")
	if err := os.MkdirAll(putOld, 0o700); err != nil {
		return fmt.Errorf("mkdir put_old: %w", err)
	}

	// rootfs must be a mount point — setupMounts bind-mounts it onto itself.
	if err := unix.PivotRoot(rootfs, putOld); err != nil {
		return fmt.Errorf("pivot_root(%s, %s): %w", rootfs, putOld, err)
	}
	if err := unix.Chdir("/"); err != nil {
		return fmt.Errorf("chdir / after pivot_root: %w", err)
	}

	// Drop the old host root. MNT_DETACH so busy mounts still detach from the tree.
	if err := unix.Unmount("/.oldroot", unix.MNT_DETACH); err != nil {
		return fmt.Errorf("unmount old root: %w", err)
	}
	if err := os.Remove("/.oldroot"); err != nil {
		// Non-fatal: directory may be busy briefly; isolation already holds.
		_ = err
	}
	return nil
}
