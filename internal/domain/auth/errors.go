package auth

import "errors"

var (
	// ErrInvalidToken indicates the refresh token is missing, malformed, or invalid.
	ErrInvalidToken = errors.New("invalid refresh token")

	// ErrTokenRevoked indicates the refresh token was already revoked or consumed (reuse attempt).
	ErrTokenRevoked = errors.New("refresh token has been revoked")

	// ErrTokenExpired indicates the refresh token validity period has passed.
	ErrTokenExpired = errors.New("refresh token has expired")

	// ErrTokenNotFound indicates the refresh token does not exist in persistence.
	ErrTokenNotFound = errors.New("refresh token not found")

	// ErrUserNotFound indicates the user associated with the token does not exist.
	ErrUserNotFound = errors.New("user not found")
)
