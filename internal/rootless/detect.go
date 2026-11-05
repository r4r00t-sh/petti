package rootless

import "os"

// Enabled reports whether Petti should use rootless mode.
// Non-root always enables it; root can force it with force=true (--rootless).
func Enabled(force bool) bool {
	if force {
		return true
	}
	return os.Geteuid() != 0
}
