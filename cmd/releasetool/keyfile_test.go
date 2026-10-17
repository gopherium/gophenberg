// SPDX-License-Identifier: Apache-2.0

package main

import (
	"bytes"
	"crypto"
	"crypto/ecdh"
	"crypto/ecdsa"
	"crypto/ed25519"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/asn1"
	"encoding/pem"
	"errors"
	"io"
	"os"
	"strings"
	"testing"
	"testing/iotest"
)

func TestWriteKeyRoundTripsAPlainEd25519Key(t *testing.T) {
	t.Parallel()

	_, key, err := ed25519.GenerateKey(nil)
	if err != nil {
		t.Fatalf("generating an Ed25519 key: %v", err)
	}
	var file bytes.Buffer

	if err := writeKey(&file, key, sealing{}); err != nil {
		t.Fatalf("writeKey: %v", err)
	}

	if !strings.HasPrefix(file.String(), "-----BEGIN PRIVATE KEY-----\n") {
		t.Errorf("writeKey wrote %q, want a plain PKCS#8 PEM block", file.String())
	}
	back, err := readKey(&file, nil)
	if err != nil || !key.Equal(back) {
		t.Errorf("readKey = %v, %v, want the key written", back, err)
	}
}

func TestWriteKeySealsAP256KeyThatOpensWithItsPassphrase(t *testing.T) {
	t.Parallel()

	key := newTestP256(t)
	var file bytes.Buffer

	if err := writeKey(&file, key, testSealing); err != nil {
		t.Fatalf("writeKey: %v", err)
	}

	if !strings.HasPrefix(file.String(), "-----BEGIN ENCRYPTED PRIVATE KEY-----\n") {
		t.Errorf("writeKey wrote %q, want an encrypted PKCS#8 PEM block", file.String())
	}
	back, err := readKey(&file, testSealing.passphrase)
	if err != nil || !key.Equal(back) {
		t.Errorf("readKey = %v, %v, want the key written", back, err)
	}
}

func TestReadKeyRefusesAWrongPassphrase(t *testing.T) {
	t.Parallel()

	var file bytes.Buffer
	if err := writeKey(&file, newTestP256(t), testSealing); err != nil {
		t.Fatalf("writeKey: %v", err)
	}

	key, err := readKey(&file, []byte("another passphrase"))

	if !errors.Is(err, errWrongPassphrase) {
		t.Errorf("readKey with another passphrase = %v, %v, want %v", key, err, errWrongPassphrase)
	}
}

func TestReadKeyOpensAKeyOpenSSLSealed(t *testing.T) {
	t.Parallel()

	key := readOpenSSLKey(t)

	public, isECDSA := key.Public().(*ecdsa.PublicKey)
	if !isECDSA || public.Curve != elliptic.P256() {
		t.Errorf("readKey of the OpenSSL file = %T, want a P-256 key", key)
	}
}

func TestReadKeyRefusesAnotherSealing(t *testing.T) {
	t.Parallel()

	pbes1 := asn1.ObjectIdentifier{1, 2, 840, 113549, 1, 5, 3}
	parameters, err := asn1.Marshal(struct {
		Salt       []byte
		Iterations int
	}{Salt: make([]byte, 8), Iterations: 2048})
	if err != nil {
		t.Fatalf("writing the PBES1 parameters: %v", err)
	}
	der, err := asn1.Marshal(encryptedKeyInfo{
		Algorithm: pkix.AlgorithmIdentifier{Algorithm: pbes1, Parameters: asn1.RawValue{FullBytes: parameters}},
		Data:      make([]byte, 32),
	})
	if err != nil {
		t.Fatalf("writing the PBES1 file: %v", err)
	}
	file := pem.EncodeToMemory(&pem.Block{Type: "ENCRYPTED PRIVATE KEY", Bytes: der})

	key, err := readKey(bytes.NewReader(file), testSealing.passphrase)

	if err == nil || !strings.Contains(err.Error(), pbes1.String()) || !strings.Contains(err.Error(), "not PBES2") {
		t.Errorf("readKey of a PBES1 file = %v, %v, want it refused by naming %s", key, err, pbes1)
	}
}

func TestReadKeyRefusesASealingItDoesNotOpen(t *testing.T) {
	t.Parallel()

	null := asn1.NullBytes
	cases := []struct {
		name   string
		change func(*sealingParts)
		want   string
	}{
		{"a derivation other than PBKDF2", func(p *sealingParts) { p.kdf = oidScrypt }, "not PBKDF2"},
		{"the default HMAC-SHA1", func(p *sealingParts) { p.prf = nil }, "not HMAC-SHA256"},
		{"HMAC-SHA512", func(p *sealingParts) { p.prf = oidHMACSHA512 }, "not HMAC-SHA256"},
		{"AES-128-CBC", func(p *sealingParts) { p.cipher = oidAES128CBC }, "not AES-256-CBC"},
		{"an IV of 8 bytes", func(p *sealingParts) { p.iv = make([]byte, 8) }, "IV of 8 bytes"},
		{"a key length of 16 bytes", func(p *sealingParts) { p.keyLength = 16 }, "key length of 16"},
		{"no rounds", func(p *sealingParts) { p.iterations = 0 }, "0 rounds"},
		{"bytes that are not whole blocks", func(p *sealingParts) { p.data = make([]byte, 15) }, "15 encrypted bytes"},
		{"no encrypted bytes", func(p *sealingParts) { p.data = nil }, "0 encrypted bytes"},
		{"bytes after the structure", func(p *sealingParts) { p.trailing = []byte{0} }, "does not parse"},
		{"PBES2 parameters that do not parse", func(p *sealingParts) { p.pbes2Params = null }, "PBES2 parameters"},
		{"PBKDF2 parameters that do not parse", func(p *sealingParts) { p.kdfParams = null }, "PBKDF2 parameters"},
		{"an IV that does not parse", func(p *sealingParts) { p.schemeParams = null }, "IV"},
	}
	for _, tc := range cases {
		parts := openableParts()
		tc.change(&parts)

		key, err := readKey(bytes.NewReader(sealedFile(t, parts)), testSealing.passphrase)

		if err == nil || !strings.Contains(err.Error(), "refused") || !strings.Contains(err.Error(), tc.want) {
			t.Errorf("readKey of %s = %v, %v, want it refused naming %q", tc.name, key, err, tc.want)
		}
	}
}

func TestReadKeyRefusesWhatHoldsNoReleaseOrRootKey(t *testing.T) {
	t.Parallel()

	sealedFile := sealedPEM(t, newTestP256(t))
	cases := []struct {
		name, file, want string
	}{
		{"no PEM block", "a release key", "holds no PEM block"},
		{"a PEM block of another type", pemOf("EC PRIVATE KEY", []byte("sec1")), "type EC PRIVATE KEY"},
		{"a plain key that does not parse", pemOf(privateKeyBlock, []byte("not DER")), "does not parse"},
		{"an X25519 key", plainPEM(t, newTestX25519(t)), "signs nothing"},
		{"an RSA key", plainPEM(t, newTestRSA(t)), "Ed25519 or P-256"},
		{"a P-384 key", plainPEM(t, newTestP384(t)), "Ed25519 or P-256"},
	}
	for _, tc := range cases {
		key, err := readKey(strings.NewReader(tc.file), nil)

		if err == nil || !strings.Contains(err.Error(), "refused") || !strings.Contains(err.Error(), tc.want) {
			t.Errorf("readKey of %s = %v, %v, want it refused naming %q", tc.name, key, err, tc.want)
		}
	}
	key, err := readKey(strings.NewReader(sealedFile), nil)
	if err == nil || !strings.Contains(err.Error(), "-passphrase-file") {
		t.Errorf("readKey of a sealed key without a passphrase = %v, %v, want it to ask for -passphrase-file", key, err)
	}
}

func TestReadKeyReportsAReaderThatFails(t *testing.T) {
	t.Parallel()

	failure := errors.New("the disk went away")

	_, err := readKey(iotest.ErrReader(failure), nil)

	if !errors.Is(err, failure) {
		t.Errorf("readKey over a failing reader = %v, want %v", err, failure)
	}
}

func TestWriteKeyRefusesASignerX509CannotWrite(t *testing.T) {
	t.Parallel()

	err := writeKey(io.Discard, unknownSigner{}, sealing{})

	if err == nil || !strings.Contains(err.Error(), "private key") {
		t.Errorf("writeKey of an unknown signer = %v, want it refused", err)
	}
}

func TestWriteKeyReportsAWriterThatFails(t *testing.T) {
	t.Parallel()

	failure := errors.New("the disk is full")

	err := writeKey(failingWriter{failure}, newTestP256(t), testSealing)

	if !errors.Is(err, failure) {
		t.Errorf("writeKey to a failing writer = %v, want %v", err, failure)
	}
}

// unknownSigner is a signer of a kind x509 cannot write.
type unknownSigner struct{}

// Public returns a public key of no known kind.
func (unknownSigner) Public() crypto.PublicKey { return struct{}{} }

// Sign refuses to sign anything.
func (unknownSigner) Sign(io.Reader, []byte, crypto.SignerOpts) ([]byte, error) {
	return nil, errors.New("unknownSigner signs nothing")
}

// failingWriter answers every write with its error.
type failingWriter struct{ err error }

// Write fails with the writer's error.
func (w failingWriter) Write([]byte) (int, error) { return 0, w.err }

// pemOf writes bytes as a PEM block of the type.
func pemOf(blockType string, der []byte) string {
	return string(pem.EncodeToMemory(&pem.Block{Type: blockType, Bytes: der}))
}

// plainPEM writes any key x509 takes as a plain PKCS#8 PEM block.
func plainPEM(t *testing.T, key any) string {
	t.Helper()
	der, err := x509.MarshalPKCS8PrivateKey(key)
	if err != nil {
		t.Fatalf("writing the %T: %v", key, err)
	}
	return pemOf(privateKeyBlock, der)
}

// sealedPEM writes a key sealed under the test sealing.
func sealedPEM(t *testing.T, key crypto.Signer) string {
	t.Helper()
	var file strings.Builder
	if err := writeKey(&file, key, testSealing); err != nil {
		t.Fatalf("writeKey: %v", err)
	}
	return file.String()
}

// newTestX25519 makes an X25519 key, which PKCS#8 holds but which signs nothing.
func newTestX25519(t *testing.T) *ecdh.PrivateKey {
	t.Helper()
	key, err := ecdh.X25519().GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("generating an X25519 key: %v", err)
	}
	return key
}

// newTestRSA makes a small RSA key for a test that only needs its kind.
func newTestRSA(t *testing.T) *rsa.PrivateKey {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("generating an RSA key: %v", err)
	}
	return key
}

// newTestP384 makes a P-384 key, a curve releasetool does not take.
func newTestP384(t *testing.T) *ecdsa.PrivateKey {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P384(), rand.Reader)
	if err != nil {
		t.Fatalf("generating a P-384 key: %v", err)
	}
	return key
}

// oidScrypt names scrypt, a key derivation readKey does not open.
var oidScrypt = asn1.ObjectIdentifier{1, 3, 6, 1, 4, 1, 11591, 4, 11}

// oidHMACSHA512 names HMAC over SHA-512, a PRF readKey does not open.
var oidHMACSHA512 = asn1.ObjectIdentifier{1, 2, 840, 113549, 2, 11}

// oidAES128CBC names AES-128 in CBC mode, a cipher readKey does not open.
var oidAES128CBC = asn1.ObjectIdentifier{2, 16, 840, 1, 101, 3, 4, 1, 2}

func TestReadKeyRefusesSealedBytesThatHoldNoKey(t *testing.T) {
	t.Parallel()

	file := pem.EncodeToMemory(&pem.Block{Type: sealedKeyBlock, Bytes: seal([]byte("no key"), testSealing)})

	key, err := readKey(bytes.NewReader(file), testSealing.passphrase)

	if !errors.Is(err, errWrongPassphrase) {
		t.Errorf("readKey of sealed bytes that hold no key = %v, %v, want %v", key, err, errWrongPassphrase)
	}
}

func TestUnpadRefusesPaddingAWrongPassphraseLeaves(t *testing.T) {
	t.Parallel()

	cases := map[string][]byte{
		"a zero length":         append(make([]byte, 15), 0),
		"a length over a block": append(make([]byte, 15), 17),
		"bytes that disagree":   append(make([]byte, 14), 1, 2),
	}
	for name, plain := range cases {
		if _, err := unpad(plain); !errors.Is(err, errWrongPassphrase) {
			t.Errorf("unpad with %s = %v, want %v", name, err, errWrongPassphrase)
		}
	}
}

func TestMustCipherPanicsOnAKeyNoDerivationGives(t *testing.T) {
	t.Parallel()

	defer func() {
		if recover() == nil {
			t.Error("mustCipher of a 5-byte key did not panic")
		}
	}()

	mustCipher(make([]byte, 5))
}

// sealingParts are the fields of a sealed key file, which a test changes one at a time.
type sealingParts struct {
	scheme, kdf, prf, cipher             asn1.ObjectIdentifier
	keyLength, iterations                int
	iv, data, trailing                   []byte
	pbes2Params, kdfParams, schemeParams []byte
}

// openableParts returns the parts of a sealing readKey opens, around bytes that hold no key.
func openableParts() sealingParts {
	return sealingParts{
		scheme: oidPBES2, kdf: oidPBKDF2, prf: oidHMACSHA256, cipher: oidAES256CBC,
		keyLength: aesKeySize, iterations: 1000, iv: make([]byte, 16), data: make([]byte, 32),
	}
}

// sealedFile writes the parts as an ENCRYPTED PRIVATE KEY PEM file.
func sealedFile(t *testing.T, p sealingParts) []byte {
	t.Helper()
	prf := pkix.AlgorithmIdentifier{}
	if p.prf != nil {
		prf = pkix.AlgorithmIdentifier{Algorithm: p.prf, Parameters: asn1.NullRawValue}
	}
	kdf := orMarshal(t, p.kdfParams, pbkdf2Params{Salt: make([]byte, 8), Iterations: p.iterations,
		KeyLength: p.keyLength, PRF: prf})
	iv := orMarshal(t, p.schemeParams, p.iv)
	params := orMarshal(t, p.pbes2Params, pbes2Params{
		KDF:    pkix.AlgorithmIdentifier{Algorithm: p.kdf, Parameters: asn1.RawValue{FullBytes: kdf}},
		Scheme: pkix.AlgorithmIdentifier{Algorithm: p.cipher, Parameters: asn1.RawValue{FullBytes: iv}},
	})
	der := orMarshal(t, nil, encryptedKeyInfo{
		Algorithm: pkix.AlgorithmIdentifier{Algorithm: p.scheme, Parameters: asn1.RawValue{FullBytes: params}},
		Data:      p.data,
	})
	return pem.EncodeToMemory(&pem.Block{Type: "ENCRYPTED PRIVATE KEY", Bytes: append(der, p.trailing...)})
}

// orMarshal returns the raw bytes a test gave, or else the DER of the value.
func orMarshal(t *testing.T, raw []byte, v any) []byte {
	t.Helper()
	if raw != nil {
		return raw
	}
	der, err := asn1.Marshal(v)
	if err != nil {
		t.Fatalf("writing %T: %v", v, err)
	}
	return der
}

func TestWritePublicKeyGivesAPEMOpenSSLReads(t *testing.T) {
	t.Parallel()

	want, err := os.ReadFile("testdata/openssl-p256.pub.pem")
	if err != nil {
		t.Fatalf("reading the PEM OpenSSL wrote: %v", err)
	}
	var got bytes.Buffer

	if err := writePublicKey(&got, readOpenSSLKey(t).Public()); err != nil {
		t.Fatalf("writePublicKey: %v", err)
	}

	if !bytes.Equal(got.Bytes(), want) {
		t.Errorf("writePublicKey wrote\n%s\nwant what openssl pkey -pubout wrote\n%s", got.Bytes(), want)
	}
}

func TestWritePublicKeyRefusesAKeyX509CannotWrite(t *testing.T) {
	t.Parallel()

	err := writePublicKey(io.Discard, struct{}{})

	if err == nil || !strings.Contains(err.Error(), "public key") {
		t.Errorf("writePublicKey of an unknown key = %v, want it refused", err)
	}
}

// opensslPassphrase is the test-only passphrase OpenSSL sealed testdata/openssl-p256.pem with.
const opensslPassphrase = "test passphrase for a synthetic key"

// readOpenSSLKey opens testdata/openssl-p256.pem, the key OpenSSL sealed with its own defaults.
func readOpenSSLKey(t *testing.T) crypto.Signer {
	t.Helper()
	file, err := os.Open("testdata/openssl-p256.pem")
	if err != nil {
		t.Fatalf("opening the OpenSSL key: %v", err)
	}
	defer func() { _ = file.Close() }()
	key, err := readKey(file, []byte(opensslPassphrase))
	if err != nil {
		t.Fatalf("readKey of the OpenSSL file: %v", err)
	}
	return key
}

// testSealing seals test keys with few rounds, since writeKey takes whatever count it is given.
var testSealing = sealing{passphrase: []byte("a test passphrase"), iterations: 1000}

// newTestP256 makes a P-256 key for a test.
func newTestP256(t *testing.T) *ecdsa.PrivateKey {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("generating a P-256 key: %v", err)
	}
	return key
}
