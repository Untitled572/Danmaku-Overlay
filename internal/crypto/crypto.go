package crypto

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"io"
	"log/slog"
)

const (
	hkdfSalt   = "DanmakuOverlay2024KeyDerivation"
	aesKeySize = 32
	nonceSize  = 12
)

func DeriveKey(secret string) []byte {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(hkdfSalt))
	t := mac.Sum(nil)

	mac2 := hmac.New(sha256.New, t)
	mac2.Write([]byte("encryption-key"))
	return mac2.Sum(nil)
}

func Encrypt(plaintext []byte, key []byte) (string, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return "", fmt.Errorf("new cipher: %w", err)
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", fmt.Errorf("new gcm: %w", err)
	}

	nonce := make([]byte, nonceSize)
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", fmt.Errorf("rand read: %w", err)
	}

	ciphertext := gcm.Seal(nil, nonce, plaintext, nil)
	out := append(nonce, ciphertext...)

	slog.Info("encrypting data")

	return base64.RawStdEncoding.EncodeToString(out), nil
}

func Decrypt(ciphertextB64 string, key []byte) ([]byte, error) {
	data, err := base64.RawStdEncoding.DecodeString(ciphertextB64)
	if err != nil {
		return nil, fmt.Errorf("base64 decode: %w", err)
	}

	if len(data) < nonceSize {
		return nil, fmt.Errorf("ciphertext too short: %w", io.ErrUnexpectedEOF)
	}

	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("new cipher: %w", err)
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("new gcm: %w", err)
	}

	nonce := data[:nonceSize]
	ciphertext := data[nonceSize:]

	plaintext, err := gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return nil, fmt.Errorf("gcm open: %w", err)
	}

	slog.Info("decrypting data")

	return plaintext, nil
}

func NewEncryptionKey() ([]byte, error) {
	key := make([]byte, aesKeySize)
	if _, err := rand.Read(key); err != nil {
		return nil, fmt.Errorf("rand read: %w", err)
	}

	slog.Info("generating new encryption key")

	return key, nil
}
