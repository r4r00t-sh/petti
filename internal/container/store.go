package container

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"syscall"

	"github.com/r4r00t-sh/petti/internal/network"
	"github.com/r4r00t-sh/petti/internal/state"
)

// Store persists container records under <root>/containers/<id>/.
type Store struct {
	Layout *state.Layout
}

// OpenStore opens the container metadata store.
func OpenStore(root string) (*Store, error) {
	l, err := state.NewLayout(root)
	if err != nil {
		return nil, err
	}
	if err := os.MkdirAll(filepath.Join(l.Root, "containers"), 0o755); err != nil {
		return nil, err
	}
	return &Store{Layout: l}, nil
}

func (s *Store) dir(id string) string {
	return filepath.Join(s.Layout.Root, "containers", id)
}

func (s *Store) statePath(id string) string {
	return filepath.Join(s.dir(id), "state.json")
}

func (s *Store) nameIndexPath() string {
	return filepath.Join(s.Layout.Root, "containers", "names.json")
}

// BundleDir returns containers/<id>/.
func (s *Store) BundleDir(id string) string {
	return s.dir(id)
}

// Save writes state.json atomically.
func (s *Store) Save(rec Record) error {
	if err := os.MkdirAll(s.dir(rec.ID), 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(rec, "", "  ")
	if err != nil {
		return err
	}
	path := s.statePath(rec.ID)
	return state.WithLock(path+".lock", func() error {
		return state.WriteFileAtomic(path, data, 0o644)
	})
}

// Load reads a container record by id.
func (s *Store) Load(id string) (Record, error) {
	data, err := os.ReadFile(s.statePath(id))
	if err != nil {
		return Record{}, fmt.Errorf("container %s: %w", id, err)
	}
	var rec Record
	if err := json.Unmarshal(data, &rec); err != nil {
		return Record{}, err
	}
	return rec, nil
}

// Delete removes the container directory (after unmount/cleanup).
func (s *Store) Delete(id string) error {
	rec, _ := s.Load(id)
	if rec.Name != "" {
		_ = s.unbindName(rec.Name)
	}
	return os.RemoveAll(s.dir(id))
}

// List returns all container records (refreshes running→exited if pid dead).
func (s *Store) List() ([]Record, error) {
	entries, err := os.ReadDir(filepath.Join(s.Layout.Root, "containers"))
	if err != nil {
		return nil, err
	}
	var out []Record
	for _, e := range entries {
		if !e.IsDir() || e.Name() == "." {
			continue
		}
		rec, err := s.Load(e.Name())
		if err != nil {
			continue
		}
		rec = s.refresh(rec)
		out = append(out, rec)
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i].CreatedAt.After(out[j].CreatedAt)
	})
	return out, nil
}

// Resolve finds a container by id prefix or name.
func (s *Store) Resolve(idOrName string) (Record, error) {
	if rec, err := s.Load(idOrName); err == nil {
		return s.refresh(rec), nil
	}
	// Name index
	if id, err := s.lookupName(idOrName); err == nil {
		rec, err := s.Load(id)
		if err != nil {
			return Record{}, err
		}
		return s.refresh(rec), nil
	}
	// Prefix match
	all, err := s.List()
	if err != nil {
		return Record{}, err
	}
	var matches []Record
	for _, r := range all {
		if strings.HasPrefix(r.ID, idOrName) {
			matches = append(matches, r)
		}
	}
	if len(matches) == 1 {
		return matches[0], nil
	}
	if len(matches) > 1 {
		return Record{}, fmt.Errorf("ambiguous container id prefix %q", idOrName)
	}
	return Record{}, fmt.Errorf("no such container: %s", idOrName)
}

func (s *Store) refresh(rec Record) Record {
	if rec.Status == StatusRunning && rec.Pid > 0 {
		if !pidAlive(rec.Pid) {
			rec.Status = StatusExited
			s.teardownNetwork(&rec)
			_ = s.Save(rec)
		}
	}
	return rec
}

func (s *Store) teardownNetwork(rec *Record) {
	if rec == nil || rec.NetworkMode == "" || rec.NetworkMode == network.ModeHost {
		return
	}
	_ = network.Teardown(s.Layout.Root, network.Endpoint{
		ContainerID: rec.ID,
		Mode:        rec.NetworkMode,
		IP:          rec.IP,
		Gateway:     rec.Gateway,
		HostVeth:    rec.HostVeth,
		Ports:       rec.Ports,
	})
	rec.HostVeth = ""
}

func pidAlive(pid int) bool {
	err := syscall.Kill(pid, 0)
	return err == nil
}

func (s *Store) bindName(name, id string) error {
	if name == "" {
		return nil
	}
	return state.WithLock(s.nameIndexPath()+".lock", func() error {
		idx := map[string]string{}
		if data, err := os.ReadFile(s.nameIndexPath()); err == nil {
			_ = json.Unmarshal(data, &idx)
		}
		if existing, ok := idx[name]; ok && existing != id {
			return fmt.Errorf("name %q already in use by %s", name, existing)
		}
		idx[name] = id
		data, _ := json.MarshalIndent(idx, "", "  ")
		return state.WriteFileAtomic(s.nameIndexPath(), data, 0o644)
	})
}

func (s *Store) unbindName(name string) error {
	return state.WithLock(s.nameIndexPath()+".lock", func() error {
		idx := map[string]string{}
		data, err := os.ReadFile(s.nameIndexPath())
		if err != nil {
			return nil
		}
		_ = json.Unmarshal(data, &idx)
		delete(idx, name)
		out, _ := json.MarshalIndent(idx, "", "  ")
		return state.WriteFileAtomic(s.nameIndexPath(), out, 0o644)
	})
}

func (s *Store) lookupName(name string) (string, error) {
	data, err := os.ReadFile(s.nameIndexPath())
	if err != nil {
		return "", err
	}
	idx := map[string]string{}
	if err := json.Unmarshal(data, &idx); err != nil {
		return "", err
	}
	id, ok := idx[name]
	if !ok {
		return "", fmt.Errorf("not found")
	}
	return id, nil
}
