package auth

import (
	"testing"
)

func TestGenerateLocalToken(t *testing.T) {
	t.Run("returns 64 character hex string", func(t *testing.T) {
		token, err := GenerateLocalToken()
		if err != nil {
			t.Fatalf("GenerateLocalToken failed: %v", err)
		}
		if len(token) != 64 {
			t.Errorf("expected 64 characters, got %d", len(token))
		}
	})

	t.Run("two calls produce different tokens", func(t *testing.T) {
		t1, err := GenerateLocalToken()
		if err != nil {
			t.Fatalf("first call failed: %v", err)
		}
		t2, err := GenerateLocalToken()
		if err != nil {
			t.Fatalf("second call failed: %v", err)
		}
		if t1 == t2 {
			t.Error("expected different tokens from two calls")
		}
	})
}
