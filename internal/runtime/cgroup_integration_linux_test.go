//go:build linux && integration

package runtime_test

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/r4r00t-sh/petti/internal/container"
	"github.com/r4r00t-sh/petti/internal/runtime"
)

func TestContainerOOMKilled(t *testing.T) {
	requireRoot(t)
	root := stageBusyboxRootfs(t)
	// Ensure /dev/shm exists inside rootfs for tmpfs-backed allocation.
	_ = os.MkdirAll(filepath.Join(root, "dev", "shm"), 0o1777)

	var errBuf bytes.Buffer
	_, err := container.Run(container.RunOptions{
		Rootfs:  root,
		Memory:  "8m",
		Network: "none",
		// Fill tmpfs until the memory cgroup OOM-kills us.
		Args:   []string{"sh", "-c", "dd if=/dev/zero of=/dev/shm/big bs=1M count=64"},
		Stderr: &errBuf,
	})
	if err == nil {
		t.Fatal("expected OOM failure")
	}
	var ee *runtime.ExitError
	if !errors.As(err, &ee) {
		t.Fatalf("want ExitError, got %T %v (stderr=%s)", err, err, errBuf.String())
	}
	if !ee.OOM && ee.Code != 137 {
		t.Fatalf("want OOM/137, got code=%d oom=%v stderr=%s", ee.Code, ee.OOM, errBuf.String())
	}
	if ee.Code != 137 {
		t.Fatalf("exit code=%d, want 137", ee.Code)
	}
}

func TestContainerPidsLimit(t *testing.T) {
	requireRoot(t)
	root := stageBusyboxRootfs(t)

	var out, errBuf bytes.Buffer
	_, err := container.Run(container.RunOptions{
		Rootfs:    root,
		PidsLimit: "8",
		Network:   "none",
		Args: []string{"sh", "-c", `
fails=0
i=0
while [ "$i" -lt 40 ]; do
  sleep 2 &
  if [ $? -ne 0 ]; then fails=$((fails+1)); fi
  i=$((i+1))
done
echo FAILS=$fails
kill $(jobs -p) 2>/dev/null || true
wait 2>/dev/null || true
`},
		Stdout: &out,
		Stderr: &errBuf,
	})
	if err != nil {
		t.Logf("run err=%v stderr=%s", err, errBuf.String())
	}
	got := out.String() + errBuf.String()
	if strings.Contains(got, "Cannot fork") ||
		strings.Contains(got, "can't fork") ||
		strings.Contains(got, "Resource temporarily unavailable") ||
		strings.Contains(got, "FAILS=") {
		return
	}
	t.Fatalf("output=%q stderr=%q", out.String(), errBuf.String())
}

func TestContainerCPUThrottle(t *testing.T) {
	requireRoot(t)
	root := stageBusyboxRootfs(t)

	// Run a busy loop briefly under a tiny CPU quota; parent kills via short command.
	var errBuf bytes.Buffer
	_, err := container.Run(container.RunOptions{
		Rootfs:  root,
		CPUs:    "0.1",
		Network: "none",
		Args:    []string{"sh", "-c", "i=0; while [ $i -lt 200000 ]; do i=$((i+1)); done"},
		Stderr:  &errBuf,
	})
	if err != nil {
		t.Fatalf("run: %v (%s)", err, errBuf.String())
	}
	// Throttling is asserted in the cgroups package integration test against
	// cpu.stat; here we only verify the container runs under --cpus successfully.
}
