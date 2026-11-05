//go:build !linux

package seccomp

import "fmt"

// BlockedNames is empty off Linux.
func BlockedNames() []string { return nil }

// Apply requires Linux.
func Apply(mode Mode) error {
	return fmt.Errorf("Petti seccomp requires Linux")
}
