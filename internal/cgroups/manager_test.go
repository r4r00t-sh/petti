package cgroups_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/r4r00t-sh/petti/internal/cgroups"
)

func TestDetectUnified(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "cgroup.controllers"), []byte("cpu memory pids\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := cgroups.DetectUnified(root); err != nil {
		t.Fatal(err)
	}
	if err := cgroups.DetectUnified(t.TempDir()); err == nil {
		t.Fatal("expected error when cgroup.controllers missing")
	}
}

func TestOOMKilledAndNRThrottled(t *testing.T) {
	path := filepath.Join(t.TempDir(), "petti", "cid")
	if err := os.MkdirAll(path, 0o755); err != nil {
		t.Fatal(err)
	}
	m := &cgroups.Manager{Path: path}
	events := filepath.Join(path, "memory.events")
	if err := os.WriteFile(events, []byte("low 0\noom_kill 0\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	ok, err := m.OOMKilled()
	if err != nil || ok {
		t.Fatalf("want false, got %v %v", ok, err)
	}
	if err := os.WriteFile(events, []byte("oom_kill 3\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	ok, err = m.OOMKilled()
	if err != nil || !ok {
		t.Fatalf("want true, got %v %v", ok, err)
	}

	stat := filepath.Join(path, "cpu.stat")
	if err := os.WriteFile(stat, []byte("nr_periods 10\nnr_throttled 4\nthrottled_usec 100\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	n, err := m.NRThrottled()
	if err != nil || n != 4 {
		t.Fatalf("nr_throttled=%d %v", n, err)
	}
}

func TestDestroyEmptyDir(t *testing.T) {
	path := filepath.Join(t.TempDir(), "emptycg")
	if err := os.MkdirAll(path, 0o755); err != nil {
		t.Fatal(err)
	}
	m := &cgroups.Manager{Path: path}
	if err := m.Destroy(); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("still present: %v", err)
	}
}

func TestDestroyMissingOK(t *testing.T) {
	m := &cgroups.Manager{Path: filepath.Join(t.TempDir(), "gone")}
	if err := m.Destroy(); err != nil {
		t.Fatal(err)
	}
}
