package registry

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"runtime"
	"strings"
	"time"
)

// Client talks to an OCI registry anonymously (Bearer token).
type Client struct {
	HTTP  *http.Client
	cache *tokenCache
}

// NewClient returns a registry client with sensible timeouts.
func NewClient() *Client {
	return &Client{
		HTTP: &http.Client{
			Timeout: 5 * time.Minute,
			Transport: &http.Transport{
				Proxy: http.ProxyFromEnvironment,
			},
		},
		cache: newTokenCache(),
	}
}

// HostPlatform returns the platform string for this binary (linux/amd64, …).
func HostPlatform() Platform {
	return Platform{OS: runtime.GOOS, Architecture: runtime.GOARCH}
}

// ParsePlatform parses "os/arch" or "os/arch/variant".
func ParsePlatform(s string) (Platform, error) {
	parts := strings.Split(s, "/")
	if len(parts) < 2 || parts[0] == "" || parts[1] == "" {
		return Platform{}, fmt.Errorf("invalid platform %q (want os/arch)", s)
	}
	p := Platform{OS: parts[0], Architecture: parts[1]}
	if len(parts) > 2 {
		p.Variant = parts[2]
	}
	return p, nil
}

func (c *Client) do(ctx context.Context, method, rawURL, repo, token string, accept []string) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, method, rawURL, nil)
	if err != nil {
		return nil, err
	}
	for _, a := range accept {
		req.Header.Add("Accept", a)
	}
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	resp, err := c.HTTP.Do(req)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusUnauthorized || token != "" {
		return resp, nil
	}
	_ = resp.Body.Close()
	ch, err := parseWWWAuthenticate(resp.Header.Get("WWW-Authenticate"))
	if err != nil {
		return nil, err
	}
	if ch.service == "" {
		ch.service = ""
	}
	tok, err := c.fetchToken(ctx, ch, repo)
	if err != nil {
		return nil, err
	}
	c.cache.mu.Lock()
	c.cache.tokens[repo] = tok
	c.cache.mu.Unlock()
	return c.do(ctx, method, rawURL, repo, tok, accept)
}

func (c *Client) tokenFor(repo string) string {
	c.cache.mu.Lock()
	defer c.cache.mu.Unlock()
	return c.cache.tokens[repo]
}

// FetchManifest resolves a reference to a platform-specific image manifest and its digest.
func (c *Client) FetchManifest(ctx context.Context, ref Reference, platform Platform) (Manifest, string, error) {
	host := ref.RegistryHost()
	repo := ref.Repository()
	name := ref.Tag
	if ref.Digest != "" {
		name = ref.Digest
	}
	if name == "" {
		name = defaultTag
	}
	u := fmt.Sprintf("https://%s/v2/%s/manifests/%s", host, repo, name)
	accept := []string{
		MediaIndexOCI,
		MediaManifestListDocker,
		MediaManifestOCI,
		MediaManifestDocker,
	}
	resp, err := c.do(ctx, http.MethodGet, u, repo, c.tokenFor(repo), accept)
	if err != nil {
		return Manifest{}, "", err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 32<<20))
	if err != nil {
		return Manifest{}, "", err
	}
	if resp.StatusCode != http.StatusOK {
		return Manifest{}, "", fmt.Errorf("manifest %s: %s: %s", ref.String(), resp.Status, truncate(body, 200))
	}
	mt := resp.Header.Get("Content-Type")
	digest := resp.Header.Get("Docker-Content-Digest")
	if digest == "" {
		sum := sha256.Sum256(body)
		digest = "sha256:" + hex.EncodeToString(sum[:])
	}

	switch {
	case strings.Contains(mt, "manifest.list") || strings.Contains(mt, "image.index") || isIndexJSON(body):
		var idx Index
		if err := json.Unmarshal(body, &idx); err != nil {
			return Manifest{}, "", err
		}
		entry, err := selectPlatform(idx.Manifests, platform)
		if err != nil {
			return Manifest{}, "", err
		}
		return c.fetchManifestDigest(ctx, ref, entry.Digest)
	default:
		var man Manifest
		if err := json.Unmarshal(body, &man); err != nil {
			return Manifest{}, "", err
		}
		return man, digest, nil
	}
}

func isIndexJSON(body []byte) bool {
	var probe struct {
		Manifests []IndexEntry `json:"manifests"`
	}
	if err := json.Unmarshal(body, &probe); err != nil {
		return false
	}
	return len(probe.Manifests) > 0
}

func selectPlatform(entries []IndexEntry, want Platform) (IndexEntry, error) {
	for _, e := range entries {
		if e.Platform.OS == "unknown" || e.Platform.Architecture == "unknown" {
			continue // attestation / junk
		}
		if e.Platform.OS == want.OS && e.Platform.Architecture == want.Architecture {
			if want.Variant == "" || e.Platform.Variant == want.Variant {
				return e, nil
			}
		}
	}
	return IndexEntry{}, fmt.Errorf("no manifest for platform %s/%s in index (%d entries)", want.OS, want.Architecture, len(entries))
}

func (c *Client) fetchManifestDigest(ctx context.Context, ref Reference, digest string) (Manifest, string, error) {
	host := ref.RegistryHost()
	repo := ref.Repository()
	u := fmt.Sprintf("https://%s/v2/%s/manifests/%s", host, repo, digest)
	accept := []string{MediaManifestOCI, MediaManifestDocker}
	resp, err := c.do(ctx, http.MethodGet, u, repo, c.tokenFor(repo), accept)
	if err != nil {
		return Manifest{}, "", err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 32<<20))
	if err != nil {
		return Manifest{}, "", err
	}
	if resp.StatusCode != http.StatusOK {
		return Manifest{}, "", fmt.Errorf("manifest %s: %s: %s", digest, resp.Status, truncate(body, 200))
	}
	var man Manifest
	if err := json.Unmarshal(body, &man); err != nil {
		return Manifest{}, "", err
	}
	got := resp.Header.Get("Docker-Content-Digest")
	if got == "" {
		sum := sha256.Sum256(body)
		got = "sha256:" + hex.EncodeToString(sum[:])
	}
	if got != digest {
		return Manifest{}, "", fmt.Errorf("manifest digest mismatch: got %s want %s", got, digest)
	}
	return man, digest, nil
}

// FetchBlob downloads a blob to destPath, verifying digest (sha256:hex).
// Supports resume via HTTP Range when a partial file exists.
func (c *Client) FetchBlob(ctx context.Context, ref Reference, digest string, destPath string, progress io.Writer) error {
	hexDig, err := stripAlgo(digest)
	if err != nil {
		return err
	}
	host := ref.RegistryHost()
	repo := ref.Repository()
	u := fmt.Sprintf("https://%s/v2/%s/blobs/%s", host, repo, digest)

	var offset int64
	if st, err := os.Stat(destPath); err == nil {
		offset = st.Size()
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return err
	}
	tok := c.tokenFor(repo)
	if tok != "" {
		req.Header.Set("Authorization", "Bearer "+tok)
	}
	if offset > 0 {
		req.Header.Set("Range", fmt.Sprintf("bytes=%d-", offset))
	}

	resp, err := c.HTTP.Do(req)
	if err != nil {
		return err
	}
	if resp.StatusCode == http.StatusUnauthorized {
		_ = resp.Body.Close()
		ch, err := parseWWWAuthenticate(resp.Header.Get("WWW-Authenticate"))
		if err != nil {
			return err
		}
		tok, err = c.fetchToken(ctx, ch, repo)
		if err != nil {
			return err
		}
		c.cache.mu.Lock()
		c.cache.tokens[repo] = tok
		c.cache.mu.Unlock()
		req, _ = http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
		req.Header.Set("Authorization", "Bearer "+tok)
		if offset > 0 {
			req.Header.Set("Range", fmt.Sprintf("bytes=%d-", offset))
		}
		resp, err = c.HTTP.Do(req)
		if err != nil {
			return err
		}
	}
	defer resp.Body.Close()

	switch resp.StatusCode {
	case http.StatusOK:
		offset = 0 // full body — restart
	case http.StatusPartialContent:
		// resume OK
	default:
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 200))
		return fmt.Errorf("blob %s: %s: %s", digest, resp.Status, body)
	}

	flags := os.O_CREATE | os.O_WRONLY
	if offset > 0 && resp.StatusCode == http.StatusPartialContent {
		flags |= os.O_APPEND
	} else {
		flags |= os.O_TRUNC
		offset = 0
	}
	f, err := os.OpenFile(destPath, flags, 0o644)
	if err != nil {
		return err
	}
	defer f.Close()

	h := sha256.New()
	if offset > 0 {
		// Re-hash existing prefix.
		rf, err := os.Open(destPath)
		if err != nil {
			return err
		}
		if _, err := io.CopyN(h, rf, offset); err != nil {
			_ = rf.Close()
			return err
		}
		_ = rf.Close()
		// File is open append; hash continues with new bytes.
	}

	pw := &progressWriter{w: f, h: h, total: resp.ContentLength + offset, written: offset, out: progress, digest: hexDig}
	if _, err := io.Copy(pw, resp.Body); err != nil {
		return err
	}
	pw.finish()
	sum := hex.EncodeToString(h.Sum(nil))
	if sum != hexDig {
		_ = os.Remove(destPath)
		return fmt.Errorf("digest mismatch for %s: got sha256:%s", digest, sum)
	}
	return nil
}

// FetchBlobBytes downloads a small blob into memory and verifies the digest.
func (c *Client) FetchBlobBytes(ctx context.Context, ref Reference, digest string) ([]byte, error) {
	host := ref.RegistryHost()
	repo := ref.Repository()
	u := fmt.Sprintf("https://%s/v2/%s/blobs/%s", host, repo, digest)
	resp, err := c.do(ctx, http.MethodGet, u, repo, c.tokenFor(repo), nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 64<<20))
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("blob %s: %s: %s", digest, resp.Status, truncate(body, 200))
	}
	hexDig, err := stripAlgo(digest)
	if err != nil {
		return nil, err
	}
	sum := sha256.Sum256(body)
	if hex.EncodeToString(sum[:]) != hexDig {
		return nil, fmt.Errorf("digest mismatch for %s", digest)
	}
	return body, nil
}

func stripAlgo(digest string) (string, error) {
	if !strings.HasPrefix(digest, "sha256:") {
		return "", fmt.Errorf("unsupported digest algorithm: %s", digest)
	}
	hexDig := strings.TrimPrefix(digest, "sha256:")
	if len(hexDig) != 64 {
		return "", fmt.Errorf("invalid digest: %s", digest)
	}
	return hexDig, nil
}

type progressWriter struct {
	w       io.Writer
	h       io.Writer
	total   int64
	written int64
	out     io.Writer
	digest  string
	last    time.Time
}

func (p *progressWriter) Write(b []byte) (int, error) {
	n, err := p.w.Write(b)
	if n > 0 {
		_, _ = p.h.Write(b[:n])
		p.written += int64(n)
		if p.out != nil && time.Since(p.last) > 200*time.Millisecond {
			p.print()
			p.last = time.Now()
		}
	}
	return n, err
}

func (p *progressWriter) finish() {
	if p.out != nil {
		p.print()
		_, _ = fmt.Fprintln(p.out)
	}
}

func (p *progressWriter) print() {
	if p.out == nil {
		return
	}
	short := p.digest
	if len(short) > 12 {
		short = short[:12]
	}
	if p.total > 0 {
		_, _ = fmt.Fprintf(p.out, "\rDownloading %s… %d/%d bytes (%.0f%%)", short, p.written, p.total, 100*float64(p.written)/float64(p.total))
	} else {
		_, _ = fmt.Fprintf(p.out, "\rDownloading %s… %d bytes", short, p.written)
	}
}
