package image_test

import (
	"archive/tar"
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/r4r00t-sh/petti/internal/image"
)

func writeTar(t *testing.T, files map[string]string) *bytes.Buffer {
	t.Helper()
	var buf bytes.Buffer
	tw := tar.NewWriter(&buf)
	for name, body := range files {
		hdr := &tar.Header{Name: name, Mode: 0o644, Size: int64(len(body))}
		if err := tw.WriteHeader(hdr); err != nil {
			t.Fatal(err)
		}
		if _, err := tw.Write([]byte(body)); err != nil {
			t.Fatal(err)
		}
	}
	if err := tw.Close(); err != nil {
		t.Fatal(err)
	}
	return &buf
}

func TestApplyLayerTarPathTraversal(t *testing.T) {
	dest := t.TempDir()
	cases := []string{
		"../escape",
		"/etc/passwd",
		"foo/../../etc/passwd",
	}
	for _, name := range cases {
		buf := writeTar(t, map[string]string{name: "x"})
		err := image.ApplyLayerTar(bytes.NewReader(buf.Bytes()), dest)
		if err == nil {
			t.Fatalf("expected reject for %q", name)
		}
	}
}

func TestApplyLayerTarSymlinkEscape(t *testing.T) {
	dest := t.TempDir()
	var buf bytes.Buffer
	tw := tar.NewWriter(&buf)
	hdr := &tar.Header{Name: "link", Mode: 0o777, Typeflag: tar.TypeSymlink, Linkname: "../../outside"}
	if err := tw.WriteHeader(hdr); err != nil {
		t.Fatal(err)
	}
	_ = tw.Close()
	if err := image.ApplyLayerTar(bytes.NewReader(buf.Bytes()), dest); err == nil {
		t.Fatal("expected symlink escape rejection")
	}
}

func TestMergeWhiteouts(t *testing.T) {
	l1 := t.TempDir()
	l2 := t.TempDir()
	if err := os.WriteFile(filepath.Join(l1, "keep"), []byte("1"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(l1, "gone"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(l2, ".wh.gone"), []byte{}, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(l2, "new"), []byte("2"), 0o644); err != nil {
		t.Fatal(err)
	}

	dest := t.TempDir()
	if err := image.MergeLayers([]string{l1, l2}, dest); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(dest, "gone")); !os.IsNotExist(err) {
		t.Fatal("whiteout did not remove gone")
	}
	b, err := os.ReadFile(filepath.Join(dest, "new"))
	if err != nil || string(b) != "2" {
		t.Fatalf("new file: %v %s", err, b)
	}
}

func TestMergeOpaqueWhiteout(t *testing.T) {
	l1 := t.TempDir()
	l2 := t.TempDir()
	dir1 := filepath.Join(l1, "d")
	if err := os.MkdirAll(dir1, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir1, "a"), []byte("a"), 0o644); err != nil {
		t.Fatal(err)
	}
	dir2 := filepath.Join(l2, "d")
	if err := os.MkdirAll(dir2, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir2, ".wh..wh..opq"), []byte{}, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir2, "b"), []byte("b"), 0o644); err != nil {
		t.Fatal(err)
	}
	dest := t.TempDir()
	if err := image.MergeLayers([]string{l1, l2}, dest); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(dest, "d", "a")); !os.IsNotExist(err) {
		t.Fatal("opaque whiteout should clear lower file a")
	}
	b, err := os.ReadFile(filepath.Join(dest, "d", "b"))
	if err != nil || string(b) != "b" {
		t.Fatalf("b: %v %s", err, b)
	}
}

func FuzzSafeJoinViaTar(f *testing.F) {
	f.Add("../x")
	f.Add("/etc/passwd")
	f.Add("ok.txt")
	f.Fuzz(func(t *testing.T, name string) {
		dest := t.TempDir()
		buf := writeTar(t, map[string]string{name: "x"})
		_ = image.ApplyLayerTar(bytes.NewReader(buf.Bytes()), dest)
	})
}
