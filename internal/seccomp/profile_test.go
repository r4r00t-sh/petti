package seccomp

import "testing"

func TestParseMode(t *testing.T) {
	m, err := ParseMode("")
	if err != nil || m != ModeDefault {
		t.Fatalf("%q %v", m, err)
	}
	m, err = ParseMode("unconfined")
	if err != nil || m != ModeUnconfined {
		t.Fatalf("%q %v", m, err)
	}
	if _, err := ParseMode("strict"); err == nil {
		t.Fatal("expected error")
	}
}

func TestParseSecurityOpt(t *testing.T) {
	k, v, err := ParseSecurityOpt("seccomp=unconfined")
	if err != nil || k != "seccomp" || v != "unconfined" {
		t.Fatalf("%s=%s %v", k, v, err)
	}
	if _, _, err := ParseSecurityOpt("noequals"); err == nil {
		t.Fatal("expected error")
	}
}

func TestBlockedNamesNonEmpty(t *testing.T) {
	if len(BlockedNames()) < 10 {
		t.Fatalf("%v", BlockedNames())
	}
}
