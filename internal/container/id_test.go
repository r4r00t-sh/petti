package container_test

import (
	"testing"

	"github.com/r4r00t-sh/petti/internal/container"
)

func TestGenerateID(t *testing.T) {
	a, err := container.GenerateID()
	if err != nil {
		t.Fatal(err)
	}
	b, err := container.GenerateID()
	if err != nil {
		t.Fatal(err)
	}
	if len(a) != 24 {
		t.Fatalf("len = %d, want 24", len(a))
	}
	if a == b {
		t.Fatalf("ids should differ: %s", a)
	}
}

func TestHostnamePrefix(t *testing.T) {
	if got := container.HostnamePrefix("abcdefghijklmnop"); got != "abcdefghijkl" {
		t.Fatalf("got %q", got)
	}
	if got := container.HostnamePrefix("abc"); got != "abc" {
		t.Fatalf("got %q", got)
	}
}
