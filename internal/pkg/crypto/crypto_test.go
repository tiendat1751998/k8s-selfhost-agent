package crypto

import (
	"os"
	"testing"
)

func TestValidateKey(t *testing.T) {
	origKey := os.Getenv("ENCRYPTION_KEY")
	defer os.Setenv("ENCRYPTION_KEY", origKey)

	// Empty key
	os.Setenv("ENCRYPTION_KEY", "")
	if err := ValidateKey(); err == nil {
		t.Error("expected error for empty ENCRYPTION_KEY, got nil")
	}

	// Too short key
	os.Setenv("ENCRYPTION_KEY", "12345")
	if err := ValidateKey(); err == nil {
		t.Error("expected error for short ENCRYPTION_KEY, got nil")
	}

	// Valid key
	os.Setenv("ENCRYPTION_KEY", "12345678901234567890123456789012")
	if err := ValidateKey(); err != nil {
		t.Errorf("expected no error for valid key, got %v", err)
	}
}

func TestEncryptDecrypt(t *testing.T) {
	origKey := os.Getenv("ENCRYPTION_KEY")
	defer os.Setenv("ENCRYPTION_KEY", origKey)
	os.Setenv("ENCRYPTION_KEY", "12345678901234567890123456789012")

	plaintext := "hello world"
	ciphertext, err := Encrypt(plaintext)
	if err != nil {
		t.Fatalf("failed to encrypt: %v", err)
	}

	decrypted, err := Decrypt(ciphertext)
	if err != nil {
		t.Fatalf("failed to decrypt: %v", err)
	}

	if decrypted != plaintext {
		t.Errorf("expected decrypted text %q, got %q", plaintext, decrypted)
	}
}

func TestEncryptDecryptEmpty(t *testing.T) {
	origKey := os.Getenv("ENCRYPTION_KEY")
	defer os.Setenv("ENCRYPTION_KEY", origKey)
	os.Setenv("ENCRYPTION_KEY", "12345678901234567890123456789012")

	ciphertext, err := Encrypt("")
	if err != nil {
		t.Fatalf("failed to encrypt empty string: %v", err)
	}
	if ciphertext != "" {
		t.Errorf("expected empty ciphertext, got %q", ciphertext)
	}

	decrypted, err := Decrypt("")
	if err != nil {
		t.Fatalf("failed to decrypt empty string: %v", err)
	}
	if decrypted != "" {
		t.Errorf("expected empty decrypted text, got %q", decrypted)
	}
}

func TestDecryptInvalid(t *testing.T) {
	origKey := os.Getenv("ENCRYPTION_KEY")
	defer os.Setenv("ENCRYPTION_KEY", origKey)
	os.Setenv("ENCRYPTION_KEY", "12345678901234567890123456789012")

	// Invalid hex string
	_, err := Decrypt("invalidhexstring")
	if err == nil {
		t.Error("expected error for invalid hex string, got nil")
	}

	// Ciphertext too short
	_, err = Decrypt("aabbcc")
	if err == nil {
		t.Error("expected error for too short ciphertext, got nil")
	}
}
