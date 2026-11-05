//go:build linux

package runtime

import (
	"os"
	"strings"
)

func rootlessStartHint() string {
	var parts []string
	parts = append(parts, "rootless userns failed")
	if raw, err := os.ReadFile("/proc/sys/kernel/apparmor_restrict_unprivileged_userns"); err == nil {
		if strings.TrimSpace(string(raw)) == "1" {
			parts = append(parts, "Ubuntu AppArmor is restricting unprivileged userns — try: sudo sysctl -w kernel.apparmor_restrict_unprivileged_userns=0 (or load docs/apparmor/petti)")
		}
	}
	parts = append(parts, "ensure /etc/subuid+/etc/subgid exist; apt install uidmap for subordinate ranges")
	return strings.Join(parts, "; ")
}
