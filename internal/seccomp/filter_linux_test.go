//go:build linux

package seccomp

import (
	"testing"
	"unsafe"

	"golang.org/x/sys/unix"
)

func TestBuildFilterSize(t *testing.T) {
	arch, err := nativeArch()
	if err != nil {
		t.Fatal(err)
	}
	blocked := defaultBlocked()
	f := buildFilter(arch, blocked)
	// arch check (3) + load nr (1) + 2 per blocked + allow (1)
	want := 3 + 1 + 2*len(blocked) + 1
	if len(f) != want {
		t.Fatalf("len=%d want %d", len(f), want)
	}
	if unsafe.Sizeof(unix.SockFilter{}) != 8 {
		t.Fatalf("SockFilter size %d", unsafe.Sizeof(unix.SockFilter{}))
	}
}

func TestApplyUnconfined(t *testing.T) {
	if err := Apply(ModeUnconfined); err != nil {
		t.Fatal(err)
	}
}
