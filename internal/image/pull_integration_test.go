//go:build integration

package image_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/r4r00t-sh/petti/internal/image"
)

func TestPullAlpine(t *testing.T) {
	if testing.Short() {
		t.Skip("network pull")
	}
	root := t.TempDir()
	store, err := image.OpenStore(root)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	rec, err := store.Pull(ctx, image.PullOptions{Ref: "alpine", Stderr: os.Stderr})
	if err != nil {
		t.Fatal(err)
	}
	if rec.Repo != "docker.io/library/alpine" {
		t.Fatalf("repo=%s", rec.Repo)
	}
	dest := filepath.Join(root, "merged")
	if err := store.PrepareRootfs(rec, dest); err != nil {
		t.Fatal(err)
	}
	b, err := os.ReadFile(filepath.Join(dest, "etc", "os-release"))
	if err != nil {
		t.Fatal(err)
	}
	if len(b) == 0 {
		t.Fatal("empty os-release")
	}
}
