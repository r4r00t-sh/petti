//go:build !linux

package runtime

import "fmt"

// Process is a stub on non-Linux.
type Process struct {
	Pid int
}

// Start reports that Petti requires Linux.
func Start(cfg Config) (*Process, error) {
	return nil, fmt.Errorf("Petti requires Linux (namespaces, mounts, cgroups)")
}

// Wait is a stub.
func (p *Process) Wait() error {
	return fmt.Errorf("Petti requires Linux")
}

// Init reports that Petti requires Linux.
func Init(cfg Config) error {
	return fmt.Errorf("Petti requires Linux (namespaces, mounts, cgroups)")
}

// ParseSyncFD is unused on non-Linux builds.
func ParseSyncFD(s string) (int, error) {
	return -1, fmt.Errorf("Petti requires Linux")
}

// ExecConfig is a stub.
type ExecConfig struct {
	TargetPid int
	Workdir   string
	User      UserSpec
	Env       []string
	Args      []string
}

// ExecInNamespaces requires Linux.
func ExecInNamespaces(cfg ExecConfig) error {
	return fmt.Errorf("Petti requires Linux")
}
