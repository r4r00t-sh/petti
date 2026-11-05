//go:build !linux

package filelock

import (
	"fmt"
	"os"
)

// Lock is a best-effort exclusive lock using O_EXCL lock files on non-Linux.
func Lock(path string) (unlock func(), err error) {
	f, err := os.OpenFile(path+".lockheld", os.O_CREATE|os.O_EXCL|os.O_RDWR, 0o644)
	if err != nil {
		return nil, fmt.Errorf("lock %s: %w", path, err)
	}
	return func() {
		_ = f.Close()
		_ = os.Remove(path + ".lockheld")
	}, nil
}
