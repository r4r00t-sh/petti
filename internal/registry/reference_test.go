package registry_test

import (
	"testing"

	"github.com/r4r00t-sh/petti/internal/registry"
)

func TestParseReference(t *testing.T) {
	cases := []struct {
		in, domain, path, tag, digest string
	}{
		{"alpine", "docker.io", "library/alpine", "latest", ""},
		{"alpine:3.20", "docker.io", "library/alpine", "3.20", ""},
		{"library/alpine:3.20", "docker.io", "library/alpine", "3.20", ""},
		{"ghcr.io/org/app:tag", "ghcr.io", "org/app", "tag", ""},
		{"alpine@sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", "docker.io", "library/alpine", "", "sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"},
	}
	for _, tc := range cases {
		ref, err := registry.ParseReference(tc.in)
		if err != nil {
			t.Fatalf("%s: %v", tc.in, err)
		}
		if ref.Domain != tc.domain || ref.Path != tc.path || ref.Tag != tc.tag || ref.Digest != tc.digest {
			t.Fatalf("%s: got %+v", tc.in, ref)
		}
	}
}

func TestParseReferenceErrors(t *testing.T) {
	for _, in := range []string{"", "alpine@", "alpine@sha256:abc"} {
		if _, err := registry.ParseReference(in); err == nil {
			t.Fatalf("expected error for %q", in)
		}
	}
}

func FuzzParseReference(f *testing.F) {
	for _, s := range []string{"alpine", "a:b", "ghcr.io/x/y:z", "x@sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"} {
		f.Add(s)
	}
	f.Fuzz(func(t *testing.T, s string) {
		_, _ = registry.ParseReference(s)
	})
}
