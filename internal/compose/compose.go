package compose

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/r4r00t-sh/petti/internal/container"
	"github.com/r4r00t-sh/petti/internal/network"
)

// File is the petti.yaml schema (compose-lite).
type File struct {
	Services map[string]Service `yaml:"services"`
}

// Service is one container definition.
type Service struct {
	Image       string            `yaml:"image"`
	Command     []string          `yaml:"command"`
	Entrypoint  []string          `yaml:"entrypoint"`
	Environment map[string]string `yaml:"environment"`
	EnvList     []string          `yaml:"env"` // KEY=VAL form
	Ports       []string          `yaml:"ports"`
	Volumes     []string          `yaml:"volumes"`
	Network     string            `yaml:"network"`
	Hostname    string            `yaml:"hostname"`
	Restart     string            `yaml:"restart"` // ignored (documented)
	ReadOnly    bool              `yaml:"read_only"`
	CapAdd      []string          `yaml:"cap_add"`
	CapDrop     []string          `yaml:"cap_drop"`
}

// ProjectName derives a name prefix from the compose file directory.
func ProjectName(composePath string) string {
	dir := filepath.Base(filepath.Dir(composePath))
	if dir == "" || dir == "." || dir == "/" {
		dir = "petti"
	}
	return sanitize(dir)
}

func sanitize(s string) string {
	s = strings.ToLower(s)
	var b strings.Builder
	for _, r := range s {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '-' || r == '_' {
			b.WriteRune(r)
		} else {
			b.WriteByte('-')
		}
	}
	out := b.String()
	if out == "" {
		return "petti"
	}
	return out
}

// Load reads petti.yaml from path.
func Load(path string) (*File, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var f File
	if err := yaml.Unmarshal(data, &f); err != nil {
		return nil, fmt.Errorf("parse %s: %w", path, err)
	}
	if len(f.Services) == 0 {
		return nil, fmt.Errorf("%s: no services", path)
	}
	for name, svc := range f.Services {
		if svc.Image == "" {
			return nil, fmt.Errorf("service %q: image is required", name)
		}
	}
	return &f, nil
}

// UpOptions configures compose up.
type UpOptions struct {
	File        string
	Detach      bool
	ProjectName string
}

// Up starts all services (detached by default for multi-container).
func Up(opts UpOptions) ([]container.RunResult, error) {
	path := opts.File
	if path == "" {
		path = "petti.yaml"
	}
	abs, err := filepath.Abs(path)
	if err != nil {
		return nil, err
	}
	f, err := Load(abs)
	if err != nil {
		return nil, err
	}
	proj := opts.ProjectName
	if proj == "" {
		proj = ProjectName(abs)
	}
	detach := opts.Detach
	if !detach {
		detach = true // compose-lite always backgrounds services
	}

	var results []container.RunResult
	for name, svc := range f.Services {
		cname := proj + "_" + name
		env := append([]string(nil), svc.EnvList...)
		for k, v := range svc.Environment {
			env = append(env, k+"="+v)
		}
		var ports []network.PortMapping
		for _, p := range svc.Ports {
			m, err := network.ParsePort(p)
			if err != nil {
				return results, fmt.Errorf("service %s ports: %w", name, err)
			}
			ports = append(ports, m)
		}
		var vols []container.Volume
		for _, v := range svc.Volumes {
			vol, err := container.ParseVolume(v)
			if err != nil {
				return results, fmt.Errorf("service %s volumes: %w", name, err)
			}
			vols = append(vols, vol)
		}
		netMode := svc.Network
		res, err := container.Run(container.RunOptions{
			Image:      svc.Image,
			Name:       cname,
			Hostname:   svc.Hostname,
			Env:        env,
			Entrypoint: svc.Entrypoint,
			Args:       svc.Command,
			Volumes:    vols,
			Ports:      ports,
			Network:    netMode,
			Detach:     detach,
			ReadOnly:   svc.ReadOnly,
			CapAdd:     svc.CapAdd,
			CapDrop:    svc.CapDrop,
		})
		if err != nil {
			return results, fmt.Errorf("service %s: %w", name, err)
		}
		results = append(results, res)
	}
	return results, nil
}

// Down stops and removes containers for the project.
func Down(composePath, project string) error {
	if composePath == "" {
		composePath = "petti.yaml"
	}
	abs, err := filepath.Abs(composePath)
	if err != nil {
		return err
	}
	if project == "" {
		project = ProjectName(abs)
	}
	f, err := Load(abs)
	if err != nil {
		return err
	}
	var first error
	for name := range f.Services {
		cname := project + "_" + name
		_ = container.Stop(cname, 5)
		if err := container.Remove(cname, true); err != nil {
			if first == nil && !strings.Contains(err.Error(), "no such container") {
				first = err
			}
		}
	}
	return first
}
