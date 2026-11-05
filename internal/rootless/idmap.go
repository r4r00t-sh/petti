package rootless

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"os/user"
	"strconv"
	"strings"
)

// IDMap is one line of a uid_map / gid_map.
type IDMap struct {
	ContainerID int
	HostID      int
	Size        int
}

// Mapping is the full set of uid/gid mappings for a container user namespace.
type Mapping struct {
	UID []IDMap
	GID []IDMap
	// UseNewuidmap is true when subordinate ranges require the setuid helpers.
	UseNewuidmap bool
}

// SubIDRange is one entry from /etc/subuid or /etc/subgid.
type SubIDRange struct {
	Start int
	Count int
}

// BuildMapping maps the current user to container root (0) and, when possible,
// maps the subordinate ID range to container IDs 1..count.
func BuildMapping() (Mapping, error) {
	uid := os.Getuid()
	gid := os.Getgid()

	m := Mapping{
		UID: []IDMap{{ContainerID: 0, HostID: uid, Size: 1}},
		GID: []IDMap{{ContainerID: 0, HostID: gid, Size: 1}},
	}

	username := ""
	if u, err := user.Current(); err == nil {
		username = u.Username
	}

	subU, errU := lookupSubID("/etc/subuid", username, uid)
	subG, errG := lookupSubID("/etc/subgid", username, gid)
	haveNew := hasHelper("newuidmap") && hasHelper("newgidmap")

	if errU == nil && errG == nil && haveNew && subU.Count > 0 && subG.Count > 0 {
		m.UID = append(m.UID, IDMap{ContainerID: 1, HostID: subU.Start, Size: subU.Count})
		m.GID = append(m.GID, IDMap{ContainerID: 1, HostID: subG.Start, Size: subG.Count})
		m.UseNewuidmap = true
		return m, nil
	}

	// Single-uid fallback (no uidmap package / no subuid): writable via /proc.
	m.UseNewuidmap = false
	return m, nil
}

func hasHelper(name string) bool {
	_, err := exec.LookPath(name)
	return err == nil
}

func lookupSubID(path, username string, id int) (SubIDRange, error) {
	f, err := os.Open(path)
	if err != nil {
		return SubIDRange{}, err
	}
	defer f.Close()

	idStr := strconv.Itoa(id)
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		parts := strings.Split(line, ":")
		if len(parts) != 3 {
			continue
		}
		name := parts[0]
		if name != username && name != idStr {
			continue
		}
		start, err1 := strconv.Atoi(parts[1])
		count, err2 := strconv.Atoi(parts[2])
		if err1 != nil || err2 != nil || count <= 0 {
			continue
		}
		return SubIDRange{Start: start, Count: count}, nil
	}
	if err := sc.Err(); err != nil {
		return SubIDRange{}, err
	}
	return SubIDRange{}, fmt.Errorf("no subid entry for %q in %s", username, path)
}

// FormatProcMap formats mappings for writing to /proc/pid/uid_map.
func FormatProcMap(maps []IDMap) string {
	var b strings.Builder
	for _, m := range maps {
		fmt.Fprintf(&b, "%d %d %d\n", m.ContainerID, m.HostID, m.Size)
	}
	return b.String()
}
