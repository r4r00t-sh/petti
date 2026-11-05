package network

import "testing"

func TestParsePort(t *testing.T) {
	cases := []struct {
		in   string
		want PortMapping
	}{
		{"8080:80", PortMapping{HostPort: 8080, ContainerPort: 80, Protocol: "tcp"}},
		{"80", PortMapping{HostPort: 80, ContainerPort: 80, Protocol: "tcp"}},
		{"8080:80/udp", PortMapping{HostPort: 8080, ContainerPort: 80, Protocol: "udp"}},
		{"127.0.0.1:8080:80", PortMapping{HostIP: "127.0.0.1", HostPort: 8080, ContainerPort: 80, Protocol: "tcp"}},
		{"0.0.0.0:443:8443/tcp", PortMapping{HostIP: "0.0.0.0", HostPort: 443, ContainerPort: 8443, Protocol: "tcp"}},
	}
	for _, c := range cases {
		got, err := ParsePort(c.in)
		if err != nil {
			t.Fatalf("%q: %v", c.in, err)
		}
		if got != c.want {
			t.Fatalf("%q: got %+v want %+v", c.in, got, c.want)
		}
	}
}

func TestParsePortBad(t *testing.T) {
	for _, s := range []string{"", "abc", "80/sctp", "1:2:3:4", "99999:80"} {
		if _, err := ParsePort(s); err == nil {
			t.Fatalf("expected error for %q", s)
		}
	}
}

func TestParseMode(t *testing.T) {
	m, err := ParseMode("")
	if err != nil || m != ModeBridge {
		t.Fatalf("default: %q %v", m, err)
	}
	m, err = ParseMode("HOST")
	if err != nil || m != ModeHost {
		t.Fatalf("host: %q %v", m, err)
	}
	if _, err := ParseMode("macvlan"); err == nil {
		t.Fatal("expected error")
	}
}
