//go:build linux

package rootless

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"syscall"
)

// SysProcMaps returns UidMappings/GidMappings suitable for syscall.SysProcAttr
// when UseNewuidmap is false (single-line proc write done by Go's fork/exec).
func (m Mapping) SysProcMaps() (uids, gids []syscall.SysProcIDMap) {
	for _, x := range m.UID {
		uids = append(uids, syscall.SysProcIDMap{
			ContainerID: x.ContainerID,
			HostID:      x.HostID,
			Size:        x.Size,
		})
	}
	for _, x := range m.GID {
		gids = append(gids, syscall.SysProcIDMap{
			ContainerID: x.ContainerID,
			HostID:      x.HostID,
			Size:        x.Size,
		})
	}
	return uids, gids
}

// Apply writes uid/gid maps for pid. When UseNewuidmap is set, shells out to
// newuidmap/newgidmap; otherwise writes /proc directly (single mapping only).
func (m Mapping) Apply(pid int) error {
	if pid <= 0 {
		return fmt.Errorf("invalid pid")
	}
	if m.UseNewuidmap {
		if err := execIDMap("newuidmap", pid, m.UID); err != nil {
			return err
		}
		return execIDMap("newgidmap", pid, m.GID)
	}
	return writeProcIDMaps(pid, m.UID, m.GID)
}

func execIDMap(bin string, pid int, maps []IDMap) error {
	args := []string{strconv.Itoa(pid)}
	for _, m := range maps {
		args = append(args, strconv.Itoa(m.ContainerID), strconv.Itoa(m.HostID), strconv.Itoa(m.Size))
	}
	cmd := exec.Command(bin, args...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("%s: %w (%s)", bin, err, trim(out))
	}
	return nil
}

func writeProcIDMaps(pid int, uids, gids []IDMap) error {
	// Unprivileged writers must deny setgroups before writing gid_map.
	setgroups := filepath.Join("/proc", strconv.Itoa(pid), "setgroups")
	if err := os.WriteFile(setgroups, []byte("deny"), 0o644); err != nil {
		// Already written / older kernel without setgroups — continue.
		if !os.IsNotExist(err) {
			// ignore EACCES race if Go already wrote maps via SysProcAttr
			_ = err
		}
	}
	uidPath := filepath.Join("/proc", strconv.Itoa(pid), "uid_map")
	gidPath := filepath.Join("/proc", strconv.Itoa(pid), "gid_map")
	if err := os.WriteFile(uidPath, []byte(FormatProcMap(uids)), 0o644); err != nil {
		return fmt.Errorf("uid_map: %w", err)
	}
	if err := os.WriteFile(gidPath, []byte(FormatProcMap(gids)), 0o644); err != nil {
		return fmt.Errorf("gid_map: %w", err)
	}
	return nil
}

func trim(b []byte) string {
	s := string(b)
	if len(s) > 200 {
		return s[:200] + "…"
	}
	return s
}
