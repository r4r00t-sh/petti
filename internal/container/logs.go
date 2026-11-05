package container

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"time"
)

// FollowLogs copies logPath to w; if follow, polls for new data like tail -f.
func FollowLogs(logPath string, w io.Writer, follow bool) error {
	f, err := os.Open(logPath)
	if err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("no logs for container")
		}
		return err
	}
	defer f.Close()
	if !follow {
		_, err := io.Copy(w, f)
		return err
	}
	r := bufio.NewReader(f)
	for {
		line, err := r.ReadString('\n')
		if len(line) > 0 {
			if _, werr := io.WriteString(w, line); werr != nil {
				return werr
			}
		}
		if err == io.EOF {
			time.Sleep(200 * time.Millisecond)
			continue
		}
		if err != nil {
			return err
		}
	}
}
