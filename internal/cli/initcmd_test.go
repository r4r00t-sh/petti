package cli

import (
	"testing"
)

func TestParseInitArgs(t *testing.T) {
	cfg, err := parseInitArgs([]string{
		"--rootfs", "/tmp/root",
		"--hostname", "box",
		"--workdir", "/tmp",
		"--env", "A=1",
		"--env", "B=2",
		"--user", "1000:100",
		"--sync-fd", "3",
		"--network", "none",
		"--",
		"/bin/sh", "-c", "echo hi",
	})
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Rootfs != "/tmp/root" || cfg.Hostname != "box" || cfg.SyncFD != 3 {
		t.Fatalf("%+v", cfg)
	}
	if cfg.NetworkMode != "none" {
		t.Fatalf("network: %+v", cfg)
	}
	if cfg.Workdir != "/tmp" || !cfg.User.Set || cfg.User.UID != 1000 || cfg.User.GID != 100 {
		t.Fatalf("%+v", cfg)
	}
	if len(cfg.Env) != 2 || cfg.Env[0] != "A=1" {
		t.Fatalf("env: %#v", cfg.Env)
	}
	if len(cfg.Args) != 3 || cfg.Args[0] != "/bin/sh" {
		t.Fatalf("args: %#v", cfg.Args)
	}
}

func TestParseInitArgsMissing(t *testing.T) {
	_, err := parseInitArgs([]string{"--rootfs", "/tmp", "--hostname", "x", "--sync-fd", "3"})
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestParseRunArgs(t *testing.T) {
	f, err := parseRunArgs([]string{
		"--rootfs=/data/root",
		"--hostname=web",
		"-w", "/app",
		"--env", "K=V",
		"--user=1:2",
		"--entrypoint", "/bin/echo",
		"--memory", "64m",
		"--cpus=0.5",
		"--pids-limit", "32",
		"--network", "bridge",
		"-p", "8080:80",
		"-p", "53:53/udp",
		"--cap-add", "SYS_ADMIN",
		"--cap-drop", "NET_RAW",
		"--security-opt", "seccomp=unconfined",
		"--read-only",
		"ps", "aux",
	})
	if err != nil {
		t.Fatal(err)
	}
	if f.Rootfs != "/data/root" || f.Hostname != "web" || f.Workdir != "/app" {
		t.Fatalf("%+v", f)
	}
	if f.User != "1:2" || f.Entrypoint != "/bin/echo" || len(f.Env) != 1 {
		t.Fatalf("%+v", f)
	}
	if f.Memory != "64m" || f.CPUs != "0.5" || f.PidsLimit != "32" {
		t.Fatalf("%+v", f)
	}
	if f.Network != "bridge" || len(f.Publish) != 2 {
		t.Fatalf("network: %+v", f)
	}
	if len(f.CapAdd) != 1 || f.CapAdd[0] != "SYS_ADMIN" || len(f.CapDrop) != 1 {
		t.Fatalf("caps: %+v", f)
	}
	if len(f.SecurityOpts) != 1 || !f.ReadOnly {
		t.Fatalf("security: %+v", f)
	}
	// With --rootfs set, remaining args are the command.
	if len(f.Args) != 2 || f.Args[0] != "ps" {
		t.Fatalf("%#v", f.Args)
	}
}
