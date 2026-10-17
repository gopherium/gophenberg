// SPDX-License-Identifier: Apache-2.0

package signature_test

import (
	"crypto/ed25519"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"os"
	"strings"
	"testing"
	"testing/iotest"

	"github.com/gopherium/gophenberg/internal/signature"
)

// liveSigner is one live FAIR signer: its key and digest in hexadecimal, its signature as published.
type liveSigner struct {
	Name      string `json:"name"`
	PublicKey string `json:"public_key"`
	Digest    string `json:"digest"`
	Signature string `json:"signature"`
}

// seedVector is the RFC 8032 TEST 1 seed over a chosen message, with every value the message gives.
type seedVector struct {
	Seed      string `json:"seed"`
	PublicKey string `json:"public_key"`
	Multikey  string `json:"multikey"`
	Message   string `json:"message"`
	Digest    string `json:"digest"`
	Checksum  string `json:"checksum"`
	Signature string `json:"signature"`
}

// refusedVectors holds the inputs the package must refuse.
type refusedVectors struct {
	HexDigestSignature string `json:"seed_signature_over_the_hex_digest"`
	StandardBase64     string `json:"query_monitor_3_20_2_in_standard_base64"`
	Secp256k1Multikey  string `json:"secp256k1_multikey"`
}

// publishedMultikey is a live key in hexadecimal and the Multikey its owner published for it.
type publishedMultikey struct {
	Name      string `json:"name"`
	PublicKey string `json:"public_key"`
	Multikey  string `json:"multikey"`
}

// testVectors is everything testdata/signers.json holds.
type testVectors struct {
	Live               []liveSigner        `json:"live"`
	PublishedMultikeys []publishedMultikey `json:"published_multikeys"`
	Seed               seedVector          `json:"seed"`
	P256Multikey       string              `json:"p256_multikey"`
	Refused            refusedVectors      `json:"refused"`
}

// loadVectors reads testdata/signers.json.
func loadVectors(t *testing.T) testVectors {
	t.Helper()
	raw, err := os.ReadFile("testdata/signers.json")
	if err != nil {
		t.Fatalf("reading the test vectors: %v", err)
	}
	var vectors testVectors
	if err := json.Unmarshal(raw, &vectors); err != nil {
		t.Fatalf("decoding the test vectors: %v", err)
	}
	return vectors
}

// mustHex decodes hexadecimal test data, failing the test on bad input.
func mustHex(t *testing.T, text string) []byte {
	t.Helper()
	raw, err := hex.DecodeString(text)
	if err != nil {
		t.Fatalf("decoding %q: %v", text, err)
	}
	return raw
}

// mustBase64URL decodes base64url test data without padding, failing the test on bad input.
func mustBase64URL(t *testing.T, text string) []byte {
	t.Helper()
	raw, err := base64.RawURLEncoding.DecodeString(text)
	if err != nil {
		t.Fatalf("decoding %q: %v", text, err)
	}
	return raw
}

// digestOf returns hexadecimal test data as a digest, failing the test on a wrong length.
func digestOf(t *testing.T, text string) [signature.DigestSize]byte {
	t.Helper()
	var digest [signature.DigestSize]byte
	if n := copy(digest[:], mustHex(t, text)); n != len(text)/2 || n != signature.DigestSize {
		t.Fatalf("the digest %q holds %d bytes, want %d", text, len(text)/2, signature.DigestSize)
	}
	return digest
}

func TestVerifyAcceptsTheLiveFAIRSigners(t *testing.T) {
	t.Parallel()

	live := loadVectors(t).Live
	if len(live) != 4 {
		t.Fatalf("the test data holds %d live signers, want 4", len(live))
	}
	for _, signer := range live {
		t.Run(signer.Name, func(t *testing.T) {
			t.Parallel()

			key := ed25519.PublicKey(mustHex(t, signer.PublicKey))
			if err := signature.Verify(key, digestOf(t, signer.Digest), signer.Signature); err != nil {
				t.Errorf("Verify(%s) = %v, want nil", signer.Name, err)
			}
		})
	}
}

func TestVerifyRefusesAFlippedDigestBit(t *testing.T) {
	t.Parallel()

	signer := loadVectors(t).Live[0]
	digest := digestOf(t, signer.Digest)
	digest[len(digest)-1] ^= 1

	err := signature.Verify(ed25519.PublicKey(mustHex(t, signer.PublicKey)), digest, signer.Signature)

	if !errors.Is(err, signature.ErrMismatch) {
		t.Errorf("Verify over a flipped bit = %v, want %v", err, signature.ErrMismatch)
	}
}

func TestVerifyRefusesStandardBase64WithPadding(t *testing.T) {
	t.Parallel()

	vectors := loadVectors(t)
	signer := vectors.Live[0]

	err := signature.Verify(ed25519.PublicKey(mustHex(t, signer.PublicKey)), digestOf(t, signer.Digest),
		vectors.Refused.StandardBase64)

	if err == nil || errors.Is(err, signature.ErrMismatch) {
		t.Fatalf("Verify in standard base64 = %v, want the encoding refused", err)
	}
	for _, want := range []string{"standard base64", "base64url without padding"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("Verify in standard base64 = %q, want it to name %q", err, want)
		}
	}
}

func TestVerifyRefusesASignatureThatBreaksALine(t *testing.T) {
	t.Parallel()

	signer := loadVectors(t).Live[0]
	key, digest := ed25519.PublicKey(mustHex(t, signer.PublicKey)), digestOf(t, signer.Digest)

	trailing, inside := signer.Signature+"\n", signer.Signature[:40]+"\r\n"+signer.Signature[40:]
	for _, broken := range []string{trailing, inside} {
		err := signature.Verify(key, digest, broken)

		if err == nil || !strings.Contains(err.Error(), "not base64url without padding") {
			t.Errorf("Verify(%q) = %v, want the line break refused", broken, err)
		}
	}
}

func TestVerifyRefusesASignatureOutsideTheBase64URLAlphabet(t *testing.T) {
	t.Parallel()

	signer := loadVectors(t).Live[0]

	err := signature.Verify(ed25519.PublicKey(mustHex(t, signer.PublicKey)), digestOf(t, signer.Digest),
		signer.Signature[:85]+"!")

	if err == nil || !strings.Contains(err.Error(), "not base64url without padding") {
		t.Errorf("Verify with a character outside the alphabet = %v, want the encoding refused", err)
	}
}

func TestVerifyRefusesANonCanonicalEncoding(t *testing.T) {
	t.Parallel()

	signer := loadVectors(t).Live[0]
	if !strings.HasSuffix(signer.Signature, "w") {
		t.Fatalf("the signature %q no longer ends in w, pick another last character", signer.Signature)
	}

	err := signature.Verify(ed25519.PublicKey(mustHex(t, signer.PublicKey)), digestOf(t, signer.Digest),
		signer.Signature[:85]+"x")

	if err == nil || !strings.Contains(err.Error(), "not base64url without padding") {
		t.Errorf("Verify with set trailing bits = %v, want the encoding refused", err)
	}
}

func TestVerifyRefusesASignatureOfTheWrongLength(t *testing.T) {
	t.Parallel()

	signer := loadVectors(t).Live[0]

	err := signature.Verify(ed25519.PublicKey(mustHex(t, signer.PublicKey)), digestOf(t, signer.Digest),
		signer.Signature[:84])

	if err == nil || !strings.Contains(err.Error(), "63 bytes") {
		t.Errorf("Verify with a short signature = %v, want it refused by its length", err)
	}
}

func TestDigestIsTheSHA384OfTheBytes(t *testing.T) {
	t.Parallel()

	seed := loadVectors(t).Seed

	digest, err := signature.Digest(strings.NewReader(seed.Message))

	if err != nil {
		t.Fatalf("Digest: %v", err)
	}
	if digest != digestOf(t, seed.Digest) {
		t.Errorf("Digest = %x, want %s", digest, seed.Digest)
	}
}

func TestDigestReportsAReaderThatFails(t *testing.T) {
	t.Parallel()

	failure := errors.New("the disk went away")

	_, err := signature.Digest(iotest.ErrReader(failure))

	if !errors.Is(err, failure) {
		t.Errorf("Digest over a failing reader = %v, want %v", err, failure)
	}
}

func TestSignGivesTheVectorOpenSSLSigned(t *testing.T) {
	t.Parallel()

	seed := loadVectors(t).Seed
	key := ed25519.NewKeyFromSeed(mustHex(t, seed.Seed))
	if public := key.Public().(ed25519.PublicKey); !public.Equal(ed25519.PublicKey(mustHex(t, seed.PublicKey))) {
		t.Fatalf("the seed gives the key %x, want RFC 8032's %s", public, seed.PublicKey)
	}

	got := signature.Sign(key, digestOf(t, seed.Digest))

	if got != seed.Signature {
		t.Errorf("Sign = %q, want %q", got, seed.Signature)
	}
}

func TestVerifyRefusesASignatureOverTheHexDigest(t *testing.T) {
	t.Parallel()

	vectors := loadVectors(t)
	seed, overHex := vectors.Seed, vectors.Refused.HexDigestSignature
	public := ed25519.NewKeyFromSeed(mustHex(t, seed.Seed)).Public().(ed25519.PublicKey)
	if !ed25519.Verify(public, []byte(seed.Digest), mustBase64URL(t, overHex)) {
		t.Fatalf("the refused vector is no signature over the %d character hex digest", len(seed.Digest))
	}

	err := signature.Verify(public, digestOf(t, seed.Digest), overHex)

	if !errors.Is(err, signature.ErrMismatch) {
		t.Errorf("Verify of a signature over the hex digest = %v, want %v", err, signature.ErrMismatch)
	}
}

func TestVerifyRefusesAKeyOfTheWrongLength(t *testing.T) {
	t.Parallel()

	signer := loadVectors(t).Live[0]

	err := signature.Verify(ed25519.PublicKey(mustHex(t, signer.PublicKey)[:31]), digestOf(t, signer.Digest),
		signer.Signature)

	if err == nil || !strings.Contains(err.Error(), "31 bytes") {
		t.Errorf("Verify under a short key = %v, want it refused by its length", err)
	}
}
