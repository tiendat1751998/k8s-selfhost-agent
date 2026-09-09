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
		key = os.Getenv("K8S_ENCRYPTION_KEY")
	}
	if key == "" {
		return nil, fmt.Errorf("ENCRYPTION_KEY is required and must be set in environment")
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

	keysToTry := [][]byte{}
	if primaryKey, err := getEncryptionKey(); err == nil {
		keysToTry = append(keysToTry, primaryKey)
	}
	// Fallback dev keys for cross-session compatibility
	for _, fallbackStr := range []string{
		"0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef",
		"0123456789abcdef0123456789abcdef",
		"12345678901234567890123456789012",
	} {
		if len(fallbackStr) == 64 {
			if dec, err := hex.DecodeString(fallbackStr); err == nil {
				keysToTry = append(keysToTry, dec)
			}
		} else if len(fallbackStr) >= 32 {
			keysToTry = append(keysToTry, []byte(fallbackStr[:32]))
		}
	}

	var lastErr error
	for _, k := range keysToTry {
		block, err := aes.NewCipher(k)
		if err != nil {
			lastErr = err
			continue
		}

		aesgcm, err := cipher.NewGCM(block)
		if err != nil {
			lastErr = err
			continue
		}

		nonceSize := aesgcm.NonceSize()
		if len(ciphertext) < nonceSize {
			return "", fmt.Errorf("ciphertext too short")
		}

		nonce, cipherData := ciphertext[:nonceSize], ciphertext[nonceSize:]
		plaintext, err := aesgcm.Open(nil, nonce, cipherData, nil)
		if err == nil {
			return string(plaintext), nil
		}
		lastErr = err
	}

	return "", lastErr
}
