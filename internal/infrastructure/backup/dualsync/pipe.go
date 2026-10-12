package dualsync

import (
	"crypto/sha256"
	"encoding/hex"
	stderrors "errors"
	"hash"
	"io"

	"github.com/klauspost/compress/zstd"
	"github.com/datdt/k8sselfhost/internal/pkg/errors"
)

// ErrCorruptedBackupData is returned when AEAD authentication verification fails.
var ErrCorruptedBackupData = stderrors.New("backup data integrity check failed: GCM authentication tag mismatch")

// PipeConfig holds options for compression and encryption pipeline
type PipeConfig struct {
	CompressionLevel int    // 1 (fastest) to 4 (better compression)
	EncryptionKey    []byte // 32 bytes for AES-256
	EnableEncryption bool
}

// ProcessingPipe wraps a writer stream with transparent compression, encryption, and SHA256 hashing
type ProcessingPipe struct {
	underlying io.Writer
	zstdWriter *zstd.Encoder
	gcmWriter  *GCMStreamWriter
	hasher     hash.Hash
	rawBytes   int64
	compBytes  int64
}

// ByteCountingWriter tracks written bytes
type ByteCountingWriter struct {
	W     io.Writer
	Count *int64
}

func (b *ByteCountingWriter) Write(p []byte) (int, error) {
	n, err := b.W.Write(p)
	if b.Count != nil {
		*b.Count += int64(n)
	}
	return n, err
}

// NewProcessingPipe creates a streaming compression, encryption, and hashing pipeline
func NewProcessingPipe(dst io.Writer, cfg PipeConfig) (*ProcessingPipe, io.WriteCloser, error) {
	hasher := sha256.New()
	p := &ProcessingPipe{
		underlying: dst,
		hasher:     hasher,
	}

	// 1. Destination with byte counting
	destWithCount := &ByteCountingWriter{W: dst, Count: &p.compBytes}

	// 2. Encryption layer (AES-256-GCM chunked AEAD)
	var finalSink io.Writer = destWithCount
	if cfg.EnableEncryption && len(cfg.EncryptionKey) > 0 {
		gw, err := NewGCMStreamWriter(destWithCount, cfg.EncryptionKey)
		if err != nil {
			return nil, nil, errors.Wrap(err, "creating GCM stream writer")
		}
		p.gcmWriter = gw
		finalSink = gw
	}

	// 3. Compression layer (zstd)
	level := zstd.SpeedDefault
	if cfg.CompressionLevel == 1 {
		level = zstd.SpeedFastest
	} else if cfg.CompressionLevel >= 3 {
		level = zstd.SpeedBetterCompression
	}

	zstdEnc, err := zstd.NewWriter(finalSink, zstd.WithEncoderLevel(level))
	if err != nil {
		return nil, nil, errors.Wrap(err, "creating zstd encoder")
	}
	p.zstdWriter = zstdEnc

	// 4. Ingress stream that counts raw bytes and updates SHA-256 hash simultaneously
	ingress := io.MultiWriter(
		&ByteCountingWriter{W: zstdEnc, Count: &p.rawBytes},
		hasher,
	)

	return p, &pipeCloser{p: p, ingress: ingress}, nil
}

type pipeCloser struct {
	p       *ProcessingPipe
	ingress io.Writer
}

func (pc *pipeCloser) Write(p []byte) (int, error) {
	return pc.ingress.Write(p)
}

func (pc *pipeCloser) Close() error {
	if pc.p.zstdWriter != nil {
		if err := pc.p.zstdWriter.Close(); err != nil {
			return err
		}
	}
	if pc.p.gcmWriter != nil {
		if err := pc.p.gcmWriter.Close(); err != nil {
			return err
		}
	}
	return nil
}

// Summary returns the processed metrics
func (p *ProcessingPipe) Summary() (rawSize int64, compressedSize int64, sha256Checksum string) {
	checksum := hex.EncodeToString(p.hasher.Sum(nil))
	return p.rawBytes, p.compBytes, checksum
}

// NewDecompressionPipe decodes and decrypts a backup stream for restore
func NewDecompressionPipe(src io.Reader, encryptionKey []byte, enableEncryption bool) (io.ReadCloser, error) {
	var inputSource io.Reader = src

	if enableEncryption && len(encryptionKey) > 0 {
		gr, err := NewGCMStreamReader(src, encryptionKey)
		if err != nil {
			return nil, errors.Wrap(err, "creating GCM stream reader for decompression")
		}
		inputSource = gr
	}

	zstdDec, err := zstd.NewReader(inputSource)
	if err != nil {
		return nil, errors.Wrap(err, "creating zstd decoder")
	}

	return zstdDec.IOReadCloser(), nil
}

// DualWriter splits writes to two independent destinations and captures any errors
type DualWriter struct {
	TargetA io.Writer
	TargetB io.Writer
}

func (d *DualWriter) Write(p []byte) (n int, err error) {
	if d.TargetA != nil {
		if _, errA := d.TargetA.Write(p); errA != nil {
			return 0, errors.Wrap(errA, "writing to primary target")
		}
	}
	if d.TargetB != nil {
		if _, errB := d.TargetB.Write(p); errB != nil {
			return 0, errors.Wrap(errB, "writing to secondary target")
		}
	}
	return len(p), nil
}
