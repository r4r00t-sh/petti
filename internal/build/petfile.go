package build

import (
	"bufio"
	"fmt"
	"os"
	"strings"
	"unicode"
)

// Kind is a Petfile instruction.
type Kind string

const (
	From       Kind = "FROM"
	Run        Kind = "RUN"
	Copy       Kind = "COPY"
	Env        Kind = "ENV"
	Workdir    Kind = "WORKDIR"
	Cmd        Kind = "CMD"
	Entrypoint Kind = "ENTRYPOINT"
)

// Inst is one parsed instruction.
type Inst struct {
	Kind Kind
	Args []string // already split; RUN keeps a single shell string in Args[0]
	Raw  string
}

// ParseFile reads a Petfile from path.
func ParseFile(path string) ([]Inst, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	return Parse(f)
}

// Parse reads Petfile instructions from r.
func Parse(r interface{ Read([]byte) (int, error) }) ([]Inst, error) {
	sc := bufio.NewScanner(r)
	sc.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	var (
		out     []Inst
		pending string
	)
	lineNo := 0
	flush := func(line string) error {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			return nil
		}
		inst, err := parseLine(line)
		if err != nil {
			return fmt.Errorf("line %d: %w", lineNo, err)
		}
		out = append(out, inst)
		return nil
	}
	for sc.Scan() {
		lineNo++
		line := sc.Text()
		trimmed := strings.TrimRightFunc(line, unicode.IsSpace)
		if strings.HasSuffix(trimmed, "\\") {
			pending += strings.TrimSuffix(trimmed, "\\") + " "
			continue
		}
		if pending != "" {
			line = pending + strings.TrimSpace(line)
			pending = ""
		}
		if err := flush(line); err != nil {
			return nil, err
		}
	}
	if pending != "" {
		return nil, fmt.Errorf("line %d: continuation at EOF", lineNo)
	}
	if err := sc.Err(); err != nil {
		return nil, err
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("empty Petfile")
	}
	if out[0].Kind != From {
		return nil, fmt.Errorf("petfile must start with FROM")
	}
	return out, nil
}

func parseLine(line string) (Inst, error) {
	fields := strings.Fields(line)
	if len(fields) == 0 {
		return Inst{}, fmt.Errorf("empty instruction")
	}
	kind := Kind(strings.ToUpper(fields[0]))
	rest := strings.TrimSpace(line[len(fields[0]):])
	inst := Inst{Kind: kind, Raw: line}

	switch kind {
	case From:
		if rest == "" {
			return Inst{}, fmt.Errorf("FROM requires an image")
		}
		inst.Args = []string{fields[1]}
		if len(fields) > 2 {
			return Inst{}, fmt.Errorf("FROM: unexpected args")
		}
	case Run:
		if rest == "" {
			return Inst{}, fmt.Errorf("RUN requires a command")
		}
		inst.Args = []string{rest}
	case Copy:
		// COPY src... dest
		if len(fields) < 3 {
			return Inst{}, fmt.Errorf("COPY requires src and dest")
		}
		inst.Args = fields[1:]
	case Env:
		// ENV KEY=VAL [KEY=VAL...]  or  ENV KEY VAL
		if rest == "" {
			return Inst{}, fmt.Errorf("ENV requires KEY=VAL")
		}
		if strings.Contains(rest, "=") {
			inst.Args = fields[1:]
		} else {
			if len(fields) != 3 {
				return Inst{}, fmt.Errorf("ENV KEY VAL requires two args")
			}
			inst.Args = []string{fields[1] + "=" + fields[2]}
		}
	case Workdir:
		if len(fields) != 2 {
			return Inst{}, fmt.Errorf("WORKDIR requires one path")
		}
		inst.Args = []string{fields[1]}
	case Cmd, Entrypoint:
		if rest == "" {
			return Inst{}, fmt.Errorf("%s requires arguments", kind)
		}
		if strings.HasPrefix(rest, "[") {
			args, err := parseJSONArray(rest)
			if err != nil {
				return Inst{}, err
			}
			inst.Args = args
		} else {
			inst.Args = []string{"/bin/sh", "-c", rest}
		}
	default:
		return Inst{}, fmt.Errorf("unknown instruction %q", fields[0])
	}
	return inst, nil
}

func parseJSONArray(s string) ([]string, error) {
	s = strings.TrimSpace(s)
	if !strings.HasPrefix(s, "[") || !strings.HasSuffix(s, "]") {
		return nil, fmt.Errorf("invalid JSON array %q", s)
	}
	inner := strings.TrimSpace(s[1 : len(s)-1])
	if inner == "" {
		return nil, nil
	}
	var out []string
	for _, part := range splitCSV(inner) {
		part = strings.TrimSpace(part)
		if len(part) >= 2 && ((part[0] == '"' && part[len(part)-1] == '"') || (part[0] == '\'' && part[len(part)-1] == '\'')) {
			part = part[1 : len(part)-1]
		}
		out = append(out, part)
	}
	return out, nil
}

func splitCSV(s string) []string {
	var (
		out   []string
		cur   strings.Builder
		quote rune
	)
	for _, r := range s {
		switch {
		case quote != 0:
			if r == quote {
				quote = 0
			}
			cur.WriteRune(r)
		case r == '"' || r == '\'':
			quote = r
			cur.WriteRune(r)
		case r == ',':
			out = append(out, cur.String())
			cur.Reset()
		default:
			cur.WriteRune(r)
		}
	}
	out = append(out, cur.String())
	return out
}
