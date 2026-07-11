package crypto

import (
	"bytes"
	"encoding/base64"
	"testing"
)

func TestDeriveKey(t *testing.T) {
	t.Run("same secret produces same key", func(t *testing.T) {
		k1 := DeriveKey("my-secret")
		k2 := DeriveKey("my-secret")
		if !bytes.Equal(k1, k2) {
			t.Error("expected identical keys for same secret")
		}
	})

	t.Run("different secrets produce different keys", func(t *testing.T) {
		k1 := DeriveKey("secret-a")
		k2 := DeriveKey("secret-b")
		if bytes.Equal(k1, k2) {
			t.Error("expected different keys for different secrets")
		}
	})

	t.Run("empty string secret", func(t *testing.T) {
		k := DeriveKey("")
		if len(k) == 0 {
			t.Error("expected non-empty key for empty secret")
		}
	})
}

func TestEncryptDecrypt(t *testing.T) {
	key := DeriveKey("test-secret")

	tests := []struct {
		name string
		data []byte
	}{
		{"short string", []byte("hello world")},
		{"long string", []byte(`Lorem ipsum dolor sit amet, consectetur adipiscing elit, sed do eiusmod tempor incididunt ut labore et dolore magna aliqua.`)},
		{"empty bytes", []byte{}},
		{"binary data", []byte{0x00, 0xFF, 0xAB, 0xCD, 0x01, 0x02, 0x7F, 0x80}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ciphertext, err := Encrypt(tt.data, key)
			if err != nil {
				t.Fatalf("encrypt failed: %v", err)
			}
			plaintext, err := Decrypt(ciphertext, key)
			if err != nil {
				t.Fatalf("decrypt failed: %v", err)
			}
			if !bytes.Equal(plaintext, tt.data) {
				t.Errorf("roundtrip mismatch: got %x, want %x", plaintext, tt.data)
			}
		})
	}
}

func TestEncryptRandomNonce(t *testing.T) {
	key := DeriveKey("nonce-test")
	plaintext := []byte("same message")

	c1, err := Encrypt(plaintext, key)
	if err != nil {
		t.Fatalf("first encrypt failed: %v", err)
	}
	c2, err := Encrypt(plaintext, key)
	if err != nil {
		t.Fatalf("second encrypt failed: %v", err)
	}

	if c1 == c2 {
		t.Error("expected different ciphertexts due to random nonce")
	}
}

func TestEncryptDecryptWrongKey(t *testing.T) {
	key1 := DeriveKey("correct-key")
	key2 := DeriveKey("wrong-key")

	ciphertext, err := Encrypt([]byte("secret data"), key1)
	if err != nil {
		t.Fatalf("encrypt failed: %v", err)
	}

	_, err = Decrypt(ciphertext, key2)
	if err == nil {
		t.Fatal("expected error decrypting with wrong key")
	}
}

func TestDecryptErrors(t *testing.T) {
	key := DeriveKey("test-secret")

	t.Run("invalid base64 input", func(t *testing.T) {
		_, err := Decrypt("!!!not-base64!!!", key)
		if err == nil {
			t.Fatal("expected error for invalid base64")
		}
	})

	t.Run("truncated ciphertext too short", func(t *testing.T) {
		short := base64.RawStdEncoding.EncodeToString([]byte("short"))
		_, err := Decrypt(short, key)
		if err == nil {
			t.Fatal("expected error for truncated ciphertext")
		}
	})

	t.Run("tampered ciphertext", func(t *testing.T) {
		ciphertext, err := Encrypt([]byte("tamper me"), key)
		if err != nil {
			t.Fatalf("encrypt failed: %v", err)
		}

		data, _ := base64.RawStdEncoding.DecodeString(ciphertext)
		data[len(data)-1] ^= 0xFF
		tampered := base64.RawStdEncoding.EncodeToString(data)

		_, err = Decrypt(tampered, key)
		if err == nil {
			t.Fatal("expected error for tampered ciphertext")
		}
	})

	t.Run("wrong key gcm auth failure", func(t *testing.T) {
		k1 := DeriveKey("real-key")
		k2 := DeriveKey("fake-key")
		ct, err := Encrypt([]byte("auth test"), k1)
		if err != nil {
			t.Fatalf("encrypt failed: %v", err)
		}
		_, err = Decrypt(ct, k2)
		if err == nil {
			t.Fatal("expected gcm auth error with wrong key")
		}
	})
}

func TestNewEncryptionKey(t *testing.T) {
	t.Run("generates 32 byte key", func(t *testing.T) {
		key, err := NewEncryptionKey()
		if err != nil {
			t.Fatalf("NewEncryptionKey failed: %v", err)
		}
		if len(key) != 32 {
			t.Errorf("expected 32 bytes, got %d", len(key))
		}
	})

	t.Run("two calls produce different keys", func(t *testing.T) {
		k1, err := NewEncryptionKey()
		if err != nil {
			t.Fatalf("first call failed: %v", err)
		}
		k2, err := NewEncryptionKey()
		if err != nil {
			t.Fatalf("second call failed: %v", err)
		}
		if bytes.Equal(k1, k2) {
			t.Error("expected different keys from two calls")
		}
	})
}
