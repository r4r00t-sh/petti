package compose

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadCompose(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "petti.yaml")
	content := `
services:
  web:
    image: alpine
    command: ["sleep", "10"]
    ports:
      - "8080:80"
    environment:
      FOO: bar
  worker:
    image: alpine
    command: ["echo", "hi"]
`
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	f, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(f.Services) != 2 {
		t.Fatalf("%+v", f.Services)
	}
	if f.Services["web"].Image != "alpine" || len(f.Services["web"].Ports) != 1 {
		t.Fatalf("%+v", f.Services["web"])
	}
	if ProjectName(path) == "" {
		t.Fatal("empty project")
	}
}
