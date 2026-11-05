//go:build linux

package runtime_test

import (
	"testing"

	"github.com/r4r00t-sh/petti/internal/runtime"
)

func TestResolveCapsDropAdd(t *testing.T) {
	keep, err := runtime.ResolveCaps([]string{"SYS_ADMIN"}, []string{"NET_RAW"})
	if err != nil {
		t.Fatal(err)
	}
	hasAdmin, hasRaw, hasChown := false, false, false
	for _, c := range keep {
		name := capName(c)
		switch name {
		case "SYS_ADMIN":
			hasAdmin = true
		case "NET_RAW":
			hasRaw = true
		case "CHOWN":
			hasChown = true
		}
	}
	if !hasAdmin || hasRaw || !hasChown {
		t.Fatalf("admin=%v raw=%v chown=%v keep=%v", hasAdmin, hasRaw, hasChown, keepNames(keep))
	}
}

func TestResolveCapsDropAll(t *testing.T) {
	keep, err := runtime.ResolveCaps([]string{"KILL"}, []string{"ALL"})
	if err != nil {
		t.Fatal(err)
	}
	if len(keep) != 1 {
		t.Fatalf("want only KILL, got %v", keepNames(keep))
	}
}

func TestParseCapName(t *testing.T) {
	_, err := runtime.ParseCapName("CAP_SYS_ADMIN")
	if err != nil {
		t.Fatal(err)
	}
	_, err = runtime.ParseCapName("sys_admin")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := runtime.ParseCapName("NOT_A_CAP"); err == nil {
		t.Fatal("expected error")
	}
}

func keepNames(caps []uintptr) []string {
	out := make([]string, 0, len(caps))
	for _, c := range caps {
		out = append(out, capName(c))
	}
	return out
}

func capName(c uintptr) string {
	for _, n := range append(runtime.DefaultCapNames(), "SYS_ADMIN", "NET_ADMIN", "SYS_MODULE") {
		v, err := runtime.ParseCapName(n)
		if err == nil && v == c {
			return n
		}
	}
	return "?"
}
