package crypto

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"io"
	"os"
)

// ValidateKey checks if the ENCRYPTION_KEY is present and valid.
func ValidateKey() error {
	_, err := getEncryptionKey()
	return err
}

func getEncryptionKey() ([]byte, error) {
	key := os.Getenv("ENCRYPTION_KEY")
	if key == "" {
		return nil, fmt.Errorf("ENCRYPTION_KEY not set")
	}
	// Try hex decode first (64-char hex = 256-bit key)
	if len(key) == 64 {
		decoded, err := hex.DecodeString(key)
		if err == nil {
			return decoded, nil
		}
	}
	// Fallback: raw bytes (32-char string = 256-bit key)
	if len(key) >= 32 {
		return []byte(key[:32]), nil
	}
	return nil, fmt.Errorf("ENCRYPTION_KEY must be at least 32 characters or 64 hex characters")
}

// Encrypt encrypts plain text using AES-GCM
func Encrypt(plaintext string) (string, error) {
	if plaintext == "" {
		return "", nil
	}
	
	key, err := getEncryptionKey()
	if err != nil {
		return "", err
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return "", err
	}

	aesgcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}

	nonce := make([]byte, aesgcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", err
	}

	ciphertext := aesgcm.Seal(nonce, nonce, []byte(plaintext), nil)
	return hex.EncodeToString(ciphertext), nil
}

// Decrypt decrypts AES-GCM encrypted hex string
func Decrypt(hexString string) (string, error) {
	if hexString == "" {
		return "", nil
	}
	
	ciphertext, err := hex.DecodeString(hexString)
	if err != nil {
		return "", err
	}

	key, err := getEncryptionKey()
	if err != nil {
		return "", err
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return "", err
	}

	aesgcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}

	nonceSize := aesgcm.NonceSize()
	if len(ciphertext) < nonceSize {
		return "", fmt.Errorf("ciphertext too short")
	}

	nonce, ciphertext := ciphertext[:nonceSize], ciphertext[nonceSize:]
	plaintext, err := aesgcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return "", err
	}

	return string(plaintext), nil
}
