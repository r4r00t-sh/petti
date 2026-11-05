package cgroups

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

// Limits are resource constraints applied to a container cgroup.
// Zero-value fields mean "unset" (leave the kernel default / max).
type Limits struct {
	MemoryBytes int64   // memory.max; also forces memory.swap.max=0 when set
	CPUs        float64 // cpu.max quota
	Pids        int64   // pids.max
}

// Set reports whether any limit is configured.
func (l Limits) Set() bool {
	return l.MemoryBytes > 0 || l.CPUs > 0 || l.Pids > 0
}

// Manager owns one container cgroup at <mount>/petti/<id>.
type Manager struct {
	Mount string
	ID    string
	Path  string
	Lim   Limits
}

// Create detects cgroup v2, ensures the petti parent + container cgroup exist,
// enables controllers, and writes limit files.
func Create(mount, id string, lim Limits) (*Manager, error) {
	if mount == "" {
		mount = DefaultMount
	}
	if id == "" {
		return nil, fmt.Errorf("cgroup id is required")
	}
	if err := DetectUnified(mount); err != nil {
		return nil, err
	}

	m := &Manager{
		Mount: mount,
		ID:    id,
		Path:  filepath.Join(mount, PettiSlice, id),
		Lim:   lim,
	}
	if err := m.ensureHierarchy(); err != nil {
		return nil, err
	}
	if err := m.applyLimits(); err != nil {
		_ = m.Destroy()
		return nil, err
	}
	return m, nil
}

func (m *Manager) ensureHierarchy() error {
	parent := filepath.Join(m.Mount, PettiSlice)
	// Enable controllers on the root so we can create a delegated subtree.
	if err := enableControllers(m.Mount, []string{"memory", "cpu", "pids"}); err != nil {
		return fmt.Errorf("enable controllers on %s: %w", m.Mount, err)
	}
	if err := os.MkdirAll(parent, 0o755); err != nil {
		return fmt.Errorf("mkdir %s: %w", parent, err)
	}
	if err := enableControllers(parent, []string{"memory", "cpu", "pids"}); err != nil {
		return fmt.Errorf("enable controllers on %s: %w", parent, err)
	}
	if err := os.MkdirAll(m.Path, 0o755); err != nil {
		return fmt.Errorf("mkdir container cgroup: %w", err)
	}
	return nil
}

func enableControllers(dir string, want []string) error {
	path := filepath.Join(dir, "cgroup.subtree_control")
	availablePath := filepath.Join(dir, "cgroup.controllers")
	availRaw, err := os.ReadFile(availablePath)
	if err != nil {
		return err
	}
	avail := map[string]struct{}{}
	for _, c := range strings.Fields(string(availRaw)) {
		avail[c] = struct{}{}
	}

	curRaw, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	enabled := map[string]struct{}{}
	for _, c := range strings.Fields(string(curRaw)) {
		enabled[c] = struct{}{}
	}

	var b strings.Builder
	for _, c := range want {
		if _, ok := avail[c]; !ok {
			return fmt.Errorf("controller %q not available in %s (have %q)", c, dir, strings.TrimSpace(string(availRaw)))
		}
		if _, ok := enabled[c]; ok {
			continue
		}
		if b.Len() > 0 {
			b.WriteByte(' ')
		}
		b.WriteByte('+')
		b.WriteString(c)
	}
	if b.Len() == 0 {
		return nil
	}
	if err := os.WriteFile(path, []byte(b.String()), 0o644); err != nil {
		return fmt.Errorf("write %s: %w", path, err)
	}
	return nil
}

func (m *Manager) applyLimits() error {
	if m.Lim.MemoryBytes > 0 {
		if err := writeFile(filepath.Join(m.Path, "memory.max"), strconv.FormatInt(m.Lim.MemoryBytes, 10)); err != nil {
			return err
		}
		// Disable swap so --memory is a hard ceiling (predictable OOM tests).
		swapPath := filepath.Join(m.Path, "memory.swap.max")
		if _, err := os.Stat(swapPath); err == nil {
			if err := writeFile(swapPath, "0"); err != nil {
				return err
			}
		}
	}
	if m.Lim.CPUs > 0 {
		if err := writeFile(filepath.Join(m.Path, "cpu.max"), CPUMax(m.Lim.CPUs)); err != nil {
			return err
		}
	}
	if m.Lim.Pids > 0 {
		if err := m.SetPidsMax(m.Lim.Pids); err != nil {
			return err
		}
	}
	return nil
}

// SetPidsMax writes pids.max. Safe to call when pids.current is already higher
// (existing tasks stay; new forks fail until the count drops).
func (m *Manager) SetPidsMax(max int64) error {
	if m == nil || max <= 0 {
		return nil
	}
	return writeFile(filepath.Join(m.Path, "pids.max"), strconv.FormatInt(max, 10))
}

// ApplyPidsMaxAt writes pids.max at an absolute cgroup directory (used by __init
// immediately before exec so the Go re-exec runtime is not constrained).
func ApplyPidsMaxAt(cgroupPath string, max int64) error {
	if cgroupPath == "" || max <= 0 {
		return nil
	}
	return writeFile(filepath.Join(cgroupPath, "pids.max"), strconv.FormatInt(max, 10))
}

func writeFile(path, content string) error {
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		return fmt.Errorf("write %s: %w", path, err)
	}
	return nil
}

// AddProc moves pid into this cgroup (must be the host PID).
// Call this before releasing the container's sync pipe. Note: pids.max for
// `petti run` is deferred until __init is about to exec (Go needs threads).
func (m *Manager) AddProc(pid int) error {
	if pid <= 0 {
		return fmt.Errorf("invalid pid %d", pid)
	}
	path := filepath.Join(m.Path, "cgroup.procs")
	if err := os.WriteFile(path, []byte(strconv.Itoa(pid)), 0o644); err != nil {
		return fmt.Errorf("cgroup.procs add %d: %w", pid, err)
	}
	return nil
}

// OOMKilled reports whether memory.events shows oom_kill > 0.
func (m *Manager) OOMKilled() (bool, error) {
	raw, err := os.ReadFile(filepath.Join(m.Path, "memory.events"))
	if err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}
		return false, err
	}
	for _, line := range strings.Split(string(raw), "\n") {
		fields := strings.Fields(line)
		if len(fields) != 2 {
			continue
		}
		if fields[0] == "oom_kill" {
			n, err := strconv.ParseInt(fields[1], 10, 64)
			if err != nil {
				return false, err
			}
			return n > 0, nil
		}
	}
	return false, nil
}

// NRThrottled returns cpu.stat nr_throttled (0 if unavailable).
func (m *Manager) NRThrottled() (uint64, error) {
	raw, err := os.ReadFile(filepath.Join(m.Path, "cpu.stat"))
	if err != nil {
		if os.IsNotExist(err) {
			return 0, nil
		}
		return 0, err
	}
	for _, line := range strings.Split(string(raw), "\n") {
		fields := strings.Fields(line)
		if len(fields) != 2 {
			continue
		}
		if fields[0] == "nr_throttled" {
			return strconv.ParseUint(fields[1], 10, 64)
		}
	}
	return 0, nil
}

// Destroy removes the container cgroup directory. Safe to call multiple times.
func (m *Manager) Destroy() error {
	if m == nil || m.Path == "" {
		return nil
	}
	// Ask the kernel to kill any leftover members (cgroup v2). Only write when
	// the file already exists — on a real cgroupfs the kernel provides it;
	// creating it on a normal directory would leave debris that blocks rmdir.
	killPath := filepath.Join(m.Path, "cgroup.kill")
	if _, err := os.Stat(killPath); err == nil {
		_ = os.WriteFile(killPath, []byte("1"), 0o644)
	}

	var last error
	for i := 0; i < 20; i++ {
		err := os.Remove(m.Path)
		if err == nil || os.IsNotExist(err) {
			return nil
		}
		last = err
		time.Sleep(25 * time.Millisecond)
	}
	return fmt.Errorf("remove cgroup %s: %w", m.Path, last)
}
