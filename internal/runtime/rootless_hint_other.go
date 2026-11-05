//go:build !linux

package runtime

func rootlessStartHint() string {
	return "rootless userns failed"
}
