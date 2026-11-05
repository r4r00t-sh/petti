//go:build !linux

package network

import "fmt"

// EnsureBridge is unavailable off Linux.
func EnsureBridge() error {
	return fmt.Errorf("Petti networking requires Linux")
}

// EnableIPForward is unavailable off Linux.
func EnableIPForward() error {
	return fmt.Errorf("Petti networking requires Linux")
}

// Setup is unavailable off Linux.
func Setup(opts SetupOpts) (*Endpoint, error) {
	return nil, fmt.Errorf("Petti networking requires Linux")
}

// Teardown is a no-op off Linux.
func Teardown(dataRoot string, ep Endpoint) error {
	return nil
}

// SetLoopbackUp is unavailable off Linux.
func SetLoopbackUp() error {
	return fmt.Errorf("Petti networking requires Linux")
}
