//go:build !linux

package runtime

import "fmt"

// DefaultCapNames matches the Linux default set for docs/tests.
func DefaultCapNames() []string {
	return []string{
		"CHOWN", "DAC_OVERRIDE", "FOWNER", "FSETID", "KILL",
		"SETGID", "SETUID", "SETPCAP", "NET_BIND_SERVICE", "NET_RAW",
		"SYS_CHROOT", "MKNOD", "AUDIT_WRITE", "SETFCAP",
	}
}

// ParseCapName is unavailable off Linux.
func ParseCapName(s string) (uintptr, error) {
	return 0, fmt.Errorf("Petti requires Linux")
}

// ResolveCaps is unavailable off Linux.
func ResolveCaps(add, drop []string) ([]uintptr, error) {
	return nil, fmt.Errorf("Petti requires Linux")
}
