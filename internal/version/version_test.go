package version_test

import (
	"runtime"
	"strings"
	"testing"

	"github.com/r4r00t-sh/petti/internal/version"
)

func TestStringFormat(t *testing.T) {
	s := version.String()
	if !strings.HasPrefix(s, "petti version ") {
		t.Fatalf("prefix: %q", s)
	}
	if !strings.Contains(s, "commit ") {
		t.Fatalf("missing commit: %q", s)
	}
	if !strings.Contains(s, "built ") {
		t.Fatalf("missing built: %q", s)
	}
	if !strings.Contains(s, runtime.Version()) {
		t.Fatalf("missing go version: %q", s)
	}
	wantGO := runtime.GOOS + "/" + runtime.GOARCH
	if !strings.Contains(s, wantGO) {
		t.Fatalf("missing platform %s: %q", wantGO, s)
	}
}
