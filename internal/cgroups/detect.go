// Package cgroups manages cgroup v2 resource limits for Petti containers.
package cgroups

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

const (
	// DefaultMount is the unified cgroup v2 mount point on modern Linux.
	DefaultMount = "/sys/fs/cgroup"
	// PettiSlice is the parent directory under which per-container cgroups live.
	PettiSlice = "petti"
)

// DetectUnified verifies that path is a cgroup v2 hierarchy.
// Petti does not support legacy (v1) or hybrid-only controllers.
func DetectUnified(mount string) error {
	if mount == "" {
		mount = DefaultMount
	}
	controllers := filepath.Join(mount, "cgroup.controllers")
	if _, err := os.Stat(controllers); err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("cgroup v2 required: %s missing (legacy cgroup v1 is not supported)", controllers)
		}
		return fmt.Errorf("cgroup detect: %w", err)
	}
	// If the classic v1 memory controller dir exists alongside, still OK as long
	// as the unified tree is present — but refuse a pure v1 mount with no controllers file.
	data, err := os.ReadFile(controllers)
	if err != nil {
		return fmt.Errorf("read cgroup.controllers: %w", err)
	}
	if strings.TrimSpace(string(data)) == "" {
		return fmt.Errorf("cgroup v2 required: cgroup.controllers is empty")
	}
	return nil
}
