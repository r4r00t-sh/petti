//go:build linux && integration

package runtime_test

import (
	"bytes"
	"debug/elf"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/r4r00t-sh/petti/internal/container"
	"github.com/r4r00t-sh/petti/internal/runtime"
	"golang.org/x/sys/unix"
)

func requireRoot(t *testing.T) {
	t.Helper()
	if os.Geteuid() != 0 {
		t.Skip("integration tests require root (sudo make integration)")
	}
}

func stageBusyboxRootfs(t *testing.T) string {
	t.Helper()
	busybox := findStaticBusybox(t)
	root := t.TempDir()
	for _, d := range []string{"bin", "proc", "sys", "dev", "tmp", "etc", "root", "workdir"} {
		if err := os.MkdirAll(filepath.Join(root, d), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	dst := filepath.Join(root, "bin", "busybox")
	in, err := os.ReadFile(busybox)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(dst, in, 0o755); err != nil {
		t.Fatal(err)
	}
	applets := []string{
		"sh", "ps", "hostname", "cat", "echo", "true", "false", "ls",
		"sleep", "pwd", "id", "grep", "mkdir", "rm", "dd",
	}
	for _, a := range applets {
		if err := os.Symlink("busybox", filepath.Join(root, "bin", a)); err != nil {
			t.Fatal(err)
		}
	}
	return root
}

// findStaticBusybox returns a statically linked busybox. A dynamic host binary
// cannot exec inside the fixture rootfs (no ld.so/libc after pivot_root).
func findStaticBusybox(t *testing.T) string {
	t.Helper()
	candidates := []string{}
	if p, err := exec.LookPath("busybox"); err == nil {
		candidates = append(candidates, p)
	}
	candidates = append(candidates, "/bin/busybox", "/usr/bin/busybox")
	seen := map[string]bool{}
	for _, p := range candidates {
		if p == "" || seen[p] {
			continue
		}
		seen[p] = true
		st, err := os.Stat(p)
		if err != nil || st.IsDir() {
			continue
		}
		ok, err := elfIsStatic(p)
		if err != nil || !ok {
			continue
		}
		return p
	}
	t.Skip("static busybox not found (install busybox-static); dynamic busybox cannot run in the fixture rootfs")
	return ""
}

func elfIsStatic(path string) (bool, error) {
	f, err := elf.Open(path)
	if err != nil {
		return false, err
	}
	defer f.Close()
	for _, p := range f.Progs {
		if p.Type == elf.PT_INTERP {
			return false, nil
		}
	}
	return true, nil
}

func TestRunHostnameAndPID1(t *testing.T) {
	requireRoot(t)
	root := stageBusyboxRootfs(t)

	var out, errBuf bytes.Buffer
	_, err := container.Run(container.RunOptions{
		Rootfs:   root,
		Hostname: "petti-test",
		Network:  "none",
		Args:     []string{"sh", "-c", "hostname; ps"},
		Stdout:   &out,
		Stderr:   &errBuf,
	})
	if err != nil {
		t.Fatalf("run: %v\nstderr=%s\nstdout=%s", err, errBuf.String(), out.String())
	}
	got := out.String()
	if !strings.Contains(got, "petti-test") {
		t.Fatalf("hostname missing in output: %q", got)
	}
	if !strings.Contains(got, "1 ") {
		t.Fatalf("expected PID 1 in ps output: %q", got)
	}
}

func TestRunDefaultHostnamePrefix(t *testing.T) {
	requireRoot(t)
	root := stageBusyboxRootfs(t)

	var out, errBuf bytes.Buffer
	res, err := container.Run(container.RunOptions{
		Rootfs:  root,
		Network: "none",
		Args:    []string{"hostname"},
		Stdout:  &out,
		Stderr:  &errBuf,
	})
	if err != nil {
		t.Fatalf("run: %v (%s)", err, errBuf.String())
	}
	hn := strings.TrimSpace(out.String())
	prefix := container.HostnamePrefix(res.ID)
	if hn != prefix {
		t.Fatalf("hostname %q != id prefix %q (id=%s)", hn, prefix, res.ID)
	}
}

func TestRunExitCode(t *testing.T) {
	requireRoot(t)
	root := stageBusyboxRootfs(t)

	_, err := container.Run(container.RunOptions{
		Rootfs:  root,
		Network: "none",
		Args:    []string{"sh", "-c", "exit 42"},
	})
	if err == nil {
		t.Fatal("expected exit error")
	}
	var ee *runtime.ExitError
	if !errors.As(err, &ee) {
		t.Fatalf("want ExitError, got %T %v", err, err)
	}
	if ee.Code != 42 {
		t.Fatalf("exit code = %d, want 42", ee.Code)
	}
}

func TestHostMountsUntouched(t *testing.T) {
	requireRoot(t)
	root := stageBusyboxRootfs(t)

	before, err := exec.Command("findmnt", "-r", "-o", "TARGET").Output()
	if err != nil {
		t.Fatalf("findmnt: %v", err)
	}

	var errBuf bytes.Buffer
	_, err = container.Run(container.RunOptions{
		Rootfs:  root,
		Network: "none",
		Args:    []string{"true"},
		Stderr:  &errBuf,
	})
	if err != nil {
		t.Fatalf("run: %v (%s)", err, errBuf.String())
	}

	after, err := exec.Command("findmnt", "-r", "-o", "TARGET").Output()
	if err != nil {
		t.Fatalf("findmnt: %v", err)
	}
	if string(before) != string(after) {
		t.Fatalf("host mount table changed\nbefore:\n%s\nafter:\n%s", before, after)
	}
}

func TestPivotRootIsolatesHost(t *testing.T) {
	requireRoot(t)
	root := stageBusyboxRootfs(t)

	// Unique marker that exists only on the host, outside the container rootfs.
	marker := filepath.Join(os.TempDir(), fmt.Sprintf("petti-host-secret-%d", os.Getpid()))
	if err := os.WriteFile(marker, []byte("secret"), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Remove(marker) })

	var out, errBuf bytes.Buffer
	_, err := container.Run(container.RunOptions{
		Rootfs:  root,
		Network: "none",
		Args: []string{"sh", "-c",
			// Cannot see host marker; cd .. from / must stay at /.
			"test ! -e " + marker + " && cd / && cd .. && pwd && test ! -d /home"},
		Stdout: &out,
		Stderr: &errBuf,
	})
	if err != nil {
		t.Fatalf("run: %v\nstderr=%s\nstdout=%s", err, errBuf.String(), out.String())
	}
	if strings.TrimSpace(out.String()) != "/" {
		t.Fatalf("pwd after cd .. = %q, want /", out.String())
	}
}

func TestWorkdirEnvEntrypointUser(t *testing.T) {
	requireRoot(t)
	root := stageBusyboxRootfs(t)

	var out, errBuf bytes.Buffer
	_, err := container.Run(container.RunOptions{
		Rootfs:     root,
		Workdir:    "/workdir",
		Env:        []string{"PETTI_GREETING=hello"},
		Entrypoint: []string{"/bin/sh"},
		Args:       []string{"-c", "pwd; echo $PETTI_GREETING; id -u"},
		User:       "0:0",
		Network:    "none",
		Stdout:     &out,
		Stderr:     &errBuf,
	})
	if err != nil {
		t.Fatalf("run: %v (%s)", err, errBuf.String())
	}
	got := out.String()
	if !strings.Contains(got, "/workdir") {
		t.Fatalf("workdir missing: %q", got)
	}
	if !strings.Contains(got, "hello") {
		t.Fatalf("env missing: %q", got)
	}
	if !strings.Contains(got, "0") {
		t.Fatalf("uid missing: %q", got)
	}
}

func TestDefaultCapabilitiesAndNoNewPrivs(t *testing.T) {
	requireRoot(t)
	root := stageBusyboxRootfs(t)

	var out, errBuf bytes.Buffer
	_, err := container.Run(container.RunOptions{
		Rootfs:  root,
		Network: "none",
		Args:    []string{"sh", "-c", "grep -E '^(CapEff|NoNewPrivs):' /proc/self/status"},
		Stdout:  &out,
		Stderr:  &errBuf,
	})
	if err != nil {
		t.Fatalf("run: %v (%s)", err, errBuf.String())
	}
	got := out.String()
	if !strings.Contains(got, "NoNewPrivs:\t1") && !strings.Contains(got, "NoNewPrivs:	1") {
		t.Fatalf("NoNewPrivs not set: %q", got)
	}

	want := expectedCapEffHex()
	var gotEff string
	for _, line := range strings.Split(got, "\n") {
		if strings.HasPrefix(line, "CapEff:") {
			fields := strings.Fields(line)
			if len(fields) >= 2 {
				gotEff = strings.ToLower(fields[1])
			}
		}
	}
	if gotEff == "" {
		t.Fatalf("CapEff missing: %q", got)
	}
	// Compare numerically — /proc may zero-pad to 16 hex digits.
	gotN, err := strconv.ParseUint(gotEff, 16, 64)
	if err != nil {
		t.Fatal(err)
	}
	wantN, err := strconv.ParseUint(want, 16, 64)
	if err != nil {
		t.Fatal(err)
	}
	if gotN != wantN {
		t.Fatalf("CapEff = %s (%d), want %s (%d)\nstatus:\n%s", gotEff, gotN, want, wantN, got)
	}
	// SYS_ADMIN bit must be clear (cap 21).
	if gotN&(1<<unix.CAP_SYS_ADMIN) != 0 {
		t.Fatalf("CAP_SYS_ADMIN still set: %s", gotEff)
	}
}

func expectedCapEffHex() string {
	caps := []uintptr{
		unix.CAP_CHOWN,
		unix.CAP_DAC_OVERRIDE,
		unix.CAP_FOWNER,
		unix.CAP_FSETID,
		unix.CAP_KILL,
		unix.CAP_SETGID,
		unix.CAP_SETUID,
		unix.CAP_SETPCAP,
		unix.CAP_NET_BIND_SERVICE,
		unix.CAP_NET_RAW,
		unix.CAP_SYS_CHROOT,
		unix.CAP_MKNOD,
		unix.CAP_AUDIT_WRITE,
		unix.CAP_SETFCAP,
	}
	var mask uint64
	for _, c := range caps {
		mask |= 1 << c
	}
	return strconv.FormatUint(mask, 16)
}
