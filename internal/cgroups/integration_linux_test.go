//go:build linux && integration

package cgroups_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/r4r00t-sh/petti/internal/cgroups"
)

func requireRoot(t *testing.T) {
	t.Helper()
	if os.Geteuid() != 0 {
		t.Skip("requires root")
	}
}

func TestCreateAttachAndCleanup(t *testing.T) {
	requireRoot(t)
	if err := cgroups.DetectUnified(""); err != nil {
		t.Fatal(err)
	}

	id := "test" + strconv.Itoa(os.Getpid())
	m, err := cgroups.Create("", id, cgroups.Limits{
		MemoryBytes: 32 * 1024 * 1024,
		CPUs:        0.25,
		Pids:        16,
	})
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := m.Destroy(); err != nil {
			t.Errorf("destroy: %v", err)
		}
		if _, err := os.Stat(m.Path); !os.IsNotExist(err) {
			t.Errorf("cgroup still exists: %v", err)
		}
	}()

	mem, err := os.ReadFile(filepath.Join(m.Path, "memory.max"))
	if err != nil || strings.TrimSpace(string(mem)) != "33554432" {
		t.Fatalf("memory.max=%q %v", mem, err)
	}

	cmd := exec.Command("sleep", "60")
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	defer func() { _ = cmd.Process.Kill(); _, _ = cmd.Process.Wait() }()

	if err := m.AddProc(cmd.Process.Pid); err != nil {
		t.Fatal(err)
	}
	procs, err := os.ReadFile(filepath.Join(m.Path, "cgroup.procs"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(procs), strconv.Itoa(cmd.Process.Pid)) {
		t.Fatalf("pid not in cgroup.procs: %q", procs)
	}
}

func TestPidsLimitBlocksFork(t *testing.T) {
	requireRoot(t)
	id := "pids" + strconv.Itoa(os.Getpid())
	m, err := cgroups.Create("", id, cgroups.Limits{Pids: 5})
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = m.Destroy() }()

	// One shell + a few short sleeps should hit the limit. Kill children so we
	// do not leave host processes around for later tests.
	cmd := exec.Command("sh", "-c", `
fails=0
n=0
while [ "$n" -lt 20 ]; do
  sleep 2 &
  if [ $? -ne 0 ]; then fails=$((fails+1)); fi
  n=$((n+1))
done
echo FAILS=$fails
kill $(jobs -p) 2>/dev/null || true
wait 2>/dev/null || true
`)
	var buf strings.Builder
	cmd.Stdout = &buf
	cmd.Stderr = &buf
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	if err := m.AddProc(cmd.Process.Pid); err != nil {
		_ = cmd.Process.Kill()
		_, _ = cmd.Process.Wait()
		t.Fatal(err)
	}
	waitErr := cmd.Wait()
	out := buf.String()
	current, _ := os.ReadFile(filepath.Join(m.Path, "pids.current"))
	t.Logf("output=%q current=%s err=%v", out, current, waitErr)
	cur, _ := strconv.Atoi(strings.TrimSpace(string(current)))
	// Hitting the limit often leaves the shell unable to echo FAILS= ("Cannot fork").
	if strings.Contains(out, "Cannot fork") || strings.Contains(out, "Resource temporarily unavailable") {
		if cur > 5 {
			t.Fatalf("pids.current=%d exceeded max=5", cur)
		}
		return
	}
	if !strings.Contains(out, "FAILS=") {
		t.Fatalf("unexpected output: %s", out)
	}
	failsLine := ""
	for _, line := range strings.Split(out, "\n") {
		if strings.HasPrefix(line, "FAILS=") {
			failsLine = line
		}
	}
	fails, _ := strconv.Atoi(strings.TrimPrefix(failsLine, "FAILS="))
	if fails == 0 && cur > 5 {
		t.Fatalf("pids limit not enforced: fails=%d current=%d", fails, cur)
	}
}

func TestCPUThrottle(t *testing.T) {
	requireRoot(t)
	id := "cpu" + strconv.Itoa(os.Getpid())
	m, err := cgroups.Create("", id, cgroups.Limits{CPUs: 0.1})
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = m.Destroy() }()

	cmd := exec.Command("sh", "-c", "while true; do :; done")
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	if err := m.AddProc(cmd.Process.Pid); err != nil {
		_ = cmd.Process.Kill()
		t.Fatal(err)
	}
	time.Sleep(1500 * time.Millisecond)
	_ = cmd.Process.Kill()
	_, _ = cmd.Process.Wait()

	n, err := m.NRThrottled()
	if err != nil {
		t.Fatal(err)
	}
	if n == 0 {
		t.Fatalf("expected cpu throttling (nr_throttled=0)")
	}
}
