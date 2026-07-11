package auth

import (
	"crypto/rand"
	"fmt"
)

func GenerateLocalToken() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("rand read: %w", err)
	}
	return fmt.Sprintf("%x", b), nil
}
