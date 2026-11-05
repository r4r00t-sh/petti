package container

import (
	"fmt"
	"strings"
	"time"

	"github.com/r4r00t-sh/petti/internal/network"
)

// Status is the container state machine.
type Status string

const (
	StatusCreated  Status = "created"
	StatusRunning  Status = "running"
	StatusExited   Status = "exited"
	StatusStopped  Status = "stopped"
	StatusRemoving Status = "removing"
)

// Volume is a bind mount host:container[:ro].
type Volume struct {
	Source   string
	Target   string
	ReadOnly bool
}

// ParseVolume parses host:container or host:container:ro.
func ParseVolume(s string) (Volume, error) {
	parts := strings.Split(s, ":")
	if len(parts) < 2 || parts[0] == "" || parts[1] == "" {
		return Volume{}, fmt.Errorf("invalid volume %q (want host:container[:ro])", s)
	}
	v := Volume{Source: parts[0], Target: parts[1]}
	if !strings.HasPrefix(v.Target, "/") {
		return Volume{}, fmt.Errorf("volume destination must be absolute: %q", v.Target)
	}
	if len(parts) >= 3 {
		switch parts[2] {
		case "ro":
			v.ReadOnly = true
		case "rw", "":
		default:
			return Volume{}, fmt.Errorf("invalid volume mode %q", parts[2])
		}
	}
	// host paths with colons (rare) — not supported in this educational parser.
	if len(parts) > 3 {
		return Volume{}, fmt.Errorf("invalid volume %q", s)
	}
	return v, nil
}

// Record is persisted container metadata.
type Record struct {
	ID         string            `json:"id"`
	Name       string            `json:"name,omitempty"`
	Image      string            `json:"image,omitempty"`
	Status     Status            `json:"status"`
	Pid        int               `json:"pid,omitempty"`
	ExitCode   int               `json:"exitCode,omitempty"`
	CreatedAt  time.Time         `json:"createdAt"`
	StartedAt  time.Time         `json:"startedAt,omitempty"`
	FinishedAt time.Time         `json:"finishedAt,omitempty"`
	Hostname   string            `json:"hostname"`
	Workdir    string            `json:"workdir"`
	Env        []string          `json:"env,omitempty"`
	User       string            `json:"user,omitempty"`
	Command    []string          `json:"command"`
	Volumes    []Volume          `json:"volumes,omitempty"`
	Detach     bool              `json:"detach"`
	AutoRemove bool              `json:"autoRemove"`
	Tty        bool              `json:"tty"`
	Memory     string            `json:"memory,omitempty"`
	CPUs       string            `json:"cpus,omitempty"`
	PidsLimit  string            `json:"pidsLimit,omitempty"`
	Rootfs     string            `json:"rootfs"` // merged path used by runtime
	BundleDir  string            `json:"bundleDir"`
	LogPath    string            `json:"logPath"`
	CgroupPath string            `json:"cgroupPath,omitempty"`
	Labels     map[string]string `json:"labels,omitempty"`

	// Networking (Milestone 6)
	NetworkMode string                `json:"networkMode,omitempty"`
	IP          string                `json:"ip,omitempty"`
	Gateway     string                `json:"gateway,omitempty"`
	HostVeth    string                `json:"hostVeth,omitempty"`
	Ports       []network.PortMapping `json:"ports,omitempty"`
	Rootless    bool                  `json:"rootless,omitempty"`
}
