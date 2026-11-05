//go:build !linux

package rootless

import (
	"fmt"
	"syscall"
)

// SysProcMaps is unused off Linux.
func (m Mapping) SysProcMaps() (uids, gids []syscall.SysProcIDMap) {
	return nil, nil
}

// Apply is unavailable off Linux.
func (m Mapping) Apply(pid int) error {
	return fmt.Errorf("Petti rootless requires Linux")
}
