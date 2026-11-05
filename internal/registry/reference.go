// Package registry implements an OCI Distribution / Docker Registry HTTP API client.
package registry

import (
	"fmt"
	"strings"
)

const (
	defaultRegistry = "docker.io"
	defaultTag      = "latest"
	officialLibrary = "library"
)

// Reference is a parsed image name.
type Reference struct {
	Domain string // registry host, e.g. docker.io, ghcr.io
	Path   string // repository path, e.g. library/alpine
	Tag    string // tag, empty if digest-only
	Digest string // sha256:… if present
}

func (r Reference) String() string {
	name := r.Domain + "/" + r.Path
	if r.Digest != "" {
		return name + "@" + r.Digest
	}
	tag := r.Tag
	if tag == "" {
		tag = defaultTag
	}
	return name + ":" + tag
}

// Repository returns domain/path for auth scopes (docker.io → no domain in scope path).
func (r Reference) Repository() string {
	return r.Path
}

// RegistryHost returns the registry API host.
func (r Reference) RegistryHost() string {
	if r.Domain == "docker.io" {
		return "registry-1.docker.io"
	}
	return r.Domain
}

// AuthRealmHost hints where to fetch tokens for known registries.
func (r Reference) AuthService() string {
	if r.Domain == "docker.io" {
		return "registry.docker.io"
	}
	return r.Domain
}

// ParseReference parses familiar image names into a Reference.
//
//	alpine
//	alpine:3.20
//	library/alpine:3.20
//	docker.io/library/alpine:3.20
//	ghcr.io/org/app:tag
//	alpine@sha256:deadbeef…
func ParseReference(raw string) (Reference, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return Reference{}, fmt.Errorf("empty image reference")
	}

	var digest string
	if i := strings.Index(raw, "@"); i >= 0 {
		digest = raw[i+1:]
		raw = raw[:i]
		if !strings.HasPrefix(digest, "sha256:") || len(digest) != len("sha256:")+64 {
			return Reference{}, fmt.Errorf("invalid digest %q", digest)
		}
	}

	tag := defaultTag
	// Split tag only on the last path component (avoid port confusion — we don't support IP:port registries without scheme parsing complexity; host:port/path is rare in educational scope).
	if digest == "" {
		if i := strings.LastIndex(raw, ":"); i >= 0 {
			// If there's a slash after the colon, it's not a tag (e.g. host:5000/repo).
			if !strings.Contains(raw[i+1:], "/") {
				tag = raw[i+1:]
				raw = raw[:i]
				if tag == "" {
					return Reference{}, fmt.Errorf("empty tag in %q", raw)
				}
			}
		}
	} else {
		tag = ""
	}

	domain := defaultRegistry
	var path string
	parts := strings.Split(raw, "/")
	switch {
	case len(parts) == 1:
		// alpine → docker.io/library/alpine
		path = officialLibrary + "/" + parts[0]
	case len(parts) >= 2 && (strings.Contains(parts[0], ".") || strings.Contains(parts[0], ":") || parts[0] == "localhost"):
		domain = parts[0]
		path = strings.Join(parts[1:], "/")
		if path == "" {
			return Reference{}, fmt.Errorf("invalid reference %q", raw)
		}
	default:
		// org/name → docker.io/org/name
		path = strings.Join(parts, "/")
	}

	if domain == "docker.io" && !strings.Contains(path, "/") {
		path = officialLibrary + "/" + path
	}

	return Reference{Domain: domain, Path: path, Tag: tag, Digest: digest}, nil
}
