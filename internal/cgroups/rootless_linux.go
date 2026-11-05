//go:build linux

package cgroups

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// CurrentPath returns the process's cgroup v2 path under mount (absolute).
func CurrentPath(mount string) (string, error) {
	if mount == "" {
		mount = DefaultMount
	}
	data, err := os.ReadFile("/proc/self/cgroup")
	if err != nil {
		return "", err
	}
	rel := ""
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		// v2: 0::/path
		if strings.HasPrefix(line, "0::") {
			rel = strings.TrimPrefix(line, "0::")
			break
		}
	}
	if rel == "" {
		return "", fmt.Errorf("no cgroup v2 path in /proc/self/cgroup")
	}
	if !strings.HasPrefix(rel, "/") {
		rel = "/" + rel
	}
	return filepath.Join(mount, rel), nil
}

// CreateRootless tries to create a container cgroup under the current user's
// delegated hierarchy. On failure it returns (nil, warning, nil) when limits
// were requested so the caller can continue without enforcement, or
// (nil, "", nil) when no limits were set.
func CreateRootless(id string, lim Limits) (*Manager, string, error) {
	base, err := CurrentPath(DefaultMount)
	if err != nil {
		if lim.Set() {
			return nil, fmt.Sprintf("cgroup limits skipped: %v", err), nil
		}
		return nil, "", nil
	}

	// Prefer a petti/ directory under the current cgroup.
	parent := filepath.Join(base, PettiSlice)
	path := filepath.Join(parent, id)

	m := &Manager{
		Mount: DefaultMount,
		ID:    id,
		Path:  path,
		Lim:   lim,
	}

	if err := os.MkdirAll(parent, 0o755); err != nil {
		if lim.Set() {
			return nil, fmt.Sprintf("cgroup limits skipped (no systemd user delegation?): %v", err), nil
		}
		return nil, "", nil
	}
	// Best-effort enable controllers on the parent we own.
	_ = enableControllers(base, []string{"memory", "cpu", "pids"})
	_ = enableControllers(parent, []string{"memory", "cpu", "pids"})

	if err := os.MkdirAll(path, 0o755); err != nil {
		if lim.Set() {
			return nil, fmt.Sprintf("cgroup limits skipped: %v", err), nil
		}
		return nil, "", nil
	}
	if lim.Set() {
		if err := m.applyLimits(); err != nil {
			_ = m.Destroy()
			return nil, fmt.Sprintf("cgroup limits skipped: %v", err), nil
		}
	}
	return m, "", nil
}
