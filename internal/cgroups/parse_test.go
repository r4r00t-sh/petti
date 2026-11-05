package cgroups_test

import (
	"testing"

	"github.com/r4r00t-sh/petti/internal/cgroups"
)

func TestParseBytes(t *testing.T) {
	cases := []struct {
		in   string
		want int64
	}{
		{"128m", 128 * 1024 * 1024},
		{"1G", 1024 * 1024 * 1024},
		{"512k", 512 * 1024},
		{"100", 100},
		{"2T", 2 * 1024 * 1024 * 1024 * 1024},
	}
	for _, tc := range cases {
		got, err := cgroups.ParseBytes(tc.in)
		if err != nil || got != tc.want {
			t.Fatalf("%q: got %d %v, want %d", tc.in, got, err, tc.want)
		}
	}
	if _, err := cgroups.ParseBytes("nope"); err == nil {
		t.Fatal("expected error")
	}
}

func TestParseCPUs(t *testing.T) {
	v, err := cgroups.ParseCPUs("0.5")
	if err != nil || v != 0.5 {
		t.Fatalf("%v %v", v, err)
	}
	if _, err := cgroups.ParseCPUs("-1"); err == nil {
		t.Fatal("expected error")
	}
}

func TestParsePids(t *testing.T) {
	v, err := cgroups.ParsePids("64")
	if err != nil || v != 64 {
		t.Fatalf("%v %v", v, err)
	}
}

func TestCPUMax(t *testing.T) {
	if got := cgroups.CPUMax(0.5); got != "50000 100000" {
		t.Fatalf("got %q", got)
	}
	if got := cgroups.CPUMax(2); got != "200000 100000" {
		t.Fatalf("got %q", got)
	}
}

func TestLimitsSet(t *testing.T) {
	if (cgroups.Limits{}).Set() {
		t.Fatal("empty should be unset")
	}
	if !(cgroups.Limits{Pids: 1}).Set() {
		t.Fatal("pids should count")
	}
}
