package runtime

import (
	"fmt"
	"strconv"
	"strings"
)

// UserSpec is a numeric uid/gid pair from --user uid[:gid].
type UserSpec struct {
	UID    int
	GID    int
	Set    bool // false → leave as current (typically root)
	HasGID bool
}

// ParseUser parses "uid" or "uid:gid". Username resolution is out of scope for M2.
func ParseUser(s string) (UserSpec, error) {
	var u UserSpec
	if s == "" {
		return u, nil
	}
	parts := strings.SplitN(s, ":", 2)
	uid, err := strconv.Atoi(parts[0])
	if err != nil || uid < 0 {
		return u, fmt.Errorf("invalid uid in --user %q", s)
	}
	u.UID = uid
	u.Set = true
	if len(parts) == 1 {
		u.GID = uid
		return u, nil
	}
	gid, err := strconv.Atoi(parts[1])
	if err != nil || gid < 0 {
		return u, fmt.Errorf("invalid gid in --user %q", s)
	}
	u.GID = gid
	u.HasGID = true
	return u, nil
}

// ParseEnv returns key, value for a KEY=VAL string.
func ParseEnv(s string) (key, val string, err error) {
	if s == "" {
		return "", "", fmt.Errorf("empty --env")
	}
	k, v, ok := strings.Cut(s, "=")
	if !ok || k == "" {
		return "", "", fmt.Errorf("invalid --env %q (want KEY=VAL)", s)
	}
	return k, v, nil
}

// MergeEnv builds the final environment: defaults, then overrides (later wins).
func MergeEnv(defaults []string, overrides []string) ([]string, error) {
	env := map[string]string{}
	order := make([]string, 0, len(defaults)+len(overrides))

	add := func(s string) error {
		k, v, err := ParseEnv(s)
		if err != nil {
			return err
		}
		if _, exists := env[k]; !exists {
			order = append(order, k)
		}
		env[k] = v
		return nil
	}
	for _, d := range defaults {
		if err := add(d); err != nil {
			return nil, err
		}
	}
	for _, o := range overrides {
		if err := add(o); err != nil {
			return nil, err
		}
	}
	out := make([]string, 0, len(order))
	for _, k := range order {
		out = append(out, k+"="+env[k])
	}
	return out, nil
}

// MergeEntrypoint prepends entrypoint to cmd args (Docker-style).
func MergeEntrypoint(entrypoint []string, cmd []string) []string {
	if len(entrypoint) == 0 {
		return cmd
	}
	out := make([]string, 0, len(entrypoint)+len(cmd))
	out = append(out, entrypoint...)
	out = append(out, cmd...)
	return out
}
