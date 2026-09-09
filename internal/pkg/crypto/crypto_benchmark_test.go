package crypto

import (
	"os"
	"testing"
)

func init() {
	os.Setenv("ENCRYPTION_KEY", "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef")
}

func BenchmarkEncrypt_Short(b *testing.B) {
	plain := "my-secret-token-123"
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		res, err := Encrypt(plain)
		if err != nil {
			b.Fatal(err)
		}
		_ = res
	}
}

func BenchmarkEncrypt_MediumPayload(b *testing.B) {
	plain := "{\"access_token\": \"eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9...\", \"refresh_token\": \"xyz789\", \"tenant_id\": \"ten-123\", \"user_id\": \"usr-456\"}"
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		res, err := Encrypt(plain)
		if err != nil {
			b.Fatal(err)
		}
		_ = res
	}
}

func BenchmarkDecrypt_Short(b *testing.B) {
	plain := "my-secret-token-123"
	encrypted, err := Encrypt(plain)
	if err != nil {
		b.Fatal(err)
	}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		res, err := Decrypt(encrypted)
		if err != nil {
			b.Fatal(err)
		}
		_ = res
	}
}

func BenchmarkEncrypt_Parallel(b *testing.B) {
	plain := "my-secret-token-123"
	b.ReportAllocs()
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			res, err := Encrypt(plain)
			if err != nil {
				b.Fatal(err)
			}
			_ = res
		}
	})
}
