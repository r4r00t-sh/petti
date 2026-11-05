//go:build linux

package runtime

import (
	"fmt"

	"golang.org/x/sys/unix"
)

func applyUser(u UserSpec) error {
	if !u.Set {
		return nil
	}
	gid := u.GID
	if err := unix.Setgroups([]int{gid}); err != nil {
		return fmt.Errorf("setgroups: %w", err)
	}
	if err := unix.Setgid(gid); err != nil {
		return fmt.Errorf("setgid %d: %w", gid, err)
	}
	if err := unix.Setuid(u.UID); err != nil {
		return fmt.Errorf("setuid %d: %w", u.UID, err)
	}
	return nil
}
