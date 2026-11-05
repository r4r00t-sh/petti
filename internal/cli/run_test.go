package cli_test

import (
	"strings"
	"testing"

	"github.com/r4r00t-sh/petti/internal/cli"
)

func TestRunRequiresImageOrRootfs(t *testing.T) {
	_, errOut, code := run("run")
	if code != cli.ExitUsage {
		t.Fatalf("exit = %d, want %d", code, cli.ExitUsage)
	}
	if !strings.Contains(errOut, "image") && !strings.Contains(errOut, "rootfs") {
		t.Fatalf("stderr: %q", errOut)
	}

	_, errOut, code = run("run", "--rootfs", "/tmp")
	if code != cli.ExitUsage {
		t.Fatalf("exit = %d, want %d", code, cli.ExitUsage)
	}
	if !strings.Contains(errOut, "command or --entrypoint is required") {
		t.Fatalf("stderr: %q", errOut)
	}
}

func TestRunHelp(t *testing.T) {
	out, errOut, code := run("run", "--help")
	if code != cli.ExitOK {
		t.Fatalf("exit = %d", code)
	}
	if errOut != "" {
		t.Fatalf("stderr: %q", errOut)
	}
	if !strings.Contains(out, "--rootfs") {
		t.Fatalf("help: %q", out)
	}
}

func TestRunUnknownFlag(t *testing.T) {
	_, errOut, code := run("run", "--nope", "--rootfs", "/tmp", "true")
	if code != cli.ExitUsage {
		t.Fatalf("exit = %d", code)
	}
	if !strings.Contains(errOut, "unknown flag") {
		t.Fatalf("stderr: %q", errOut)
	}
}

func TestRunMissingRootfsDir(t *testing.T) {
	_, errOut, code := run("run", "--rootfs", "/no/such/petti/rootfs", "true")
	if code != cli.ExitRuntime {
		t.Fatalf("exit = %d, want %d; stderr=%q", code, cli.ExitRuntime, errOut)
	}
	if !strings.Contains(errOut, "rootfs") {
		t.Fatalf("stderr: %q", errOut)
	}
}

func TestRunEntrypointOnlyOK(t *testing.T) {
	_, errOut, code := run("run", "--rootfs", "/no/such/petti/rootfs", "--entrypoint", "/bin/true")
	if code != cli.ExitRuntime {
		t.Fatalf("exit = %d, stderr=%q", code, errOut)
	}
}

func TestParseRunEnvWorkdirUser(t *testing.T) {
	out, _, code := run("run", "--help")
	if code != cli.ExitOK {
		t.Fatal(code)
	}
	for _, want := range []string{"--workdir", "--env", "--user", "--entrypoint", "--memory", "--cpus", "--pids-limit"} {
		if !strings.Contains(out, want) {
			t.Fatalf("help missing %q", want)
		}
	}
}

func TestRunBadMemory(t *testing.T) {
	_, errOut, code := run("run", "--rootfs", "/tmp", "--memory", "xyz", "true")
	if code != cli.ExitRuntime {
		t.Fatalf("exit=%d stderr=%q", code, errOut)
	}
	if !strings.Contains(errOut, "memory") {
		t.Fatalf("stderr=%q", errOut)
	}
}

func TestRunMissingLocalImage(t *testing.T) {
	_, errOut, code := run("run", "petti-no-such-image:latest", "true")
	if code != cli.ExitRuntime {
		t.Fatalf("exit=%d stderr=%q", code, errOut)
	}
	if !strings.Contains(errOut, "not found") && !strings.Contains(errOut, "pull") {
		t.Fatalf("stderr=%q", errOut)
	}
}
