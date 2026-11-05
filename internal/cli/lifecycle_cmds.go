package cli

import (
	"fmt"
	"strconv"
	"strings"
	"syscall"
	"text/tabwriter"

	"github.com/r4r00t-sh/petti/internal/container"
)

func psCommand(args []string, streams IO) int {
	all := false
	for _, a := range args {
		switch a {
		case "-a", "--all":
			all = true
		case "-h", "--help":
			_, _ = fmt.Fprint(streams.Out, "Usage: petti ps [-a]\n")
			return ExitOK
		default:
			if strings.HasPrefix(a, "-") {
				_, _ = fmt.Fprintf(streams.Err, "petti ps: unknown flag %s\n", a)
				return ExitUsage
			}
		}
	}
	list, err := container.ListRecords(container.ListFilters{All: all})
	if err != nil {
		_, _ = fmt.Fprintf(streams.Err, "petti ps: %v\n", err)
		return ExitRuntime
	}
	tw := tabwriter.NewWriter(streams.Out, 0, 4, 2, ' ', 0)
	_, _ = fmt.Fprintln(tw, "CONTAINER ID\tIMAGE\tCOMMAND\tSTATUS\tIP\tPORTS\tNAME")
	for _, r := range list {
		cmd := strings.Join(r.Command, " ")
		if len(cmd) > 20 {
			cmd = cmd[:20] + "…"
		}
		name := r.Name
		if name == "" {
			name = "-"
		}
		ip := r.IP
		if ip == "" {
			ip = "-"
		}
		ports := "-"
		if len(r.Ports) > 0 {
			var ps []string
			for _, p := range r.Ports {
				ps = append(ps, p.String())
			}
			ports = strings.Join(ps, ",")
		}
		id := r.ID
		if len(id) > 12 {
			id = id[:12]
		}
		_, _ = fmt.Fprintf(tw, "%s\t%s\t%s\t%s\t%s\t%s\t%s\n", id, r.Image, cmd, r.Status, ip, ports, name)
	}
	_ = tw.Flush()
	return ExitOK
}

func stopCommand(args []string, streams IO) int {
	timeout := 10
	var positional []string
	for i := 0; i < len(args); i++ {
		a := args[i]
		switch {
		case a == "-t" || a == "--time":
			if i+1 >= len(args) {
				_, _ = fmt.Fprintf(streams.Err, "petti stop: %s requires seconds\n", a)
				return ExitUsage
			}
			n, err := strconv.Atoi(args[i+1])
			if err != nil {
				_, _ = fmt.Fprintf(streams.Err, "petti stop: invalid timeout\n")
				return ExitUsage
			}
			timeout = n
			i++
		case a == "-h" || a == "--help":
			_, _ = fmt.Fprint(streams.Out, "Usage: petti stop [-t seconds] <id|name>\n")
			return ExitOK
		case strings.HasPrefix(a, "-"):
			_, _ = fmt.Fprintf(streams.Err, "petti stop: unknown flag %s\n", a)
			return ExitUsage
		default:
			positional = append(positional, a)
		}
	}
	if len(positional) != 1 {
		_, _ = fmt.Fprintf(streams.Err, "petti stop: container required\n")
		return ExitUsage
	}
	if err := container.Stop(positional[0], timeout); err != nil {
		_, _ = fmt.Fprintf(streams.Err, "petti stop: %v\n", err)
		return ExitRuntime
	}
	_, _ = fmt.Fprintln(streams.Out, positional[0])
	return ExitOK
}

func killCommand(args []string, streams IO) int {
	var positional []string
	for _, a := range args {
		if a == "-h" || a == "--help" {
			_, _ = fmt.Fprint(streams.Out, "Usage: petti kill <id|name>\n")
			return ExitOK
		}
		if strings.HasPrefix(a, "-") {
			_, _ = fmt.Fprintf(streams.Err, "petti kill: unknown flag %s\n", a)
			return ExitUsage
		}
		positional = append(positional, a)
	}
	if len(positional) != 1 {
		_, _ = fmt.Fprintf(streams.Err, "petti kill: container required\n")
		return ExitUsage
	}
	if err := container.Kill(positional[0], syscall.SIGKILL); err != nil {
		_, _ = fmt.Fprintf(streams.Err, "petti kill: %v\n", err)
		return ExitRuntime
	}
	_, _ = fmt.Fprintln(streams.Out, positional[0])
	return ExitOK
}

func rmCommand(args []string, streams IO) int {
	force := false
	var positional []string
	for _, a := range args {
		switch a {
		case "-f", "--force":
			force = true
		case "-h", "--help":
			_, _ = fmt.Fprint(streams.Out, "Usage: petti rm [-f] <id|name>\n")
			return ExitOK
		default:
			if strings.HasPrefix(a, "-") {
				_, _ = fmt.Fprintf(streams.Err, "petti rm: unknown flag %s\n", a)
				return ExitUsage
			}
			positional = append(positional, a)
		}
	}
	if len(positional) != 1 {
		_, _ = fmt.Fprintf(streams.Err, "petti rm: container required\n")
		return ExitUsage
	}
	if err := container.Remove(positional[0], force); err != nil {
		_, _ = fmt.Fprintf(streams.Err, "petti rm: %v\n", err)
		return ExitRuntime
	}
	_, _ = fmt.Fprintln(streams.Out, positional[0])
	return ExitOK
}

func logsCommand(args []string, streams IO) int {
	follow := false
	var positional []string
	for _, a := range args {
		switch a {
		case "-f", "--follow":
			follow = true
		case "-h", "--help":
			_, _ = fmt.Fprint(streams.Out, "Usage: petti logs [-f] <id|name>\n")
			return ExitOK
		default:
			if strings.HasPrefix(a, "-") {
				_, _ = fmt.Fprintf(streams.Err, "petti logs: unknown flag %s\n", a)
				return ExitUsage
			}
			positional = append(positional, a)
		}
	}
	if len(positional) != 1 {
		_, _ = fmt.Fprintf(streams.Err, "petti logs: container required\n")
		return ExitUsage
	}
	store, err := container.OpenStore("")
	if err != nil {
		_, _ = fmt.Fprintf(streams.Err, "petti logs: %v\n", err)
		return ExitRuntime
	}
	rec, err := store.Resolve(positional[0])
	if err != nil {
		_, _ = fmt.Fprintf(streams.Err, "petti logs: %v\n", err)
		return ExitRuntime
	}
	if err := container.FollowLogs(rec.LogPath, streams.Out, follow); err != nil {
		_, _ = fmt.Fprintf(streams.Err, "petti logs: %v\n", err)
		return ExitRuntime
	}
	return ExitOK
}

func inspectCommand(args []string, streams IO) int {
	var positional []string
	for _, a := range args {
		if a == "-h" || a == "--help" {
			_, _ = fmt.Fprint(streams.Out, "Usage: petti inspect <id|name>\n")
			return ExitOK
		}
		if strings.HasPrefix(a, "-") {
			_, _ = fmt.Fprintf(streams.Err, "petti inspect: unknown flag %s\n", a)
			return ExitUsage
		}
		positional = append(positional, a)
	}
	if len(positional) != 1 {
		_, _ = fmt.Fprintf(streams.Err, "petti inspect: container required\n")
		return ExitUsage
	}
	data, err := container.InspectJSON(positional[0])
	if err != nil {
		_, _ = fmt.Fprintf(streams.Err, "petti inspect: %v\n", err)
		return ExitRuntime
	}
	_, _ = fmt.Fprintln(streams.Out, string(data))
	return ExitOK
}

func execCommand(args []string, streams IO) int {
	tty := false
	var positional []string
	i := 0
	for i < len(args) {
		a := args[i]
		switch {
		case a == "-h" || a == "--help":
			_, _ = fmt.Fprint(streams.Out, "Usage: petti exec [-it] <id|name> <cmd...>\n")
			return ExitOK
		case a == "-t" || a == "-i":
			tty = true
			i++
		case a == "-it" || a == "-ti":
			tty = true
			i++
		case strings.HasPrefix(a, "-"):
			_, _ = fmt.Fprintf(streams.Err, "petti exec: unknown flag %s\n", a)
			return ExitUsage
		default:
			positional = args[i:]
			i = len(args)
		}
	}
	if len(positional) < 2 {
		_, _ = fmt.Fprintf(streams.Err, "petti exec: container and command required\n")
		return ExitUsage
	}
	_ = tty
	err := container.Exec(container.ExecOptions{
		IDOrName: positional[0],
		Args:     positional[1:],
		Tty:      tty,
		Stdin:    streams.In,
		Stdout:   streams.Out,
		Stderr:   streams.Err,
	})
	if err != nil {
		_, _ = fmt.Fprintf(streams.Err, "petti exec: %v\n", err)
		return ExitRuntime
	}
	return ExitOK
}
