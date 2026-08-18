package dualsync_test

import (
	"bytes"
	"crypto/rand"
	"io"
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
