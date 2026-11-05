package container

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
)

// GenerateID returns a 12-byte (24 hex char) container ID, similar in spirit
// to short Docker IDs. The first 12 hex characters are a convenient hostname prefix.
func GenerateID() (string, error) {
	var b [12]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", fmt.Errorf("generate container id: %w", err)
	}
	return hex.EncodeToString(b[:]), nil
}

// HostnamePrefix returns a DNS-safe hostname derived from the container ID.
func HostnamePrefix(id string) string {
	const max = 12
	if len(id) <= max {
		return id
	}
	return id[:max]
}
