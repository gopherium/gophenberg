// SPDX-License-Identifier: Apache-2.0

package signature_test

import (
	"crypto"
	"crypto/ecdsa"
	"crypto/ed25519"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"math/big"
	"slices"
	"strings"
	"testing"
	"testing/iotest"

	"github.com/gopherium/gophenberg/internal/signature"
)

func TestMultikeyMatchesTheLiveAndSeedKeys(t *testing.T) {
	t.Parallel()

	vectors := loadVectors(t)
	cases := []publishedMultikey{{Name: "the seed", PublicKey: vectors.Seed.PublicKey, Multikey: vectors.Seed.Multikey}}
	cases = append(cases, vectors.PublishedMultikeys...)
	if len(cases) != 2 {
		t.Fatalf("the test data holds %d published Multikeys, want 1", len(cases)-1)
	}
	for _, tc := range cases {
		got, err := signature.Multikey(ed25519.PublicKey(mustHex(t, tc.PublicKey)))

		if err != nil || got != tc.Multikey {
			t.Errorf("Multikey(%s) = %q, %v, want %q", tc.Name, got, err, tc.Multikey)
		}
	}
}

func TestParseMultikeyReadsP256BackToTheSameText(t *testing.T) {
	t.Parallel()

	fresh, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("generating a P-256 key: %v", err)
	}
	freshText, err := signature.Multikey(&fresh.PublicKey)
	if err != nil {
		t.Fatalf("Multikey of a fresh P-256 key: %v", err)
	}
	for _, text := range []string{loadVectors(t).P256Multikey, freshText} {
		key, err := signature.ParseMultikey(text)
		if err != nil {
			t.Fatalf("ParseMultikey(%q): %v", text, err)
		}
		if _, isP256 := key.(*ecdsa.PublicKey); !isP256 {
			t.Fatalf("ParseMultikey(%q) = %T, want a P-256 key", text, key)
		}

		back, err := signature.Multikey(key)

		if err != nil || back != text {
			t.Errorf("Multikey(ParseMultikey(%q)) = %q, %v, want the text back", text, back, err)
		}
	}
	if !strings.HasPrefix(freshText, "zDn") {
		t.Errorf("Multikey of a fresh P-256 key = %q, want it to start with zDn", freshText)
	}
}

func TestParseMultikeyRefusesSecp256k1(t *testing.T) {
	t.Parallel()

	text := loadVectors(t).Refused.Secp256k1Multikey

	key, err := signature.ParseMultikey(text)

	if err == nil || !strings.Contains(err.Error(), "e701") {
		t.Errorf("ParseMultikey(%q) = %v, %v, want it refused by naming the codec e701", text, key, err)
	}
}

func TestParseMultikeyReadsTheLiveEd25519Key(t *testing.T) {
	t.Parallel()

	published := loadVectors(t).PublishedMultikeys[0]

	key, err := signature.ParseMultikey(published.Multikey)

	want := ed25519.PublicKey(mustHex(t, published.PublicKey))
	if got, isEd25519 := key.(ed25519.PublicKey); err != nil || !isEd25519 || !got.Equal(want) {
		t.Errorf("ParseMultikey(%q) = %v, %v, want %x", published.Multikey, key, err, []byte(want))
	}
}

func TestParseMultikeyRefusesTextThatIsNotBase58btc(t *testing.T) {
	t.Parallel()

	live := loadVectors(t).PublishedMultikeys[0].Multikey
	cases := map[string]string{
		"no multibase prefix":       live[1:],
		"a character outside":       live[:20] + "0" + live[21:],
		"a leading zero byte":       "z1" + live[1:],
		"too short to name a codec": "z2",
		"empty":                     "z",
	}
	for name, text := range cases {
		key, err := signature.ParseMultikey(text)

		if err == nil {
			t.Errorf("ParseMultikey with %s (%q) = %v, want it refused", name, text, key)
		}
	}
}

func TestParseMultikeyRefusesAKeyOfTheWrongLength(t *testing.T) {
	t.Parallel()

	cases := map[string][]byte{
		"an Ed25519 key of 31 bytes": slices.Concat([]byte{0xed, 0x01}, make([]byte, 31)),
		"a P-256 point of 32 bytes":  slices.Concat([]byte{0x80, 0x24}, mustHex(t, strings.Repeat("02", 32))),
	}
	for name, payload := range cases {
		text := "z" + base58(payload)

		key, err := signature.ParseMultikey(text)

		if err == nil || !strings.Contains(err.Error(), "refused") {
			t.Errorf("ParseMultikey of %s = %v, %v, want it refused", name, key, err)
		}
	}
}

func TestMultikeyRefusesKeysItCannotWrite(t *testing.T) {
	t.Parallel()

	p384, err := ecdsa.GenerateKey(elliptic.P384(), rand.Reader)
	if err != nil {
		t.Fatalf("generating a P-384 key: %v", err)
	}
	cases := map[string]crypto.PublicKey{
		"an Ed25519 key of 31 bytes": ed25519.PublicKey(make([]byte, 31)),
		"a P-384 key":                &p384.PublicKey,
		"an RSA key":                 &rsa.PublicKey{},
	}
	for name, key := range cases {
		text, err := signature.Multikey(key)

		if err == nil || !strings.Contains(err.Error(), "refused") {
			t.Errorf("Multikey of %s = %q, %v, want it refused", name, text, err)
		}
	}
}

func TestReadPublicKeyTakesAPEMOrAMultikey(t *testing.T) {
	t.Parallel()

	published := loadVectors(t).PublishedMultikeys[0]
	live := ed25519.PublicKey(mustHex(t, published.PublicKey))
	root, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("generating a P-256 key: %v", err)
	}
	rootPEM := publicPEM(t, &root.PublicKey)
	cases := map[string]struct {
		text string
		want crypto.PublicKey
	}{
		"a Multikey":                      {text: published.Multikey, want: live},
		"a Multikey and a newline":        {text: published.Multikey + "\n", want: live},
		"an Ed25519 PEM":                  {text: publicPEM(t, live), want: live},
		"a P-256 PEM":                     {text: rootPEM, want: &root.PublicKey},
		"a PEM after a line of its own":   {text: "release key\n" + publicPEM(t, live), want: live},
		"a P-256 PEM with no final break": {text: strings.TrimSuffix(rootPEM, "\n"), want: &root.PublicKey},
	}
	for name, tc := range cases {
		key, err := signature.ReadPublicKey(strings.NewReader(tc.text))

		if equal, ok := key.(interface{ Equal(crypto.PublicKey) bool }); err != nil || !ok || !equal.Equal(tc.want) {
			t.Errorf("ReadPublicKey of %s = %v, %v, want %v", name, key, err, tc.want)
		}
	}
}

func TestReadPublicKeyRefusesEverythingElse(t *testing.T) {
	t.Parallel()

	signer, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("generating an RSA key: %v", err)
	}
	private, err := x509.MarshalPKCS8PrivateKey(signer)
	if err != nil {
		t.Fatalf("writing the RSA key: %v", err)
	}
	cases := map[string]string{
		"an RSA public key":   publicPEM(t, &signer.PublicKey),
		"a private key":       string(pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: private})),
		"a broken public key": string(pem.EncodeToMemory(&pem.Block{Type: "PUBLIC KEY", Bytes: []byte("not DER")})),
		"a hexadecimal key":   strings.Repeat("ab", 32),
	}
	for name, text := range cases {
		key, err := signature.ReadPublicKey(strings.NewReader(text))

		if err == nil || !strings.Contains(err.Error(), "refused") {
			t.Errorf("ReadPublicKey of %s = %v, %v, want it refused", name, key, err)
		}
	}
}

func TestReadPublicKeyReportsAReaderThatFails(t *testing.T) {
	t.Parallel()

	failure := errors.New("the disk went away")

	_, err := signature.ReadPublicKey(iotest.ErrReader(failure))

	if !errors.Is(err, failure) {
		t.Errorf("ReadPublicKey over a failing reader = %v, want %v", err, failure)
	}
}

// publicPEM writes a public key as SubjectPublicKeyInfo PEM, failing the test on a key x509 cannot write.
func publicPEM(t *testing.T, key crypto.PublicKey) string {
	t.Helper()
	der, err := x509.MarshalPKIXPublicKey(key)
	if err != nil {
		t.Fatalf("writing the public key: %v", err)
	}
	return string(pem.EncodeToMemory(&pem.Block{Type: "PUBLIC KEY", Bytes: der}))
}

// base58 writes bytes in base58btc through big integers, a second implementation to test against.
func base58(raw []byte) string {
	const alphabet = "123456789ABCDEFGHJKLMNPQRSTUVWXYZabcdefghijkmnopqrstuvwxyz"
	n, base, digit := new(big.Int).SetBytes(raw), big.NewInt(58), new(big.Int)
	var text []byte
	for n.Sign() > 0 {
		n.DivMod(n, base, digit)
		text = append([]byte{alphabet[digit.Int64()]}, text...)
	}
	return string(text)
}
