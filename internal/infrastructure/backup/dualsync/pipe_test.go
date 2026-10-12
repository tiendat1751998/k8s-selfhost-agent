package dualsync_test

import (
	"bytes"
	"crypto/rand"
	"errors"
	"io"
	"strings"
	"testing"

	"github.com/datdt/k8sselfhost/internal/infrastructure/backup/dualsync"
)

func TestDualWriter_Success(t *testing.T) {
	var bufA bytes.Buffer
	var bufB bytes.Buffer

	dw := &dualsync.DualWriter{
		TargetA: &bufA,
		TargetB: &bufB,
	}

	payload := []byte("hello dual-target backup stream testing payload")
	n, err := dw.Write(payload)
	if err != nil {
		t.Fatalf("unexpected error writing to dual writer: %v", err)
	}
	if n != len(payload) {
		t.Fatalf("expected written %d, got %d", len(payload), n)
	}

	if !bytes.Equal(bufA.Bytes(), payload) {
		t.Errorf("bufA content mismatch")
	}
	if !bytes.Equal(bufB.Bytes(), payload) {
		t.Errorf("bufB content mismatch")
	}
}

func TestProcessingPipe_CompressionAndEncryption(t *testing.T) {
	var outputBuf bytes.Buffer

	key := make([]byte, 32)
	_, _ = rand.Read(key)

	cfg := dualsync.PipeConfig{
		CompressionLevel: 3,
		EncryptionKey:    key,
		EnableEncryption: true,
	}

	pipe, closer, err := dualsync.NewProcessingPipe(&outputBuf, cfg)
	if err != nil {
		t.Fatalf("failed to create processing pipe: %v", err)
	}

	rawPayload := bytes.Repeat([]byte("PostgreSQL database table data test string 1234567890\n"), 500)
	n, err := closer.Write(rawPayload)
	if err != nil {
		t.Fatalf("failed to write to pipe: %v", err)
	}
	if n != len(rawPayload) {
		t.Fatalf("expected write %d bytes, got %d", len(rawPayload), n)
	}

	if err := closer.Close(); err != nil {
		t.Fatalf("failed to close pipe: %v", err)
	}

	rawSize, compSize, checksum := pipe.Summary()
	if rawSize != int64(len(rawPayload)) {
		t.Errorf("expected rawSize %d, got %d", len(rawPayload), rawSize)
	}
	if compSize == 0 || compSize >= rawSize {
		t.Errorf("expected compSize (%d) < rawSize (%d)", compSize, rawSize)
	}
	if len(checksum) != 64 {
		t.Errorf("expected 64 char hex SHA-256 checksum, got %s", checksum)
	}

	// Decompress and decrypt stream
	decompStream, err := dualsync.NewDecompressionPipe(&outputBuf, key, true)
	if err != nil {
		t.Fatalf("failed to create decompression pipe: %v", err)
	}
	defer decompStream.Close()

	restoredBytes, err := io.ReadAll(decompStream)
	if err != nil {
		t.Fatalf("failed to read decompressed stream: %v", err)
	}

	if !bytes.Equal(restoredBytes, rawPayload) {
		t.Errorf("restored payload does not match original raw payload")
	}
}

func TestProcessingPipe_NoEncryption(t *testing.T) {
	var outputBuf bytes.Buffer

	cfg := dualsync.PipeConfig{
		CompressionLevel: 1,
		EnableEncryption: false,
	}

	pipe, closer, err := dualsync.NewProcessingPipe(&outputBuf, cfg)
	if err != nil {
		t.Fatalf("failed to create pipe without encryption: %v", err)
	}

	rawPayload := []byte("simple compression without encryption test data")
	_, err = closer.Write(rawPayload)
	if err != nil {
		t.Fatalf("failed write: %v", err)
	}
	_ = closer.Close()

	rawSize, _, _ := pipe.Summary()
	if rawSize != int64(len(rawPayload)) {
		t.Errorf("expected raw size %d, got %d", len(rawPayload), rawSize)
	}

	decompStream, err := dualsync.NewDecompressionPipe(&outputBuf, nil, false)
	if err != nil {
		t.Fatalf("failed decompression: %v", err)
	}
	defer decompStream.Close()

	restored, err := io.ReadAll(decompStream)
	if err != nil {
		t.Fatalf("read decompressed: %v", err)
	}

	if !bytes.Equal(restored, rawPayload) {
		t.Errorf("mismatch in unencrypted compression test")
	}
}

func TestGCM_BitRotDetection(t *testing.T) {
	var outputBuf bytes.Buffer

	key := make([]byte, 32)
	_, _ = rand.Read(key)

	cfg := dualsync.PipeConfig{
		CompressionLevel: 1,
		EncryptionKey:    key,
		EnableEncryption: true,
	}

	_, closer, err := dualsync.NewProcessingPipe(&outputBuf, cfg)
	if err != nil {
		t.Fatalf("failed to create pipe: %v", err)
	}

	rawPayload := []byte("confidential database data payload for tamper detection test")
	_, err = closer.Write(rawPayload)
	if err != nil {
		t.Fatalf("failed to write: %v", err)
	}
	if err := closer.Close(); err != nil {
		t.Fatalf("failed to close: %v", err)
	}

	cipherBytes := outputBuf.Bytes()
	if len(cipherBytes) < 20 {
		t.Fatalf("cipher stream too short: %d bytes", len(cipherBytes))
	}

	// Tamper 1 bit in the ciphertext payload (near the end, but before any padding)
	tampered := make([]byte, len(cipherBytes))
	copy(tampered, cipherBytes)
	tampered[len(tampered)-5] ^= 0x01 // flip 1 bit

	decompStream, err := dualsync.NewDecompressionPipe(bytes.NewReader(tampered), key, true)
	if err != nil {
		// If error returned during creation, verify it indicates corruption
		if !errors.Is(err, dualsync.ErrCorruptedBackupData) && !strings.Contains(err.Error(), "authentication") {
			t.Fatalf("expected ErrCorruptedBackupData on creation, got: %v", err)
		}
		return
	}
	defer decompStream.Close()

	_, readErr := io.ReadAll(decompStream)
	if readErr == nil {
		t.Fatalf("expected decryption error due to tampered ciphertext bit-rot, but got nil error")
	}
	if !errors.Is(readErr, dualsync.ErrCorruptedBackupData) && !strings.Contains(readErr.Error(), "authentication") {
		t.Errorf("expected error to wrap ErrCorruptedBackupData, got: %v", readErr)
	}
}

func TestGCM_MultiChunkIntegrity(t *testing.T) {
	var outputBuf bytes.Buffer

	key := make([]byte, 32)
	_, _ = rand.Read(key)

	cfg := dualsync.PipeConfig{
		CompressionLevel: 1,
		EncryptionKey:    key,
		EnableEncryption: true,
	}

	pipe, closer, err := dualsync.NewProcessingPipe(&outputBuf, cfg)
	if err != nil {
		t.Fatalf("failed to create pipe: %v", err)
	}

	// 256KB to guarantee multiple 64KB GCM chunks
	payload := make([]byte, 256*1024)
	for i := range payload {
		payload[i] = byte(i % 251)
	}

	n, err := closer.Write(payload)
	if err != nil {
		t.Fatalf("write failed: %v", err)
	}
	if n != len(payload) {
		t.Fatalf("expected %d bytes written, got %d", len(payload), n)
	}
	if err := closer.Close(); err != nil {
		t.Fatalf("close failed: %v", err)
	}

	rawSize, compSize, checksum := pipe.Summary()
	if rawSize != int64(len(payload)) {
		t.Errorf("rawSize mismatch: expected %d, got %d", len(payload), rawSize)
	}
	if compSize == 0 {
		t.Errorf("expected non-zero compressed size")
	}
	if len(checksum) != 64 {
		t.Errorf("invalid checksum format: %s", checksum)
	}

	decomp, err := dualsync.NewDecompressionPipe(&outputBuf, key, true)
	if err != nil {
		t.Fatalf("failed decompression init: %v", err)
	}
	defer decomp.Close()

	restored, err := io.ReadAll(decomp)
	if err != nil {
		t.Fatalf("failed to read restored stream: %v", err)
	}
	if !bytes.Equal(restored, payload) {
		t.Errorf("multi-chunk restored payload mismatch: %d != %d", len(restored), len(payload))
	}
}

func TestGCM_TruncatedStreamDetection(t *testing.T) {
	var outputBuf bytes.Buffer

	key := make([]byte, 32)
	_, _ = rand.Read(key)

	cfg := dualsync.PipeConfig{
		CompressionLevel: 1,
		EncryptionKey:    key,
		EnableEncryption: true,
	}

	_, closer, err := dualsync.NewProcessingPipe(&outputBuf, cfg)
	if err != nil {
		t.Fatalf("failed to create pipe: %v", err)
	}

	payload := make([]byte, 128*1024)
	_, _ = closer.Write(payload)
	_ = closer.Close()

	// Truncate ciphertext in half
	truncated := outputBuf.Bytes()[:outputBuf.Len()/2]

	decomp, err := dualsync.NewDecompressionPipe(bytes.NewReader(truncated), key, true)
	if err != nil {
		if !errors.Is(err, dualsync.ErrCorruptedBackupData) && !strings.Contains(err.Error(), "authentication") {
			t.Fatalf("expected corruption error on init, got: %v", err)
		}
		return
	}
	defer decomp.Close()

	_, readErr := io.ReadAll(decomp)
	if readErr == nil {
		t.Fatalf("expected read error on truncated stream, got nil")
	}
}

func TestGCM_InvalidKeyLength(t *testing.T) {
	var buf bytes.Buffer
	cfg := dualsync.PipeConfig{
		CompressionLevel: 1,
		EncryptionKey:    []byte("too-short-key"),
		EnableEncryption: true,
	}

	_, _, err := dualsync.NewProcessingPipe(&buf, cfg)
	if err == nil {
		t.Fatalf("expected error for invalid key length, got nil")
	}

	_, err = dualsync.NewDecompressionPipe(&buf, []byte("short"), true)
	if err == nil {
		t.Fatalf("expected error for invalid key in decompression, got nil")
	}
}


