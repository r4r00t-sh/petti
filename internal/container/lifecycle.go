package container

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"syscall"
	"time"

	"github.com/r4r00t-sh/petti/internal/cgroups"
	"github.com/r4r00t-sh/petti/internal/network"
	"github.com/r4r00t-sh/petti/internal/rootfs"
)

// Stop sends SIGTERM then SIGKILL after timeoutSeconds.
func Stop(idOrName string, timeoutSeconds int) error {
	store, err := OpenStore("")
	if err != nil {
		return err
	}
	rec, err := store.Resolve(idOrName)
	if err != nil {
		return err
	}
	rec = store.refresh(rec)
	if rec.Status != StatusRunning || rec.Pid <= 0 {
		return fmt.Errorf("container %s is not running", idOrName)
	}
	if timeoutSeconds <= 0 {
		timeoutSeconds = 10
	}
	_ = syscall.Kill(rec.Pid, syscall.SIGTERM)
	deadline := time.Now().Add(time.Duration(timeoutSeconds) * time.Second)
	for time.Now().Before(deadline) {
		if !pidAlive(rec.Pid) {
			break
		}
		time.Sleep(100 * time.Millisecond)
	}
	if pidAlive(rec.Pid) {
		_ = syscall.Kill(rec.Pid, syscall.SIGKILL)
		time.Sleep(100 * time.Millisecond)
	}
	rec.Status = StatusStopped
	rec.FinishedAt = time.Now().UTC()
	store.teardownNetwork(&rec)
	_ = store.Save(rec)
	return nil
}

// Kill sends a signal (default SIGKILL) to the container init.
func Kill(idOrName string, sig syscall.Signal) error {
	if sig == 0 {
		sig = syscall.SIGKILL
	}
	store, err := OpenStore("")
	if err != nil {
		return err
	}
	rec, err := store.Resolve(idOrName)
	if err != nil {
		return err
	}
	if rec.Pid <= 0 {
		return fmt.Errorf("container %s has no pid", idOrName)
	}
	if err := syscall.Kill(rec.Pid, sig); err != nil {
		return err
	}
	if sig == syscall.SIGKILL {
		rec.Status = StatusStopped
		rec.FinishedAt = time.Now().UTC()
		_ = store.Save(rec)
	}
	return nil
}

// Remove deletes a container. force kills if running.
func Remove(idOrName string, force bool) error {
	store, err := OpenStore("")
	if err != nil {
		return err
	}
	rec, err := store.Resolve(idOrName)
	if err != nil {
		return err
	}
	rec = store.refresh(rec)
	if rec.Status == StatusRunning && rec.Pid > 0 {
		if !force {
			return fmt.Errorf("container %s is running; use -f to force", idOrName)
		}
		_ = syscall.Kill(rec.Pid, syscall.SIGKILL)
		time.Sleep(150 * time.Millisecond)
	}
	rec.Status = StatusRemoving
	_ = store.Save(rec)

	_ = network.Teardown(store.Layout.Root, network.Endpoint{
		ContainerID: rec.ID,
		Mode:        rec.NetworkMode,
		IP:          rec.IP,
		Gateway:     rec.Gateway,
		HostVeth:    rec.HostVeth,
		Ports:       rec.Ports,
	})

	// Unmount overlay if present.
	merged := filepath.Join(rec.BundleDir, "rootfs", "merged")
	if rec.Rootfs != "" {
		_ = rootfs.UnmountPath(rec.Rootfs)
	}
	_ = rootfs.UnmountPath(merged)

	if rec.CgroupPath != "" {
		cg := &cgroups.Manager{Path: rec.CgroupPath}
		_ = cg.Destroy()
	}
	return store.Delete(rec.ID)
}

// InspectJSON returns the container record as JSON.
func InspectJSON(idOrName string) ([]byte, error) {
	store, err := OpenStore("")
	if err != nil {
		return nil, err
	}
	rec, err := store.Resolve(idOrName)
	if err != nil {
		return nil, err
	}
	rec = store.refresh(rec)
	return json.MarshalIndent(rec, "", "  ")
}

// ExecOptions configures petti exec.
type ExecOptions struct {
	IDOrName string
	Args     []string
	Tty      bool
	Workdir  string
	Env      []string
	Stdin    io.Reader
	Stdout   io.Writer
	Stderr   io.Writer
}

// Exec joins the container namespaces and runs a command.
func Exec(opts ExecOptions) error {
	store, err := OpenStore("")
	if err != nil {
		return err
	}
	rec, err := store.Resolve(opts.IDOrName)
	if err != nil {
		return err
	}
	rec = store.refresh(rec)
	if rec.Status != StatusRunning || rec.Pid <= 0 || !pidAlive(rec.Pid) {
		return fmt.Errorf("container %s is not running", opts.IDOrName)
	}
	if len(opts.Args) == 0 {
		return fmt.Errorf("command required")
	}
	self, err := os.Executable()
	if err != nil {
		return err
	}
	if _, err := os.Stat("/proc/self/exe"); err == nil {
		self = "/proc/self/exe"
	}
	wd := opts.Workdir
	if wd == "" {
		wd = rec.Workdir
	}
	args := []string{"__exec", "--pid", fmt.Sprintf("%d", rec.Pid), "--workdir", wd, "--"}
	args = append(args, opts.Args...)
	cmd := exec.Command(self, args...)
	cmd.Stdin = opts.Stdin
	cmd.Stdout = opts.Stdout
	cmd.Stderr = opts.Stderr
	if cmd.Stdin == nil {
		cmd.Stdin = os.Stdin
	}
	if cmd.Stdout == nil {
		cmd.Stdout = os.Stdout
	}
	if cmd.Stderr == nil {
		cmd.Stderr = os.Stderr
	}
	return cmd.Run()
}

// ListFilters for ps.
type ListFilters struct {
	All bool
}

// ListRecords returns containers for ps.
func ListRecords(f ListFilters) ([]Record, error) {
	store, err := OpenStore("")
	if err != nil {
		return nil, err
	}
	all, err := store.List()
	if err != nil {
		return nil, err
	}
	if f.All {
		return all, nil
	}
	var out []Record
	for _, r := range all {
		if r.Status == StatusRunning {
			out = append(out, r)
		}
	}
	return out, nil
}
