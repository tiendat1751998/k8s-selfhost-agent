package storage

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"sort"
	"strings"
	"time"

	"github.com/datdt/k8sselfhost/internal/domain/backup"
	"github.com/datdt/k8sselfhost/internal/pkg/errors"
)

const (
	s3DefaultRegion = "us-east-1"
	s3Service       = "s3"
)

type S3Storage struct {
	endpoint        string
	bucket          string
	region          string
	accessKeyID     string
	secretAccessKey string
	useSSL          bool
	httpClient      *http.Client
}

func NewS3Storage(storage *backup.BackupStorage) *S3Storage {
	useSSL := strings.HasPrefix(storage.Endpoint, "https://")
	cleanEndpoint := strings.TrimPrefix(strings.TrimPrefix(storage.Endpoint, "https://"), "http://")
	region := storage.Credentials["region"]
	if region == "" {
		region = s3DefaultRegion
	}
	return &S3Storage{
		endpoint:        cleanEndpoint,
		bucket:          storage.Bucket,
		region:          region,
		accessKeyID:     storage.Credentials["access_key"],
		secretAccessKey: storage.Credentials["secret_key"],
		useSSL:          useSSL,
		httpClient:      &http.Client{Timeout: 30 * time.Minute},
	}
}

// signRequest applies AWS Signature Version 4 to the request.
// Minimal implementation using only stdlib — sufficient for S3-compatible APIs (MinIO, AWS S3).
func (s *S3Storage) signRequest(req *http.Request, payloadHash string) {
	if s.accessKeyID == "" || s.secretAccessKey == "" {
		return // Skip signing if no credentials (public bucket)
	}

	now := time.Now().UTC()
	dateStamp := now.Format("20060102")
	amzDate := now.Format("20060102T150405Z")

	req.Header.Set("x-amz-date", amzDate)
	req.Header.Set("x-amz-content-sha256", payloadHash)
	if req.Host == "" {
		req.Host = req.URL.Host
	}

	// Canonical headers
	signedHeaders := []string{"host", "x-amz-content-sha256", "x-amz-date"}
	for k := range req.Header {
		lk := strings.ToLower(k)
		if strings.HasPrefix(lk, "x-amz-meta-") {
			signedHeaders = append(signedHeaders, lk)
		}
	}
	sort.Strings(signedHeaders)

	var canonicalHeaders strings.Builder
	for _, h := range signedHeaders {
		if h == "host" {
			canonicalHeaders.WriteString("host:" + req.Host + "\n")
		} else {
			canonicalHeaders.WriteString(h + ":" + req.Header.Get(http.CanonicalHeaderKey(h)) + "\n")
		}
	}

	canonicalRequest := strings.Join([]string{
		req.Method,
		req.URL.Path,
		req.URL.RawQuery,
		canonicalHeaders.String(),
		strings.Join(signedHeaders, ";"),
		payloadHash,
	}, "\n")

	credentialScope := dateStamp + "/" + s.region + "/" + s3Service + "/aws4_request"
	stringToSign := "AWS4-HMAC-SHA256\n" + amzDate + "\n" + credentialScope + "\n" + hashSHA256([]byte(canonicalRequest))

	signingKey := deriveSigningKey(s.secretAccessKey, dateStamp, s.region, s3Service)
	signature := hex.EncodeToString(hmacSHA256(signingKey, []byte(stringToSign)))

	authHeader := fmt.Sprintf("AWS4-HMAC-SHA256 Credential=%s/%s, SignedHeaders=%s, Signature=%s",
		s.accessKeyID, credentialScope, strings.Join(signedHeaders, ";"), signature)
	req.Header.Set("Authorization", authHeader)
}

func hmacSHA256(key, data []byte) []byte {
	h := hmac.New(sha256.New, key)
	h.Write(data)
	return h.Sum(nil)
}

func hashSHA256(data []byte) string {
	h := sha256.Sum256(data)
	return hex.EncodeToString(h[:])
}

func deriveSigningKey(secret, dateStamp, region, service string) []byte {
	kDate := hmacSHA256([]byte("AWS4"+secret), []byte(dateStamp))
	kRegion := hmacSHA256(kDate, []byte(region))
	kService := hmacSHA256(kRegion, []byte(service))
	return hmacSHA256(kService, []byte("aws4_request"))
}

func (s *S3Storage) Type() string {
	return "s3"
}

func (s *S3Storage) UploadStream(ctx context.Context, relPath string, reader io.Reader, size int64, metadata map[string]string) (string, error) {
	scheme := "http"
	if s.useSSL {
		scheme = "https"
	}
	url := fmt.Sprintf("%s://%s/%s/%s", scheme, s.endpoint, s.bucket, relPath)

	req, err := http.NewRequestWithContext(ctx, http.MethodPut, url, reader)
	if err != nil {
		return "", errors.Wrap(err, "creating S3 upload request")
	}

	req.Header.Set("Content-Type", "application/zstd")
	for k, v := range metadata {
		req.Header.Set(fmt.Sprintf("x-amz-meta-%s", k), v)
	}

	// For S3 compatible endpoints (MinIO/S3), sign and execute
	s.signRequest(req, "UNSIGNED-PAYLOAD")
	resp, err := s.httpClient.Do(req)
	if err != nil {
		return "", errors.Wrap(err, "uploading stream to S3 compatible storage")
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		body, _ := io.ReadAll(resp.Body)
		return "", errors.NewInternal(fmt.Sprintf("S3 upload failed with status %d: %s", resp.StatusCode, string(body)), errors.ErrInternal)
	}


	return fmt.Sprintf("s3://%s/%s", s.bucket, relPath), nil
}

func (s *S3Storage) DownloadStream(ctx context.Context, relPath string) (io.ReadCloser, error) {
	scheme := "http"
	if s.useSSL {
		scheme = "https"
	}
	url := fmt.Sprintf("%s://%s/%s/%s", scheme, s.endpoint, s.bucket, relPath)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, errors.Wrap(err, "creating S3 download request")
	}

	s.signRequest(req, hashSHA256(nil))

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return nil, errors.Wrap(err, "downloading stream from S3 compatible storage")
	}

	if resp.StatusCode >= 400 {
		resp.Body.Close()
		return nil, errors.NewNotFound("s3_object", fmt.Sprintf("failed to get %s: HTTP %d", relPath, resp.StatusCode))
	}

	return resp.Body, nil
}

func (s *S3Storage) Delete(ctx context.Context, relPath string) error {
	scheme := "http"
	if s.useSSL {
		scheme = "https"
	}
	url := fmt.Sprintf("%s://%s/%s/%s", scheme, s.endpoint, s.bucket, relPath)

	req, err := http.NewRequestWithContext(ctx, http.MethodDelete, url, nil)
	if err != nil {
		return errors.Wrap(err, "creating S3 delete request")
	}

	s.signRequest(req, hashSHA256(nil))

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return errors.Wrap(err, "deleting S3 object")
	}
	defer resp.Body.Close()
	return nil
}

func (s *S3Storage) Exists(ctx context.Context, relPath string) (bool, error) {
	scheme := "http"
	if s.useSSL {
		scheme = "https"
	}
	url := fmt.Sprintf("%s://%s/%s/%s", scheme, s.endpoint, s.bucket, relPath)

	req, err := http.NewRequestWithContext(ctx, http.MethodHead, url, nil)
	if err != nil {
		return false, errors.Wrap(err, "creating S3 HEAD request")
	}

	s.signRequest(req, hashSHA256(nil))

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return false, errors.Wrap(err, "checking S3 object existence")
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusOK {
		return true, nil
	}
	if resp.StatusCode == http.StatusNotFound {
		return false, nil
	}
	return false, fmt.Errorf("unexpected status: %d", resp.StatusCode)
}
