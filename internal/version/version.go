// Package version holds build-time identity for the petti binary.
// Version, Commit, and BuildDate are injected via -ldflags in the Makefile.
package version

import (
	"fmt"
	"runtime"
)

// Injected at link time; fall back to "dev" when unset.
var (
	Version   = "dev"
	Commit    = "dev"
	BuildDate = "dev"
)

// String returns the canonical version line printed by -v / --v / --version / version.
// Format: petti version 0.1.0 (commit abc1234, built 2026-11-05, go1.xx, linux/amd64)
func String() string {
	return fmt.Sprintf(
		"petti version %s (commit %s, built %s, %s, %s/%s)",
		Version,
		Commit,
		BuildDate,
		runtime.Version(),
		runtime.GOOS,
		runtime.GOARCH,
	)
}
