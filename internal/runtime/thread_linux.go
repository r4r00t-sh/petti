//go:build linux

package runtime

import "runtime"

// runtimeLockOSThread pins this goroutine to its OS thread. Namespace and
// mount operations are per-thread kernel state; without this, the Go
// scheduler could migrate us mid-setup onto a thread that never unshared.
func runtimeLockOSThread() {
	runtime.LockOSThread()
}
