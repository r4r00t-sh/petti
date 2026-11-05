package cli_test

import (
	"bytes"
	"strings"
	"testing"

	"github.com/r4r00t-sh/petti/internal/cli"
	"github.com/r4r00t-sh/petti/internal/version"
)

func run(args ...string) (stdout, stderr string, code int) {
	var out, errBuf bytes.Buffer
	code = cli.Execute(args, cli.IO{Out: &out, Err: &errBuf})
	return out.String(), errBuf.String(), code
}

func TestVersionSpellingsProduceIdenticalOutput(t *testing.T) {
	spellings := [][]string{
		{"-v"},
		{"--v"},
		{"--version"},
		{"version"},
	}

	var firstOut string
	var firstCode int
	for i, args := range spellings {
		out, errOut, code := run(args...)
		if errOut != "" {
			t.Fatalf("%v: unexpected stderr: %q", args, errOut)
		}
		if code != cli.ExitOK {
			t.Fatalf("%v: exit code = %d, want %d", args, code, cli.ExitOK)
		}
		if !strings.HasPrefix(out, "petti version ") {
			t.Fatalf("%v: output %q missing version prefix", args, out)
		}
		if !strings.HasSuffix(out, "\n") {
			t.Fatalf("%v: output missing trailing newline: %q", args, out)
		}
		if i == 0 {
			firstOut = out
			firstCode = code
			continue
		}
		if out != firstOut {
			t.Errorf("%v output differs from -v:\n  got:  %q\n  want: %q", args, out, firstOut)
		}
		if code != firstCode {
			t.Errorf("%v exit code %d != %d", args, code, firstCode)
		}
	}

	// Sanity: matches version.String() plus newline.
	want := version.String() + "\n"
	if firstOut != want {
		t.Errorf("version line = %q, want %q", firstOut, want)
	}
}

func TestHelpAndNoArgs(t *testing.T) {
	cases := []struct {
		name string
		args []string
	}{
		{"no args", nil},
		{"-h", []string{"-h"}},
		{"--help", []string{"--help"}},
		{"help", []string{"help"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			out, errOut, code := run(tc.args...)
			if code != cli.ExitOK {
				t.Fatalf("exit = %d, want %d", code, cli.ExitOK)
			}
			if errOut != "" {
				t.Fatalf("stderr = %q", errOut)
			}
			if !strings.Contains(out, "Usage:") {
				t.Fatalf("help missing Usage: %q", out)
			}
			if out != cli.Usage() {
				t.Fatalf("help text mismatch")
			}
		})
	}
}

func TestUnknownCommandExitUsage(t *testing.T) {
	out, errOut, code := run("not-a-real-command")
	if code != cli.ExitUsage {
		t.Fatalf("exit = %d, want %d", code, cli.ExitUsage)
	}
	if out != "" {
		t.Fatalf("stdout should be empty, got %q", out)
	}
	if !strings.Contains(errOut, "unknown command") {
		t.Fatalf("stderr missing unknown command: %q", errOut)
	}
	if !strings.Contains(errOut, "Usage:") {
		t.Fatalf("stderr should include usage: %q", errOut)
	}
}

func TestUnknownFlagExitUsage(t *testing.T) {
	_, errOut, code := run("--nope")
	if code != cli.ExitUsage {
		t.Fatalf("exit = %d, want %d", code, cli.ExitUsage)
	}
	if !strings.Contains(errOut, "unknown flag") {
		t.Fatalf("stderr missing unknown flag: %q", errOut)
	}
}
