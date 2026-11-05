package rootless

import (
	"os"
	"path/filepath"
	"testing"
)

func TestFormatProcMap(t *testing.T) {
	s := FormatProcMap([]IDMap{{0, 1000, 1}, {1, 100000, 65536}})
	want := "0 1000 1\n1 100000 65536\n"
	if s != want {
		t.Fatalf("got %q want %q", s, want)
	}
}

func TestLookupSubID(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "subuid")
	content := "# comment\n" +
		"alice:200000:65536\n" +
		"1000:100000:65536\n"
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	r, err := lookupSubID(path, "bob", 1000)
	if err != nil {
		t.Fatal(err)
	}
	if r.Start != 100000 || r.Count != 65536 {
		t.Fatalf("%+v", r)
	}
	r, err = lookupSubID(path, "alice", 1)
	if err != nil {
		t.Fatal(err)
	}
	if r.Start != 200000 {
		t.Fatalf("%+v", r)
	}
}

func TestEnabled(t *testing.T) {
	if !Enabled(true) {
		t.Fatal("force should enable")
	}
}
