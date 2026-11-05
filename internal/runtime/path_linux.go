//go:build linux

package runtime

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// resolveArgv0 finds the executable inside the already-chrooted root.
func resolveArgv0(argv0 string) (string, error) {
	if argv0 == "" {
		return "", fmt.Errorf("empty command")
	}
	if strings.Contains(argv0, "/") {
		if err := checkExec(argv0); err != nil {
			return "", err
		}
		return argv0, nil
	}
	for _, dir := range []string{"/bin", "/usr/bin", "/sbin", "/usr/sbin"} {
		p := filepath.Join(dir, argv0)
		if err := checkExec(p); err == nil {
			return p, nil
		}
	}
	return "", fmt.Errorf("executable %q not found in /bin:/usr/bin:/sbin:/usr/sbin", argv0)
}

func checkExec(path string) error {
	st, err := os.Stat(path)
	if err != nil {
		return err
	}
	if st.IsDir() {
		return fmt.Errorf("%s is a directory", path)
	}
	// Mode check is advisory; Exec will fail if not executable.
	if st.Mode()&0o111 == 0 {
		return fmt.Errorf("%s is not executable", path)
	}
	return nil
}
