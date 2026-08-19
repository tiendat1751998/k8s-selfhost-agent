package auth

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/pquerna/otp"
	"github.com/pquerna/otp/totp"
	"golang.org/x/crypto/bcrypt"

	"github.com/datdt/k8sselfhost/internal/pkg/crypto"
)

// TOTPSetupResult holds the secret and QR code URI generated during MFA setup.
type TOTPSetupResult struct {
	Secret    string `json:"secret"`
	QRCodeURI string `json:"qr_uri"`
}

// MFAStatusResult represents the current MFA configuration status for a user.
type MFAStatusResult struct {
	Enabled    bool       `json:"enabled"`
	VerifiedAt *time.Time `json:"verified_at,omitempty"`
}

type totpRateLimiter struct {
	mu       sync.Mutex
	attempts map[string][]time.Time
	maxFails int
	window   time.Duration
}

func newTOTPRateLimiter(maxFails int, window time.Duration) *totpRateLimiter {
	return &totpRateLimiter{
		attempts: make(map[string][]time.Time),
		maxFails: maxFails,
		window:   window,
	}
}

func (rl *totpRateLimiter) allowAttempt(userID string) bool {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	now := time.Now()
	cutoff := now.Add(-rl.window)

	times := rl.attempts[userID]
	var active []time.Time
	for _, t := range times {
		if t.After(cutoff) {
			active = append(active, t)
		}
	}
	rl.attempts[userID] = active

	return len(active) < rl.maxFails
}

func (rl *totpRateLimiter) recordFailure(userID string) {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	rl.attempts[userID] = append(rl.attempts[userID], time.Now())
}

func (rl *totpRateLimiter) recordSuccess(userID string) {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	delete(rl.attempts, userID)
}

// SetupTOTP generates a new TOTP secret and otpauth URL for the user.
func (u *Usecase) SetupTOTP(ctx context.Context, userID string) (*TOTPSetupResult, error) {
	usr, err := u.repo.GetByID(ctx, userID)
	if err != nil || usr == nil {
		return nil, errors.New("user not found")
	}

	key, err := totp.Generate(totp.GenerateOpts{
		Issuer:      "K8sControl",
		AccountName: usr.Email,
		SecretSize:  32,
		Algorithm:   otp.AlgorithmSHA1,
		Period:      30,
		Digits:      otp.DigitsSix,
	})
	if err != nil {
		return nil, fmt.Errorf("generating totp key: %w", err)
	}

	encSecret, err := crypto.Encrypt(key.Secret())
	if err != nil {
		return nil, fmt.Errorf("encrypting totp secret: %w", err)
	}

	if err := u.repo.UpdateMFA(ctx, userID, encSecret, false); err != nil {
		return nil, fmt.Errorf("persisting unverified totp secret: %w", err)
	}

	return &TOTPSetupResult{
		Secret:    key.Secret(),
		QRCodeURI: key.URL(),
	}, nil
}

// VerifyAndEnableTOTP validates initial code to confirm setup, generates recovery codes, and enables MFA.
func (u *Usecase) VerifyAndEnableTOTP(ctx context.Context, userID, code string) ([]string, error) {
	encSecret, err := u.repo.GetMFASecret(ctx, userID)
	if err != nil || encSecret == "" {
		return nil, errors.New("MFA setup not initialized")
	}

	decryptedSecret, err := crypto.Decrypt(encSecret)
	if err != nil {
		return nil, fmt.Errorf("decrypting MFA secret: %w", err)
	}

	valid, err := totp.ValidateCustom(
		code,
		decryptedSecret,
		time.Now().UTC(),
		totp.ValidateOpts{
			Period:    30,
			Skew:      1,
			Digits:    otp.DigitsSix,
			Algorithm: otp.AlgorithmSHA1,
		},
	)
	if err != nil || !valid {
		return nil, errors.New("invalid TOTP code")
	}

	// Generate 10 random 8-character recovery codes
	plainCodes := make([]string, 10)
	hashedCodes := make([]string, 10)
	for i := 0; i < 10; i++ {
		b := make([]byte, 4)
		if _, err := rand.Read(b); err != nil {
			return nil, fmt.Errorf("generating recovery code: %w", err)
		}
		plainCodes[i] = hex.EncodeToString(b)
		hash, err := bcrypt.GenerateFromPassword([]byte(plainCodes[i]), bcrypt.DefaultCost)
		if err != nil {
			return nil, fmt.Errorf("hashing recovery code: %w", err)
		}
		hashedCodes[i] = string(hash)
	}

	jsonBytes, err := json.Marshal(hashedCodes)
	if err != nil {
		return nil, fmt.Errorf("marshaling recovery codes: %w", err)
	}

	encCodes, err := crypto.Encrypt(string(jsonBytes))
	if err != nil {
		return nil, fmt.Errorf("encrypting recovery codes: %w", err)
	}

	if err := u.repo.SetRecoveryCodes(ctx, userID, encCodes); err != nil {
		return nil, fmt.Errorf("saving recovery codes: %w", err)
	}

	if err := u.repo.UpdateMFA(ctx, userID, "", true); err != nil {
		return nil, fmt.Errorf("enabling MFA: %w", err)
	}

	return plainCodes, nil
}

// VerifyTOTP validates a TOTP code during login with rate limiting.
func (u *Usecase) VerifyTOTP(ctx context.Context, userID, code string) error {
	if !u.rateLimiter.allowAttempt(userID) {
		return errors.New("too many failed MFA attempts, please try again in 15 minutes")
	}

	usr, err := u.repo.GetByID(ctx, userID)
	if err != nil || usr == nil {
		u.rateLimiter.recordFailure(userID)
		return errors.New("user not found")
	}

	if !usr.MFAEnabled || usr.MFASecret == "" {
		return errors.New("MFA is not enabled for this user")
	}

	decryptedSecret, err := crypto.Decrypt(usr.MFASecret)
	if err != nil {
		return fmt.Errorf("decrypting MFA secret: %w", err)
	}

	valid, err := totp.ValidateCustom(
		code,
		decryptedSecret,
		time.Now().UTC(),
		totp.ValidateOpts{
			Period:    30,
			Skew:      1,
			Digits:    otp.DigitsSix,
			Algorithm: otp.AlgorithmSHA1,
		},
	)
	if err != nil || !valid {
		u.rateLimiter.recordFailure(userID)
		return errors.New("invalid TOTP code")
	}

	u.rateLimiter.recordSuccess(userID)
	return nil
}

// VerifyRecoveryCode consumes a single recovery code for 2FA login.
func (u *Usecase) VerifyRecoveryCode(ctx context.Context, userID, code string) error {
	usr, err := u.repo.GetByID(ctx, userID)
	if err != nil || usr == nil {
		return errors.New("user not found")
	}

	if !usr.MFAEnabled {
		return errors.New("MFA is not enabled for this user")
	}

	if usr.MFARecoveryCodes == "" {
		return errors.New("no recovery codes available")
	}

	decryptedJSON, err := crypto.Decrypt(usr.MFARecoveryCodes)
	if err != nil {
		return fmt.Errorf("decrypting recovery codes: %w", err)
	}

	var hashedCodes []string
	if err := json.Unmarshal([]byte(decryptedJSON), &hashedCodes); err != nil {
		return fmt.Errorf("unmarshaling recovery codes: %w", err)
	}

	matchIndex := -1
	for i, h := range hashedCodes {
		if err := bcrypt.CompareHashAndPassword([]byte(h), []byte(code)); err == nil {
			matchIndex = i
			break
		}
	}

	if matchIndex == -1 {
		return errors.New("invalid recovery code")
	}

	// Consume the matched recovery code
	hashedCodes = append(hashedCodes[:matchIndex], hashedCodes[matchIndex+1:]...)

	var encCodes string
	if len(hashedCodes) > 0 {
		newJSON, err := json.Marshal(hashedCodes)
		if err != nil {
			return err
		}
		encCodes, err = crypto.Encrypt(string(newJSON))
		if err != nil {
			return err
		}
	}

	if err := u.repo.SetRecoveryCodes(ctx, userID, encCodes); err != nil {
		return fmt.Errorf("updating recovery codes: %w", err)
	}

	return nil
}

// DisableTOTP disables MFA after validating current password and a valid TOTP code.
func (u *Usecase) DisableTOTP(ctx context.Context, userID, password, code string) error {
	usr, err := u.repo.GetByID(ctx, userID)
	if err != nil || usr == nil {
		return errors.New("user not found")
	}

	if err := bcrypt.CompareHashAndPassword([]byte(usr.PasswordHash), []byte(password)); err != nil {
		return errors.New("invalid password")
	}

	if !usr.MFAEnabled || usr.MFASecret == "" {
		return errors.New("MFA is not enabled")
	}

	decryptedSecret, err := crypto.Decrypt(usr.MFASecret)
	if err != nil {
		return fmt.Errorf("decrypting MFA secret: %w", err)
	}

	valid, err := totp.ValidateCustom(
		code,
		decryptedSecret,
		time.Now().UTC(),
		totp.ValidateOpts{
			Period:    30,
			Skew:      1,
			Digits:    otp.DigitsSix,
			Algorithm: otp.AlgorithmSHA1,
		},
	)
	if err != nil || !valid {
		return errors.New("invalid TOTP code")
	}

	if err := u.repo.UpdateMFA(ctx, userID, "", false); err != nil {
		return fmt.Errorf("disabling MFA: %w", err)
	}

	_ = u.repo.SetRecoveryCodes(ctx, userID, "")
	return nil
}

// GetMFAStatus returns whether MFA is enabled and the timestamp when it was verified.
func (u *Usecase) GetMFAStatus(ctx context.Context, userID string) (*MFAStatusResult, error) {
	usr, err := u.repo.GetByID(ctx, userID)
	if err != nil || usr == nil {
		return nil, errors.New("user not found")
	}

	return &MFAStatusResult{
		Enabled:    usr.MFAEnabled,
		VerifiedAt: usr.MFAVerifiedAt,
	}, nil
}
