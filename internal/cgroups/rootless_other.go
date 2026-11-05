//go:build !linux

package cgroups

import "fmt"

// CurrentPath is unavailable off Linux.
func CurrentPath(mount string) (string, error) {
	return "", fmt.Errorf("Petti requires Linux")
}

// CreateRootless is unavailable off Linux.
func CreateRootless(id string, lim Limits) (*Manager, string, error) {
	return nil, "", fmt.Errorf("Petti requires Linux")
}
