package runtime_test

import (
	"testing"

	"github.com/r4r00t-sh/petti/internal/runtime"
)

func TestParseSyncFD(t *testing.T) {
	fd, err := runtime.ParseSyncFD("3")
	if err != nil || fd != 3 {
		t.Fatalf("got %d %v", fd, err)
	}
	if _, err := runtime.ParseSyncFD("x"); err == nil {
		t.Fatal("expected error")
	}
}

func TestExitErrorMessage(t *testing.T) {
	err := &runtime.ExitError{Code: 7}
	if err.Error() == "" {
		t.Fatal("empty")
	}
}
