package cgroups

import (
	"fmt"
	"math"
	"strconv"
	"strings"
	"unicode"
)

// ParseBytes parses Docker-ish sizes: "128m", "1G", "512k", or raw bytes.
func ParseBytes(s string) (int64, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0, fmt.Errorf("empty memory size")
	}
	if s == "0" {
		return 0, nil
	}

	mult := int64(1)
	last := s[len(s)-1]
	num := s
	if !unicode.IsDigit(rune(last)) {
		num = s[:len(s)-1]
		switch unicode.ToLower(rune(last)) {
		case 'k':
			mult = 1024
		case 'm':
			mult = 1024 * 1024
		case 'g':
			mult = 1024 * 1024 * 1024
		case 't':
			mult = 1024 * 1024 * 1024 * 1024
		case 'b':
			mult = 1
		default:
			return 0, fmt.Errorf("invalid memory size %q", s)
		}
	}
	v, err := strconv.ParseInt(strings.TrimSpace(num), 10, 64)
	if err != nil || v < 0 {
		return 0, fmt.Errorf("invalid memory size %q", s)
	}
	return v * mult, nil
}

// ParseCPUs parses a CPU quota like "0.5" or "2".
func ParseCPUs(s string) (float64, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0, fmt.Errorf("empty cpus value")
	}
	v, err := strconv.ParseFloat(s, 64)
	if err != nil || v < 0 || math.IsNaN(v) || math.IsInf(v, 0) {
		return 0, fmt.Errorf("invalid cpus value %q", s)
	}
	return v, nil
}

// ParsePids parses a process limit.
func ParsePids(s string) (int64, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0, fmt.Errorf("empty pids limit")
	}
	v, err := strconv.ParseInt(s, 10, 64)
	if err != nil || v < 0 {
		return 0, fmt.Errorf("invalid pids limit %q", s)
	}
	return v, nil
}

// CPUMax returns the cgroup v2 cpu.max payload for the given CPU count.
// Period is 100000 µs (100ms), matching common Docker/systemd defaults.
func CPUMax(cpus float64) string {
	const period = 100000
	if cpus <= 0 {
		return "max"
	}
	quota := int64(math.Round(cpus * period))
	if quota < 1000 {
		// Kernel rejects very small quotas; keep a tiny but valid floor.
		quota = 1000
	}
	return fmt.Sprintf("%d %d", quota, period)
}
