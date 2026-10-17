// SPDX-License-Identifier: Apache-2.0

package signature

import (
	"bytes"
	"crypto"
	"crypto/ecdsa"
	"crypto/ed25519"
	"crypto/elliptic"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"fmt"
	"io"
	"slices"
	"strings"
)

// base58Alphabet lists the base58btc digits in order of value.
const base58Alphabet = "123456789ABCDEFGHJKLMNPQRSTUVWXYZabcdefghijkmnopqrstuvwxyz"

// base58Radix is the number of base58btc digits.
const base58Radix = len(base58Alphabet)

// multibasePrefix marks a Multikey's text as base58btc.
const multibasePrefix = "z"

// codecSize is the length of the multicodec prefix of every key a Multikey here holds.
const codecSize = 2

// p256CoordinateSize is the length of one coordinate of a P-256 point.
const p256CoordinateSize = 32

// publicKeyBlock is the PEM type of a SubjectPublicKeyInfo public key.
const publicKeyBlock = "PUBLIC KEY"

// ed25519Codec is the multicodec prefix of an Ed25519 public key.
var ed25519Codec = []byte{0xed, 0x01}

// p256Codec is the multicodec prefix of a compressed P-256 public key.
var p256Codec = []byte{0x80, 0x24}

// Multikey returns a public key as a FAIR Multikey: z6Mk for Ed25519, zDn for P-256.
func Multikey(key crypto.PublicKey) (string, error) {
	payload, err := multikeyPayload(key)
	if err != nil {
		return "", err
	}
	return multibasePrefix + encodeBase58(payload), nil
}

// ParseMultikey returns the public key a Multikey holds, refusing every curve but Ed25519 and P-256.
func ParseMultikey(text string) (crypto.PublicKey, error) {
	payload, err := decodeMultikey(text)
	if err != nil {
		return nil, err
	}
	codec, key := payload[:codecSize], payload[codecSize:]
	switch {
	case bytes.Equal(codec, ed25519Codec):
		return parseEd25519(key)
	case bytes.Equal(codec, p256Codec):
		return parseP256(key)
	default:
		return nil, fmt.Errorf("signature: refused a Multikey of codec %x: only Ed25519 (ed01) and P-256 (8024) are read",
			codec)
	}
}

// ReadPublicKey reads a public key from a PEM file or a Multikey text.
func ReadPublicKey(r io.Reader) (crypto.PublicKey, error) {
	raw, err := io.ReadAll(r)
	if err != nil {
		return nil, fmt.Errorf("signature: reading the public key: %w", err)
	}
	block, _ := pem.Decode(raw)
	if block == nil {
		return ParseMultikey(string(bytes.TrimSpace(raw)))
	}
	return parsePublicPEM(block)
}

// parsePublicPEM returns the Ed25519 or P-256 key a PUBLIC KEY block holds, refusing every other block and key.
func parsePublicPEM(block *pem.Block) (crypto.PublicKey, error) {
	if block.Type != publicKeyBlock {
		return nil, fmt.Errorf("signature: refused a PEM block of type %s: a public key file holds %s", block.Type,
			publicKeyBlock)
	}
	key, err := x509.ParsePKIXPublicKey(block.Bytes)
	if err != nil {
		return nil, fmt.Errorf("signature: refused a public key that does not parse: %w", err)
	}
	if _, err := multikeyPayload(key); err != nil {
		return nil, err
	}
	return key, nil
}

// multikeyPayload returns the codec prefix and the key bytes a Multikey carries, refusing every other key.
func multikeyPayload(key crypto.PublicKey) ([]byte, error) {
	switch key := key.(type) {
	case ed25519.PublicKey:
		if len(key) != ed25519.PublicKeySize {
			return nil, fmt.Errorf("signature: refused an Ed25519 key of %d bytes, which holds %d", len(key),
				ed25519.PublicKeySize)
		}
		return slices.Concat(ed25519Codec, key), nil
	case *ecdsa.PublicKey:
		if key.Curve != elliptic.P256() {
			return nil, errors.New("signature: refused an ECDSA key on a curve other than P-256")
		}
		return slices.Concat(p256Codec, compressP256(key)), nil
	default:
		return nil, fmt.Errorf("signature: refused a %T key: a Multikey holds Ed25519 or P-256", key)
	}
}

// decodeMultikey returns the payload of a Multikey's text, refusing text too short to name a codec.
func decodeMultikey(text string) ([]byte, error) {
	body, found := strings.CutPrefix(text, multibasePrefix)
	if !found {
		return nil, errors.New("signature: refused a Multikey that does not start with z, the base58btc prefix")
	}
	if strings.HasPrefix(body, base58Alphabet[:1]) {
		return nil, errors.New("signature: refused a Multikey that starts with a zero byte, which names no codec")
	}
	payload, err := decodeBase58(body)
	if err != nil {
		return nil, err
	}
	if len(payload) < codecSize {
		return nil, fmt.Errorf("signature: refused a Multikey of %d bytes, too short to name a codec", len(payload))
	}
	return payload, nil
}

// parseEd25519 returns the Ed25519 key a Multikey's key bytes hold, refusing a wrong length.
func parseEd25519(key []byte) (crypto.PublicKey, error) {
	if len(key) != ed25519.PublicKeySize {
		return nil, fmt.Errorf("signature: refused an Ed25519 Multikey of %d bytes, which holds %d", len(key),
			ed25519.PublicKeySize)
	}
	return ed25519.PublicKey(key), nil
}

// compressP256 returns the 33-byte compressed point of a P-256 key.
func compressP256(key *ecdsa.PublicKey) []byte {
	point, _ := key.Bytes()
	return append([]byte{2 | point[2*p256CoordinateSize]&1}, point[1:1+p256CoordinateSize]...)
}

// parseP256 returns the P-256 key a compressed point names, refusing a point off the curve.
func parseP256(compressed []byte) (crypto.PublicKey, error) {
	x, y := elliptic.UnmarshalCompressed(elliptic.P256(), compressed)
	if x == nil {
		return nil, errors.New("signature: refused a P-256 Multikey that holds no compressed point on the curve")
	}
	point := make([]byte, 1+2*p256CoordinateSize)
	point[0] = 4
	x.FillBytes(point[1 : 1+p256CoordinateSize])
	y.FillBytes(point[1+p256CoordinateSize:])
	key, _ := ecdsa.ParseUncompressedPublicKey(elliptic.P256(), point)
	return key, nil
}

// decodeBase58 reads base58btc text, refusing a character outside the alphabet.
func decodeBase58(text string) ([]byte, error) {
	var payload []byte
	for i := range len(text) {
		carry := strings.IndexByte(base58Alphabet, text[i])
		if carry < 0 {
			return nil, fmt.Errorf("signature: refused a Multikey holding %q, which base58btc does not use", text[i])
		}
		for j := range payload {
			carry += int(payload[j]) * base58Radix
			payload[j] = byte(carry)
			carry >>= 8
		}
		for carry > 0 {
			payload = append(payload, byte(carry))
			carry >>= 8
		}
	}
	slices.Reverse(payload)
	return payload, nil
}

// encodeBase58 writes a payload that starts with a non-zero codec byte in base58btc.
func encodeBase58(payload []byte) string {
	var digits []byte
	for _, b := range payload {
		carry := int(b)
		for i := range digits {
			carry += int(digits[i]) << 8
			digits[i] = byte(carry % base58Radix)
			carry /= base58Radix
		}
		for carry > 0 {
			digits = append(digits, byte(carry%base58Radix))
			carry /= base58Radix
		}
	}
	text := make([]byte, len(digits))
	for i, digit := range digits {
		text[len(text)-1-i] = base58Alphabet[digit]
	}
	return string(text)
}
