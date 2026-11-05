package registry

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
)

type tokenCache struct {
	mu     sync.Mutex
	tokens map[string]string
}

func newTokenCache() *tokenCache {
	return &tokenCache{tokens: map[string]string{}}
}

type challenge struct {
	realm   string
	service string
	scope   string
}

func parseWWWAuthenticate(h string) (challenge, error) {
	// Bearer realm="…",service="…",scope="…"
	var c challenge
	h = strings.TrimSpace(h)
	if !strings.HasPrefix(strings.ToLower(h), "bearer ") {
		return c, fmt.Errorf("unsupported auth challenge: %s", h)
	}
	rest := strings.TrimSpace(h[len("Bearer "):])
	for _, part := range strings.Split(rest, ",") {
		part = strings.TrimSpace(part)
		kv := strings.SplitN(part, "=", 2)
		if len(kv) != 2 {
			continue
		}
		key := strings.ToLower(strings.TrimSpace(kv[0]))
		val := strings.Trim(strings.TrimSpace(kv[1]), `"`)
		switch key {
		case "realm":
			c.realm = val
		case "service":
			c.service = val
		case "scope":
			c.scope = val
		}
	}
	if c.realm == "" {
		return c, fmt.Errorf("auth challenge missing realm")
	}
	return c, nil
}

func (c *Client) fetchToken(ctx context.Context, ch challenge, repo string) (string, error) {
	scope := ch.scope
	if scope == "" {
		scope = "repository:" + repo + ":pull"
	}
	u, err := url.Parse(ch.realm)
	if err != nil {
		return "", err
	}
	q := u.Query()
	if ch.service != "" {
		q.Set("service", ch.service)
	}
	q.Set("scope", scope)
	u.RawQuery = q.Encode()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return "", err
	}
	resp, err := c.HTTP.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return "", err
	}
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("token %s: %s: %s", u.Host, resp.Status, truncate(body, 200))
	}
	var out struct {
		Token       string `json:"token"`
		AccessToken string `json:"access_token"`
	}
	if err := json.Unmarshal(body, &out); err != nil {
		return "", err
	}
	tok := out.Token
	if tok == "" {
		tok = out.AccessToken
	}
	if tok == "" {
		return "", fmt.Errorf("token response missing token")
	}
	return tok, nil
}

func truncate(b []byte, n int) string {
	if len(b) <= n {
		return string(b)
	}
	return string(b[:n]) + "…"
}
