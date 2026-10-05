package crypto

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"strings"
)

// VersionPrefix is the prefix used to identify encrypted secrets.
const VersionPrefix = "enc:v1:"

// Encryptor defines the interface for encrypting and decrypting sensitive values.
type Encryptor interface {
	Encrypt(plaintext string) (string, error)
	Decrypt(ciphertext string) (string, error)
}

// DefaultEncryptor implements Encryptor using package-level AES-256-GCM.
type DefaultEncryptor struct{}

func (DefaultEncryptor) Encrypt(plaintext string) (string, error) {
	return Encrypt(plaintext)
}

func (DefaultEncryptor) Decrypt(ciphertext string) (string, error) {
	return Decrypt(ciphertext)
}

// ValidateKey checks if the encryption key is present and valid.
func ValidateKey() error {
	_, err := getEncryptionKey()
	return err
}

func getEncryptionKey() ([]byte, error) {
	key := os.Getenv("MASTER_ENCRYPTION_KEY")
	if key == "" {
		key = os.Getenv("ENCRYPTION_KEY")
	}
	if key == "" {
		key = os.Getenv("K8S_ENCRYPTION_KEY")
	}
	if key == "" {
		return nil, fmt.Errorf("MASTER_ENCRYPTION_KEY or ENCRYPTION_KEY is required and must be set in environment")
	}
	// Try hex decode first (64-char hex = 256-bit key)
	if len(key) == 64 {
		decoded, err := hex.DecodeString(key)
		if err == nil {
			return decoded, nil
		}
	}
	// Try base64 decode (32-byte key is 44 chars in base64)
	if decoded, err := base64.StdEncoding.DecodeString(key); err == nil && len(decoded) == 32 {
		return decoded, nil
	}
	// Fallback: raw bytes (32-char string = 256-bit key)
	if len(key) >= 32 {
		return []byte(key[:32]), nil
	}
	return nil, fmt.Errorf("encryption key must be at least 32 characters, 64 hex characters, or 32-byte base64")
}

// Encrypt encrypts plain text using AES-GCM and prepends VersionPrefix
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
	return VersionPrefix + hex.EncodeToString(ciphertext), nil
}

// Decrypt decrypts AES-GCM encrypted hex string with optional enc:v1: prefix.
// If the ciphertext has the enc:v1: prefix, it strips it and decrypts.
// If it does not have the prefix and is legacy plaintext, it is returned as-is
// for backward compatibility.
func Decrypt(ciphertextStr string) (string, error) {
	if ciphertextStr == "" {
		return "", nil
	}

	if strings.HasPrefix(ciphertextStr, VersionPrefix) {
		rawHex := strings.TrimPrefix(ciphertextStr, VersionPrefix)
		return decryptAESGCM(rawHex)
	}

	// Legacy row backward compatibility:
	// If it contains YAML/JSON indicators or newlines, it's plaintext.
	if strings.Contains(ciphertextStr, "apiVersion:") ||
		strings.Contains(ciphertextStr, "\"apiVersion\"") ||
		strings.Contains(ciphertextStr, "\n") ||
		strings.HasPrefix(ciphertextStr, "---") ||
		strings.HasPrefix(ciphertextStr, "{") {
		return ciphertextStr, nil
	}

	// Try legacy hex decrypt (encrypted before version prefix was added)
	decrypted, err := decryptAESGCM(ciphertextStr)
	if err == nil {
		return decrypted, nil
	}

	return "", err
}

func decryptAESGCM(hexString string) (string, error) {
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
