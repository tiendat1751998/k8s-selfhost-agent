package dualsync

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/binary"
	stderrors "errors"
	"fmt"
	"io"

	"github.com/datdt/k8sselfhost/internal/pkg/errors"
)

// GCMChunkSize is 64KB for chunked AEAD streaming
const (
	GCMChunkSize = 64 * 1024
	GCMNonceSize = 12
	GCMTagSize   = 16
)

// GCMStreamWriter wraps an io.Writer with chunked AES-256-GCM encryption.
type GCMStreamWriter struct {
	w         io.Writer
	aead      cipher.AEAD
	baseNonce []byte
	chunkSeq  uint64
	buf       []byte
	closed    bool
}

// NewGCMStreamWriter initializes a GCM stream writer.
// It generates a random 12-byte base nonce and writes it as the stream header.
func NewGCMStreamWriter(w io.Writer, key []byte) (*GCMStreamWriter, error) {
	if len(key) != 32 {
		return nil, errors.NewValidation("encryption_key", "AES-256 key must be exactly 32 bytes")
	}

	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, errors.Wrap(err, "creating AES cipher")
	}

	aead, err := cipher.NewGCM(block)
	if err != nil {
		return nil, errors.Wrap(err, "creating GCM mode")
	}

	baseNonce := make([]byte, GCMNonceSize)
	if _, err := io.ReadFull(rand.Reader, baseNonce); err != nil {
		return nil, errors.Wrap(err, "generating random base nonce")
	}

	// Write 12-byte base nonce header
	if _, err := w.Write(baseNonce); err != nil {
		return nil, errors.Wrap(err, "writing base nonce header")
	}

	return &GCMStreamWriter{
		w:         w,
		aead:      aead,
		baseNonce: baseNonce,
		buf:       make([]byte, 0, GCMChunkSize),
	}, nil
}

func deriveNonce(baseNonce []byte, seq uint64) []byte {
	nonce := make([]byte, GCMNonceSize)
	copy(nonce, baseNonce)
	binary.BigEndian.PutUint64(nonce[4:], binary.BigEndian.Uint64(nonce[4:])^seq)
	return nonce
}

func (gw *GCMStreamWriter) writeChunk(plaintext []byte, isLast byte) error {
	nonce := deriveNonce(gw.baseNonce, gw.chunkSeq)
	ad := make([]byte, 9)
	binary.BigEndian.PutUint64(ad[:8], gw.chunkSeq)
	ad[8] = isLast

	ciphertext := gw.aead.Seal(nil, nonce, plaintext, ad)
	chunkLen := uint32(len(ciphertext))

	var hdr [5]byte
	binary.BigEndian.PutUint32(hdr[:4], chunkLen)
	hdr[4] = isLast

	if _, err := gw.w.Write(hdr[:]); err != nil {
		return errors.Wrap(err, "writing chunk header")
	}
	if _, err := gw.w.Write(ciphertext); err != nil {
		return errors.Wrap(err, "writing chunk ciphertext")
	}

	gw.chunkSeq++
	return nil
}

func (gw *GCMStreamWriter) Write(p []byte) (int, error) {
	if gw.closed {
		return 0, errors.NewValidation("stream", "write to closed GCMStreamWriter")
	}

	total := len(p)
	for len(p) > 0 {
		space := GCMChunkSize - len(gw.buf)
		if len(p) < space {
			gw.buf = append(gw.buf, p...)
			break
		}
		gw.buf = append(gw.buf, p[:space]...)
		p = p[space:]
		if len(p) > 0 {
			if err := gw.writeChunk(gw.buf, 0); err != nil {
				return 0, err
			}
			gw.buf = gw.buf[:0]
		}
	}
	return total, nil
}

func (gw *GCMStreamWriter) Close() error {
	if gw.closed {
		return nil
	}
	gw.closed = true

	if err := gw.writeChunk(gw.buf, 1); err != nil {
		return err
	}
	gw.buf = nil
	return nil
}

// GCMStreamReader reads and verifies chunked AES-256-GCM ciphertext.
type GCMStreamReader struct {
	r         io.Reader
	aead      cipher.AEAD
	baseNonce []byte
	chunkSeq  uint64
	buf       []byte
	eof       bool
}

// NewGCMStreamReader initializes a GCM stream reader from an encrypted source.
func NewGCMStreamReader(r io.Reader, key []byte) (*GCMStreamReader, error) {
	if len(key) != 32 {
		return nil, errors.NewValidation("encryption_key", "AES-256 key must be exactly 32 bytes")
	}

	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, errors.Wrap(err, "creating AES cipher")
	}

	aead, err := cipher.NewGCM(block)
	if err != nil {
		return nil, errors.Wrap(err, "creating GCM mode")
	}

	baseNonce := make([]byte, GCMNonceSize)
	if _, err := io.ReadFull(r, baseNonce); err != nil {
		if stderrors.Is(err, io.EOF) || stderrors.Is(err, io.ErrUnexpectedEOF) {
			return nil, fmt.Errorf("%w: reading base nonce header: %v", ErrCorruptedBackupData, err)
		}
		return nil, errors.Wrap(err, "reading base nonce header")
	}

	return &GCMStreamReader{
		r:         r,
		aead:      aead,
		baseNonce: baseNonce,
	}, nil
}

func (gr *GCMStreamReader) Read(p []byte) (int, error) {
	if len(gr.buf) > 0 {
		n := copy(p, gr.buf)
		gr.buf = gr.buf[n:]
		return n, nil
	}

	if gr.eof {
		return 0, io.EOF
	}

	var hdr [5]byte
	if _, err := io.ReadFull(gr.r, hdr[:]); err != nil {
		if stderrors.Is(err, io.EOF) {
			return 0, fmt.Errorf("%w: stream truncated before final chunk", ErrCorruptedBackupData)
		}
		return 0, fmt.Errorf("%w: reading chunk header: %v", ErrCorruptedBackupData, err)
	}

	chunkLen := binary.BigEndian.Uint32(hdr[:4])
	isLast := hdr[4]

	if chunkLen < GCMTagSize || chunkLen > GCMChunkSize+GCMTagSize {
		return 0, fmt.Errorf("%w: invalid chunk length %d", ErrCorruptedBackupData, chunkLen)
	}

	ciphertext := make([]byte, chunkLen)
	if _, err := io.ReadFull(gr.r, ciphertext); err != nil {
		return 0, fmt.Errorf("%w: reading chunk ciphertext: %v", ErrCorruptedBackupData, err)
	}

	nonce := deriveNonce(gr.baseNonce, gr.chunkSeq)
	ad := make([]byte, 9)
	binary.BigEndian.PutUint64(ad[:8], gr.chunkSeq)
	ad[8] = isLast

	plaintext, err := gr.aead.Open(nil, nonce, ciphertext, ad)
	if err != nil {
		return 0, fmt.Errorf("%w: %v", ErrCorruptedBackupData, err)
	}

	gr.chunkSeq++
	if isLast == 1 {
		gr.eof = true
	}

	gr.buf = plaintext
	n := copy(p, gr.buf)
	gr.buf = gr.buf[n:]
	return n, nil
}
