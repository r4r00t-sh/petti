package container_test

import (
	"testing"

	"github.com/r4r00t-sh/petti/internal/container"
)

func TestParseVolume(t *testing.T) {
	v, err := container.ParseVolume("/tmp/data:/data")
	if err != nil || v.Source != "/tmp/data" || v.Target != "/data" || v.ReadOnly {
		t.Fatalf("%+v %v", v, err)
	}
	v, err = container.ParseVolume("/tmp/data:/data:ro")
	if err != nil || !v.ReadOnly {
		t.Fatalf("%+v %v", v, err)
	}
	if _, err := container.ParseVolume("bad"); err == nil {
		t.Fatal("expected error")
	}
	if _, err := container.ParseVolume("/tmp:rel"); err == nil {
		t.Fatal("expected absolute dest")
	}
}
