package network

import (
	"fmt"
	"os"
	"path/filepath"
)

// WriteDNSFiles writes hosts and resolv.conf into dir for bind-mounting into the container.
// For bridge mode, hosts includes the container IP and hostname.
func WriteDNSFiles(dir, hostname, ip string) (hostsPath, resolvPath string, err error) {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", "", err
	}
	hostsPath = filepath.Join(dir, "hosts")
	resolvPath = filepath.Join(dir, "resolv.conf")

	hosts := "127.0.0.1\tlocalhost\n::1\tlocalhost ip6-localhost ip6-loopback\n"
	if ip != "" && hostname != "" {
		hosts += fmt.Sprintf("%s\t%s\n", ip, hostname)
	} else if hostname != "" {
		hosts += fmt.Sprintf("127.0.0.1\t%s\n", hostname)
	}
	if err := os.WriteFile(hostsPath, []byte(hosts), 0o644); err != nil {
		return "", "", err
	}

	resolv := ""
	if data, err := os.ReadFile("/etc/resolv.conf"); err == nil && len(data) > 0 {
		resolv = string(data)
	} else {
		resolv = "nameserver 8.8.8.8\nnameserver 1.1.1.1\n"
	}
	if err := os.WriteFile(resolvPath, []byte(resolv), 0o644); err != nil {
		return "", "", err
	}
	return hostsPath, resolvPath, nil
}
