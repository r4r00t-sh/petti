// Package runtime implements the re-exec init process, namespaces, and mounts.
//
// Go's runtime is multithreaded, so we cannot safely unshare namespaces and
// continue in the same process. Instead the parent re-execs itself as "__init"
// with Cloneflags set; that child becomes PID 1 of the new PID namespace.
package runtime

import (
	"fmt"
	"io"
	"os"

	"github.com/r4r00t-sh/petti/internal/cgroups"
	"github.com/r4r00t-sh/petti/internal/network"
	"github.com/r4r00t-sh/petti/internal/seccomp"
)

// VolumeSpec is a bind mount applied under the rootfs before pivot_root.
type VolumeSpec struct {
	Source   string
	Target   string // absolute path inside the container
	ReadOnly bool
}

// Config is everything the parent and __init child need to start a process
// inside fresh namespaces.
type Config struct {
	ID          string
	Rootfs      string
	Hostname    string
	Workdir     string
	Env         []string
	User        UserSpec
	Args        []string
	Volumes     []VolumeSpec
	Resources   cgroups.Limits
	CgroupMount string
	CgroupPath  string // absolute cgroup dir; set for __init when applying deferred pids
	PidsLimit   int64  // deferred pids.max applied in __init just before exec
	Detach      bool   // if true, Start does not Wait and skips Pdeathsig
	SyncFD      int

	// Networking (Milestone 6). Mode defaults to bridge when empty.
	NetworkMode string
	NetworkRoot string // data root for IPAM
	Ports       []network.PortMapping

	// Rootless (Milestone 7): CLONE_NEWUSER + uid/gid mapping.
	Rootless bool

	// Hardening (Milestone 8).
	CapAdd   []string
	CapDrop  []string
	Seccomp  seccomp.Mode // empty → default
	ReadOnly bool
	KeepCaps []uintptr // resolved allow-list; nil → defaults inside Init

	Stdout io.Writer
	Stderr io.Writer
	Stdin  io.Reader
}

// ExitError means the container process exited with a non-zero status.
type ExitError struct {
	Code int
	OOM  bool
}

func (e *ExitError) Error() string {
	if e.OOM {
		return "container was OOM-killed"
	}
	return fmt.Sprintf("container exited with code %d", e.Code)
}

// DefaultIO fills nil stdio streams from the process.
func (c *Config) DefaultIO() {
	if c.Stdin == nil {
		c.Stdin = os.Stdin
	}
	if c.Stdout == nil {
		c.Stdout = os.Stdout
	}
	if c.Stderr == nil {
		c.Stderr = os.Stderr
	}
}

// DefaultEnv is the base environment inside a container before --env overrides.
func DefaultEnv(user UserSpec) []string {
	home := "/root"
	if user.Set && user.UID != 0 {
		home = "/"
	}
	return []string{
		"PATH=/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin",
		"HOME=" + home,
		"TERM=xterm",
	}
}
