package main

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"golang.org/x/crypto/bcrypt"
)

func TestGetPassword(t *testing.T) {
	t.Run("empty args and env returns error", func(t *testing.T) {
		_, err := getPassword([]string{"gen_hash"}, func(k string) string { return "" })
		assert.ErrorIs(t, err, ErrEmptyPassword)
	})

	t.Run("reads from args if provided", func(t *testing.T) {
		pass, err := getPassword([]string{"gen_hash", "my-secret-pass"}, func(k string) string { return "" })
		assert.NoError(t, err)
		assert.Equal(t, "my-secret-pass", pass)
	})

	t.Run("reads from env if arg not provided", func(t *testing.T) {
		pass, err := getPassword([]string{"gen_hash"}, func(k string) string {
			if k == "ADMIN_PASSWORD" {
				return "env-secret-pass"
			}
			return ""
		})
		assert.NoError(t, err)
		assert.Equal(t, "env-secret-pass", pass)
	})
}

func TestHashPassword(t *testing.T) {
	t.Run("empty password fails", func(t *testing.T) {
		_, err := hashPassword("")
		assert.ErrorIs(t, err, ErrEmptyPassword)
	})

	t.Run("valid password generates valid bcrypt hash", func(t *testing.T) {
		hash, err := hashPassword("supersecret")
		assert.NoError(t, err)
		assert.NotEmpty(t, hash)

		// Verify with bcrypt
		err = bcrypt.CompareHashAndPassword([]byte(hash), []byte("supersecret"))
		assert.NoError(t, err)
	})
}
