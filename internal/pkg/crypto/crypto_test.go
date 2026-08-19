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

func TestGetEncryptionKey_HexDecode_64Chars(t *testing.T) {
	origKey := os.Getenv("ENCRYPTION_KEY")
	defer os.Setenv("ENCRYPTION_KEY", origKey)

	// 64-char hex string (32 bytes = 256 bits)
	hexKey := "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"
	os.Setenv("ENCRYPTION_KEY", hexKey)

	key, err := getEncryptionKey()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(key) != 32 {
		t.Fatalf("expected 32 bytes key, got %d", len(key))
	}

	plaintext := "crypto test with 64-char hex key"
	ciphertext, err := Encrypt(plaintext)
	if err != nil {
		t.Fatalf("failed to encrypt: %v", err)
	}
	decrypted, err := Decrypt(ciphertext)
	if err != nil {
		t.Fatalf("failed to decrypt: %v", err)
	}
	if decrypted != plaintext {
		t.Errorf("expected %q, got %q", plaintext, decrypted)
	}
}

func TestGetEncryptionKey_RawBytes_32Chars(t *testing.T) {
	origKey := os.Getenv("ENCRYPTION_KEY")
	defer os.Setenv("ENCRYPTION_KEY", origKey)

	// 32-char ASCII string
	rawKey := "12345678901234567890123456789012"
	os.Setenv("ENCRYPTION_KEY", rawKey)

	key, err := getEncryptionKey()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(key) != 32 {
		t.Fatalf("expected 32 bytes key, got %d", len(key))
	}
	if string(key) != rawKey {
		t.Errorf("expected key %q, got %q", rawKey, string(key))
	}

	// 40-char ASCII string (should take first 32 chars)
	os.Setenv("ENCRYPTION_KEY", rawKey+"extrabytes")
	keyLong, err := getEncryptionKey()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(keyLong) != 32 {
		t.Fatalf("expected 32 bytes key, got %d", len(keyLong))
	}
	if string(keyLong) != rawKey {
		t.Errorf("expected sliced key %q, got %q", rawKey, string(keyLong))
	}
}

func TestGetEncryptionKey_TooShort_Error(t *testing.T) {
	origKey := os.Getenv("ENCRYPTION_KEY")
	defer os.Setenv("ENCRYPTION_KEY", origKey)

	cases := []struct {
		name string
		key  string
	}{
		{"empty", ""},
		{"too short ascii", "short-key-123"},
		{"31 chars", "1234567890123456789012345678901"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			os.Setenv("ENCRYPTION_KEY", tc.key)
			_, err := getEncryptionKey()
			if err == nil {
				t.Errorf("expected error for key %q, got nil", tc.key)
			}
		})
	}
}
