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

// ValidateKey checks if the ENCRYPTION_KEY is present and is at least 32 characters long.
func ValidateKey() error {
	key := os.Getenv("ENCRYPTION_KEY")
	if key == "" {
		return fmt.Errorf("ENCRYPTION_KEY environment variable is not set")
	}
	if len(key) < 32 {
		return fmt.Errorf("ENCRYPTION_KEY must be at least 32 characters long")
	}
	return nil
}

func getEncryptionKey() []byte {
	key := os.Getenv("ENCRYPTION_KEY")
	if key == "" {
		panic("ENCRYPTION_KEY environment variable is not set")
	}
	if len(key) < 32 {
		panic("ENCRYPTION_KEY must be at least 32 characters long")
	}
	return []byte(key[:32])
}

// Encrypt encrypts plain text using AES-GCM
func Encrypt(plaintext string) (string, error) {
	if plaintext == "" {
		return "", nil
	}
	
	block, err := aes.NewCipher(getEncryptionKey())
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

	block, err := aes.NewCipher(getEncryptionKey())
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
