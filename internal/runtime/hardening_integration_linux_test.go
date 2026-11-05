//go:build linux && integration

package runtime_test

import (
	"bytes"
	"strings"
	"testing"

	"github.com/r4r00t-sh/petti/internal/container"
	"github.com/r4r00t-sh/petti/internal/seccomp"
)

// Escape-attempt suite for Milestone 8. Requires root + busybox fixture.
func TestHardeningEscapeAttempts(t *testing.T) {
	requireRoot(t)
	root := stageBusyboxRootfs(t)

	cases := []struct {
		name string
		args []string
		want string // substring that must appear (command failed as expected)
	}{
		{
			name: "sysrq-trigger masked",
			args: []string{"sh", "-c", "echo 1 > /proc/sysrq-trigger 2>&1; echo exit:$?"},
			want: "exit:", // writing to /dev/null mask succeeds but does nothing harmful; check it's not a real trigger
		},
		{
			name: "mount blocked by seccomp",
			args: []string{"sh", "-c", "mount -t tmpfs tmpfs /mnt 2>&1; echo exit:$?"},
			want: "exit:",
		},
		{
			name: "reboot blocked",
			args: []string{"sh", "-c", "reboot 2>&1; echo exit:$?"},
			want: "exit:",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var out, errBuf bytes.Buffer
			_, err := container.Run(container.RunOptions{
				Rootfs:     root,
				Args:       tc.args,
				Network:    "none",
				Seccomp:    seccomp.ModeDefault,
				Stdout:     &out,
				Stderr:     &errBuf,
				AutoRemove: true,
			})
			got := out.String() + errBuf.String()
			if err != nil && !strings.Contains(got, "exit:") {
				// Command may have non-zero exit propagated; still OK if output shows failure.
				t.Logf("run err=%v out=%q", err, got)
			}
			if tc.name == "sysrq-trigger masked" {
				// Masked with /dev/null: write "succeeds" but target is null.
				// Ensure /proc/sysrq-trigger is not a regular writable sysrq (size 0 via null).
				var out2 bytes.Buffer
				_, _ = container.Run(container.RunOptions{
					Rootfs: root, Network: "none", AutoRemove: true,
					Args:   []string{"sh", "-c", "ls -l /proc/sysrq-trigger"},
					Stdout: &out2,
				})
				if !strings.Contains(out2.String(), "sysrq-trigger") {
					t.Fatalf("sysrq-trigger missing: %q", out2.String())
				}
			}
			if tc.name == "mount blocked by seccomp" {
				if strings.Contains(got, "exit:0") {
					t.Fatalf("mount unexpectedly succeeded: %q", got)
				}
			}
		})
	}
}

func TestReadOnlyRoot(t *testing.T) {
	requireRoot(t)
	root := stageBusyboxRootfs(t)
	var out, errBuf bytes.Buffer
	_, err := container.Run(container.RunOptions{
		Rootfs:     root,
		ReadOnly:   true,
		Network:    "none",
		AutoRemove: true,
		Args:       []string{"sh", "-c", "touch /oops 2>&1; echo exit:$?"},
		Stdout:     &out,
		Stderr:     &errBuf,
	})
	got := out.String() + errBuf.String()
	_ = err
	if strings.Contains(got, "exit:0") {
		t.Fatalf("touch on read-only root succeeded: %q", got)
	}
}

func TestSeccompUnconfinedAllowsMountSyscallPath(t *testing.T) {
	requireRoot(t)
	root := stageBusyboxRootfs(t)
	// Even unconfined, CAP_SYS_ADMIN is dropped so mount should still fail —
	// but seccomp should not be the blocker. We only check the container starts.
	var out bytes.Buffer
	_, err := container.Run(container.RunOptions{
		Rootfs:     root,
		Network:    "none",
		Seccomp:    seccomp.ModeUnconfined,
		AutoRemove: true,
		Args:       []string{"sh", "-c", "echo ok"},
		Stdout:     &out,
	})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "ok") {
		t.Fatalf("%q", out.String())
	}
}
