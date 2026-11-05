//go:build linux

package runtime

import (
	"fmt"
	"os"
	"path/filepath"

	"golang.org/x/sys/unix"
)

// setupMounts prepares the container filesystem view under rootfs.
//
// Ordering matters:
//  1. Make mount propagation private so container mounts never leak to the host
//     (see mount_namespaces(7) and shared subtrees).
//  2. Bind-mount rootfs onto itself so it is a mount point (required by pivot_root).
//  3. Mount /proc, /dev (+ nodes), /dev/pts, /dev/shm, /sys under rootfs.
func setupMounts(rootfs string) error {
	// Step 1: never let mounts propagate to/from the host.
	if err := unix.Mount("", "/", "", unix.MS_REC|unix.MS_PRIVATE, ""); err != nil {
		return fmt.Errorf("make / MS_PRIVATE: %w", err)
	}

	// Step 2: bind rootfs to itself so it is a distinct mount point.
	if err := unix.Mount(rootfs, rootfs, "", unix.MS_BIND|unix.MS_REC, ""); err != nil {
		return fmt.Errorf("bind rootfs: %w", err)
	}

	proc := filepath.Join(rootfs, "proc")
	if err := os.MkdirAll(proc, 0o755); err != nil {
		return err
	}
	if err := unix.Mount("proc", proc, "proc", unix.MS_NOSUID|unix.MS_NOEXEC|unix.MS_NODEV, ""); err != nil {
		return fmt.Errorf("mount proc: %w", err)
	}

	if err := setupDev(rootfs); err != nil {
		return err
	}

	sys := filepath.Join(rootfs, "sys")
	if err := os.MkdirAll(sys, 0o755); err != nil {
		return err
	}
	// Read-only sysfs: containers should not tweak host sysctls via /sys.
	if err := unix.Mount("sysfs", sys, "sysfs", unix.MS_RDONLY|unix.MS_NOSUID|unix.MS_NOEXEC|unix.MS_NODEV, ""); err != nil {
		return fmt.Errorf("mount sysfs: %w", err)
	}

	if err := maskSensitivePaths(rootfs); err != nil {
		return err
	}
	return nil
}

func setupDev(rootfs string) error {
	dev := filepath.Join(rootfs, "dev")
	if err := os.MkdirAll(dev, 0o755); err != nil {
		return err
	}
	if err := unix.Mount("tmpfs", dev, "tmpfs", unix.MS_NOSUID|unix.MS_STRICTATIME, "mode=755,size=65536k"); err != nil {
		return fmt.Errorf("mount /dev tmpfs: %w", err)
	}

	nodes := []struct {
		name string
		mode uint32
		dev  uint64
	}{
		{"null", unix.S_IFCHR | 0o666, unix.Mkdev(1, 3)},
		{"zero", unix.S_IFCHR | 0o666, unix.Mkdev(1, 5)},
		{"full", unix.S_IFCHR | 0o666, unix.Mkdev(1, 7)},
		{"random", unix.S_IFCHR | 0o666, unix.Mkdev(1, 8)},
		{"urandom", unix.S_IFCHR | 0o666, unix.Mkdev(1, 9)},
		{"tty", unix.S_IFCHR | 0o666, unix.Mkdev(5, 0)},
	}
	for _, n := range nodes {
		path := filepath.Join(dev, n.name)
		if err := unix.Mknod(path, n.mode, int(n.dev)); err != nil {
			// Fall back to bind-mounting the host node (same major/minor).
			if err := bindHostDev(filepath.Join("/dev", n.name), path); err != nil {
				return fmt.Errorf("mknod %s: %w", n.name, err)
			}
			continue
		}
		if err := unix.Chmod(path, n.mode&0o777); err != nil {
			return fmt.Errorf("chmod %s: %w", n.name, err)
		}
	}

	pts := filepath.Join(dev, "pts")
	if err := os.MkdirAll(pts, 0o755); err != nil {
		return err
	}
	if err := unix.Mount("devpts", pts, "devpts", unix.MS_NOSUID|unix.MS_NOEXEC, "newinstance,ptmxmode=0666,mode=0620,gid=5"); err != nil {
		// gid=5 (tty) may not exist in the rootfs; retry without gid.
		if err2 := unix.Mount("devpts", pts, "devpts", unix.MS_NOSUID|unix.MS_NOEXEC, "newinstance,ptmxmode=0666,mode=0620"); err2 != nil {
			return fmt.Errorf("mount devpts: %w", err2)
		}
	}
	ptmx := filepath.Join(dev, "ptmx")
	if err := os.Symlink("pts/ptmx", ptmx); err != nil && !os.IsExist(err) {
		return fmt.Errorf("symlink ptmx: %w", err)
	}

	shm := filepath.Join(dev, "shm")
	if err := os.MkdirAll(shm, 0o1777); err != nil {
		return err
	}
	if err := unix.Mount("shm", shm, "tmpfs", unix.MS_NOSUID|unix.MS_NODEV, "mode=1777,size=65536k"); err != nil {
		return fmt.Errorf("mount /dev/shm: %w", err)
	}

	return nil
}

func bindHostDev(host, target string) error {
	f, err := os.Create(target)
	if err != nil {
		return err
	}
	_ = f.Close()
	return unix.Mount(host, target, "", unix.MS_BIND, "")
}
