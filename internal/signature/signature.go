// SPDX-License-Identifier: Apache-2.0

// Package signature signs, checks and fingerprints release files in FAIR's wire format.
package signature

import (
	"crypto/ed25519"
	"crypto/sha512"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"strings"
)

// DigestSize is the length of the SHA-384 digest a signature covers.
const DigestSize = sha512.Size384

// ErrMismatch reports a well-formed signature that does not hold for the digest under the key.
var ErrMismatch = errors.New("signature: refused a signature that does not hold for this digest under this key")

// errStandardBase64 refuses a signature written in standard base64 or with padding.
var errStandardBase64 = errors.New(
	"signature: refused a signature in standard base64 or with padding: FAIR writes base64url without padding")

// Digest returns the SHA-384 digest of what r holds, the bytes a signature covers.
func Digest(r io.Reader) ([DigestSize]byte, error) {
	var digest [DigestSize]byte
	hash := sha512.New384()
	if _, err := io.Copy(hash, r); err != nil {
		return digest, fmt.Errorf("signature: reading the bytes to digest: %w", err)
	}
	hash.Sum(digest[:0])
	return digest, nil
}

// Sign returns the signature of a digest: Ed25519 over its raw bytes, in base64url without padding.
func Sign(key ed25519.PrivateKey, digest [DigestSize]byte) string {
	return base64.RawURLEncoding.EncodeToString(ed25519.Sign(key, digest[:]))
}

// Verify reports whether a signature of a digest holds for the key, refusing every other encoding.
func Verify(key ed25519.PublicKey, digest [DigestSize]byte, signature string) error {
	if len(key) != ed25519.PublicKeySize {
		return fmt.Errorf("signature: refused an Ed25519 key of %d bytes, which holds %d", len(key), ed25519.PublicKeySize)
	}
	raw, err := decodeSignature(signature)
	if err != nil {
		return err
	}
	if !ed25519.Verify(key, digest[:], raw) {
		return ErrMismatch
	}
	return nil
}

// decodeSignature returns the bytes of a signature in base64url without padding, refusing every other form.
func decodeSignature(signature string) ([]byte, error) {
	if strings.ContainsAny(signature, "+/=") {
		return nil, errStandardBase64
	}
	if strings.ContainsAny(signature, "\r\n") {
		return nil, errors.New("signature: refused a signature that is not base64url without padding: it breaks a line")
	}
	raw, err := base64.RawURLEncoding.Strict().DecodeString(signature)
	if err != nil {
		return nil, fmt.Errorf("signature: refused a signature that is not base64url without padding: %w", err)
	}
	if len(raw) != ed25519.SignatureSize {
		return nil, fmt.Errorf("signature: refused a signature of %d bytes, which holds %d", len(raw), ed25519.SignatureSize)
	}
	return raw, nil
}
