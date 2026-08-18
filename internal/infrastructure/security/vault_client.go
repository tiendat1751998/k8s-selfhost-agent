package security

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/datdt/k8sselfhost/internal/pkg/errors"
)

type VaultConfig struct {
	Address   string
	Token     string
	AppRoleID string
	SecretID  string
	Timeout   time.Duration
}

type VaultClient struct {
	config     VaultConfig
	httpClient *http.Client
	token      string
}

func NewVaultClient(cfg VaultConfig) *VaultClient {
	if cfg.Timeout <= 0 {
		cfg.Timeout = 10 * time.Second
	}
	return &VaultClient{
		config:     cfg,
		httpClient: &http.Client{Timeout: cfg.Timeout},
		token:      cfg.Token,
	}
}

func (v *VaultClient) AuthenticateAppRole(ctx context.Context) error {
	if v.config.AppRoleID == "" || v.config.SecretID == "" {
		return nil
	}

	url := fmt.Sprintf("%s/v1/auth/approle/login", v.config.Address)
	payload := map[string]string{
		"role_id":   v.config.AppRoleID,
		"secret_id": v.config.SecretID,
	}

	data, err := json.Marshal(payload)
	if err != nil {
		return errors.Wrap(err, "marshaling approle payload")
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(data))
	if err != nil {
		return errors.Wrap(err, "creating approle login request")
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := v.httpClient.Do(req)
	if err != nil {
		return errors.Wrap(err, "executing approle login")
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		body, _ := io.ReadAll(resp.Body)
		return errors.NewInternal(fmt.Sprintf("vault approle login failed with status %d: %s", resp.StatusCode, string(body)), errors.ErrInternal)
	}

	var result struct {
		Auth struct {
			ClientToken string `json:"client_token"`
		} `json:"auth"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return errors.Wrap(err, "decoding vault token")
	}

	v.token = result.Auth.ClientToken
	return nil
}

func (v *VaultClient) ReadSecret(ctx context.Context, mountPath, secretPath string) (map[string]interface{}, error) {
	url := fmt.Sprintf("%s/v1/%s/data/%s", v.config.Address, mountPath, secretPath)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, errors.Wrap(err, "creating read secret request")
	}
	req.Header.Set("X-Vault-Token", v.token)

	resp, err := v.httpClient.Do(req)
	if err != nil {
		return nil, errors.Wrap(err, "executing vault read secret")
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return nil, errors.NewNotFound("vault_secret", secretPath)
	}
	if resp.StatusCode >= 400 {
		body, _ := io.ReadAll(resp.Body)
		return nil, errors.NewInternal(fmt.Sprintf("vault read failed: %s", string(body)), errors.ErrInternal)
	}

	var result struct {
		Data struct {
			Data map[string]interface{} `json:"data"`
		} `json:"data"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, errors.Wrap(err, "decoding vault secret response")
	}

	return result.Data.Data, nil
}

func (v *VaultClient) WriteSecret(ctx context.Context, mountPath, secretPath string, secretData map[string]interface{}) error {
	url := fmt.Sprintf("%s/v1/%s/data/%s", v.config.Address, mountPath, secretPath)

	payload := map[string]interface{}{
		"data": secretData,
	}
	data, _ := json.Marshal(payload)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(data))
	if err != nil {
		return errors.Wrap(err, "creating write secret request")
	}
	req.Header.Set("X-Vault-Token", v.token)
	req.Header.Set("Content-Type", "application/json")

	resp, err := v.httpClient.Do(req)
	if err != nil {
		return errors.Wrap(err, "writing secret to vault")
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		return errors.NewInternal(fmt.Sprintf("vault write failed with status %d", resp.StatusCode), errors.ErrInternal)
	}
	return nil
}
