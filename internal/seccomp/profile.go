package seccomp

import "fmt"

// Mode selects which filter (if any) to apply.
type Mode string

const (
	// ModeDefault is Petti's denylist profile.
	ModeDefault Mode = "default"
	// ModeUnconfined disables seccomp.
	ModeUnconfined Mode = "unconfined"
)

// ParseMode accepts values from --security-opt seccomp=...
func ParseMode(s string) (Mode, error) {
	switch s {
	case "", "default", "petti/default":
		return ModeDefault, nil
	case "unconfined":
		return ModeUnconfined, nil
	default:
		return "", fmt.Errorf("unsupported seccomp mode %q (want default|unconfined)", s)
	}
}

// ParseSecurityOpt parses a single --security-opt value (e.g. seccomp=unconfined).
func ParseSecurityOpt(opt string) (key, value string, err error) {
	for i := 0; i < len(opt); i++ {
		if opt[i] == '=' {
			return opt[:i], opt[i+1:], nil
		}
	}
	return "", "", fmt.Errorf("invalid --security-opt %q (want key=value)", opt)
}
