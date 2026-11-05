// Package image manages the local image store, pull/unpack, and image config.
package image

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/r4r00t-sh/petti/internal/registry"
	"github.com/r4r00t-sh/petti/internal/state"
)

// Record is the on-disk metadata for a pulled image tag.
type Record struct {
	ID             string               `json:"id"` // config digest
	Repo           string               `json:"repo"`
	Tag            string               `json:"tag"`
	ManifestDigest string               `json:"manifestDigest"`
	ConfigDigest   string               `json:"configDigest"`
	Created        string               `json:"created"`
	Size           int64                `json:"size"`
	Layers         []LayerMeta          `json:"layers"`
	Config         registry.ImageConfig `json:"config"`
	PulledAt       time.Time            `json:"pulledAt"`
}

// LayerMeta describes one rootfs layer.
type LayerMeta struct {
	Digest string `json:"digest"` // compressed blob digest
	DiffID string `json:"diffID"` // uncompressed tar digest
	Size   int64  `json:"size"`
}

// Store is a local image store under a state.Layout.
type Store struct {
	Layout *state.Layout
}

// OpenStore creates/opens the image store at root (empty → DataRoot).
func OpenStore(root string) (*Store, error) {
	l, err := state.NewLayout(root)
	if err != nil {
		return nil, err
	}
	return &Store{Layout: l}, nil
}

func (s *Store) recordPath(repo, tag string) string {
	safe := strings.ReplaceAll(repo, "/", "_")
	safe = strings.ReplaceAll(safe, ":", "_")
	return filepath.Join(s.Layout.Images, safe+"_"+tag+".json")
}

// SaveRecord atomically writes an image record.
func (s *Store) SaveRecord(rec Record) error {
	data, err := json.MarshalIndent(rec, "", "  ")
	if err != nil {
		return err
	}
	path := s.recordPath(rec.Repo, rec.Tag)
	lock := path + ".lock"
	return state.WithLock(lock, func() error {
		return state.WriteFileAtomic(path, data, 0o644)
	})
}

// Get returns a record by repo and tag.
func (s *Store) Get(repo, tag string) (Record, error) {
	if tag == "" {
		tag = "latest"
	}
	path := s.recordPath(repo, tag)
	data, err := os.ReadFile(path)
	if err != nil {
		return Record{}, fmt.Errorf("image %s:%s not found locally (try: petti pull)", repo, tag)
	}
	var rec Record
	if err := json.Unmarshal(data, &rec); err != nil {
		return Record{}, err
	}
	return rec, nil
}

// Resolve looks up a familiar name (alpine, alpine:3.20, …).
func (s *Store) Resolve(name string) (Record, error) {
	ref, err := registry.ParseReference(name)
	if err != nil {
		return Record{}, err
	}
	tag := ref.Tag
	if tag == "" {
		tag = "latest"
	}
	repo := ref.Domain + "/" + ref.Path
	return s.Get(repo, tag)
}

// List returns all local image records sorted by repo/tag.
func (s *Store) List() ([]Record, error) {
	entries, err := os.ReadDir(s.Layout.Images)
	if err != nil {
		return nil, err
	}
	var out []Record
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".json") {
			continue
		}
		data, err := os.ReadFile(filepath.Join(s.Layout.Images, e.Name()))
		if err != nil {
			continue
		}
		var rec Record
		if err := json.Unmarshal(data, &rec); err != nil {
			continue
		}
		out = append(out, rec)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Repo == out[j].Repo {
			return out[i].Tag < out[j].Tag
		}
		return out[i].Repo < out[j].Repo
	})
	return out, nil
}

// Remove deletes an image record if present. Blobs/layers are left for GC (prune later).
func (s *Store) Remove(name string) error {
	ref, err := registry.ParseReference(name)
	if err != nil {
		return err
	}
	tag := ref.Tag
	if tag == "" {
		tag = "latest"
	}
	repo := ref.Domain + "/" + ref.Path
	path := s.recordPath(repo, tag)
	if err := os.Remove(path); err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("no such image: %s:%s", repo, tag)
		}
		return err
	}
	return nil
}

// PrepareRootfs merges unpacked layers into dest for running a container (pre-overlay).
func (s *Store) PrepareRootfs(rec Record, dest string) error {
	var dirs []string
	for _, layer := range rec.Layers {
		hex := strings.TrimPrefix(layer.DiffID, "sha256:")
		dir := s.Layout.LayerPath(hex)
		if st, err := os.Stat(dir); err != nil || !st.IsDir() {
			return fmt.Errorf("missing unpacked layer %s (re-pull the image)", layer.DiffID)
		}
		dirs = append(dirs, dir)
	}
	return MergeLayers(dirs, dest)
}
