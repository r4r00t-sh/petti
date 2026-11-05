package archive

import (
	"path/filepath"
	"testing"

	"github.com/r4r00t-sh/petti/internal/image"
	"github.com/r4r00t-sh/petti/internal/registry"
	"github.com/r4r00t-sh/petti/internal/state"
)

func TestSaveLoadRoundTrip(t *testing.T) {
	root := t.TempDir()
	store, err := image.OpenStore(root)
	if err != nil {
		t.Fatal(err)
	}
	cfg := registry.ImageConfig{}
	cfg.Config.Cmd = []string{"true"}
	cfg.RootFS.Type = "layers"
	cfgHex := "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"
	if err := state.WriteFileAtomic(store.Layout.BlobPath(cfgHex), []byte(`{}`), 0o644); err != nil {
		t.Fatal(err)
	}
	rec := image.Record{
		ID:           "sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
		Repo:         "localhost/demo",
		Tag:          "v1",
		ConfigDigest: "sha256:" + cfgHex,
		Config:       cfg,
	}
	if err := store.SaveRecord(rec); err != nil {
		t.Fatal(err)
	}

	dest := filepath.Join(t.TempDir(), "img.tar.gz")
	if err := Save(store, []string{"localhost/demo:v1"}, dest); err != nil {
		t.Fatal(err)
	}

	root2 := t.TempDir()
	store2, err := image.OpenStore(root2)
	if err != nil {
		t.Fatal(err)
	}
	loaded, err := Load(store2, dest)
	if err != nil {
		t.Fatal(err)
	}
	if len(loaded) != 1 || loaded[0].Repo != "localhost/demo" {
		t.Fatalf("%+v", loaded)
	}
	if _, err := store2.Resolve("localhost/demo:v1"); err != nil {
		t.Fatal(err)
	}
}
