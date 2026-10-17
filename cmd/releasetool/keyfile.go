// SPDX-License-Identifier: Apache-2.0

package main

import (
	"bytes"
	"crypto"
	"crypto/aes"
	"crypto/cipher"
	"crypto/pbkdf2"
	"crypto/rand"
	"crypto/sha256"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/asn1"
	"encoding/pem"
	"errors"
	"fmt"
	"io"
	"os"

	"github.com/gopherium/gophenberg/internal/signature"
)

// privateKeyBlock is the PEM type of a plain PKCS#8 private key.
const privateKeyBlock = "PRIVATE KEY"

// sealedKeyBlock is the PEM type of an encrypted PKCS#8 private key.
const sealedKeyBlock = "ENCRYPTED PRIVATE KEY"

// publicKeyBlock is the PEM type of a SubjectPublicKeyInfo public key.
const publicKeyBlock = "PUBLIC KEY"

// saltSize is the length of the random salt keygen seals a key file with.
const saltSize = 16

// aesKeySize is the length of an AES-256 key.
const aesKeySize = 32

// opensOnly names the one sealing readKey opens.
const opensOnly = "releasetool opens PBES2 with PBKDF2-HMAC-SHA256 and AES-256-CBC only"

// errWrongPassphrase reports a passphrase that does not open a sealed key file.
var errWrongPassphrase = errors.New("the passphrase does not open this key")

// oidPBES2 names the PKCS #5 password-based encryption scheme 2.
var oidPBES2 = asn1.ObjectIdentifier{1, 2, 840, 113549, 1, 5, 13}

// oidPBKDF2 names the PKCS #5 password-based key derivation function 2.
var oidPBKDF2 = asn1.ObjectIdentifier{1, 2, 840, 113549, 1, 5, 12}

// oidHMACSHA256 names HMAC over SHA-256, the function PBKDF2 derives the key with.
var oidHMACSHA256 = asn1.ObjectIdentifier{1, 2, 840, 113549, 2, 9}

// oidAES256CBC names AES-256 in CBC mode, the cipher that encrypts the key.
var oidAES256CBC = asn1.ObjectIdentifier{2, 16, 840, 1, 101, 3, 4, 1, 42}

// phrase holds the bytes of a passphrase, the secret that seals and opens a key file.
type phrase []byte

// derivedKey is the AES-256 key PBKDF2 derives from a passphrase.
type derivedKey []byte

// sealing says how a passphrase protects a key file, an empty passphrase meaning not at all.
type sealing struct {
	passphrase phrase
	iterations int
}

// encryptedKeyInfo is the PKCS #8 EncryptedPrivateKeyInfo structure.
type encryptedKeyInfo struct {
	Algorithm pkix.AlgorithmIdentifier
	Data      []byte
}

// pbes2Params is the PBES2-params structure: the key derivation and the encryption scheme.
type pbes2Params struct {
	KDF    pkix.AlgorithmIdentifier
	Scheme pkix.AlgorithmIdentifier
}

// pbkdf2Params is the PBKDF2-params structure, whose key length and PRF RFC 8018 makes optional.
type pbkdf2Params struct {
	Salt       []byte
	Iterations int
	KeyLength  int                      `asn1:"optional"`
	PRF        pkix.AlgorithmIdentifier `asn1:"optional"`
}

// sealed holds what opening a sealed key file needs: the salt, the rounds, the IV and the encrypted key.
type sealed struct {
	salt       []byte
	iterations int
	iv         []byte
	data       []byte
}

// writeKey writes a private key as PKCS#8 PEM, encrypted when the sealing holds a passphrase.
func writeKey(w io.Writer, key crypto.Signer, s sealing) error {
	der, err := x509.MarshalPKCS8PrivateKey(key)
	if err != nil {
		return fmt.Errorf("writing the private key: %w", err)
	}
	block := &pem.Block{Type: privateKeyBlock, Bytes: der}
	if len(s.passphrase) > 0 {
		block = &pem.Block{Type: sealedKeyBlock, Bytes: seal(der, s)}
	}
	return pem.Encode(w, block)
}

// writePublicKey writes a public key as SubjectPublicKeyInfo PEM, the form OpenSSL reads.
func writePublicKey(w io.Writer, key crypto.PublicKey) error {
	der, err := x509.MarshalPKIXPublicKey(key)
	if err != nil {
		return fmt.Errorf("writing the public key: %w", err)
	}
	return pem.Encode(w, &pem.Block{Type: publicKeyBlock, Bytes: der})
}

// readPassphrase returns the first line of a passphrase file, refusing a terminal, a carriage return and an empty line.
func readPassphrase(path string) (phrase, error) {
	if info, err := os.Stat(path); err == nil && info.Mode()&os.ModeCharDevice != 0 {
		return nil, fmt.Errorf("refused the passphrase file %s: it is a terminal or other device, where a typed "+
			"passphrase shows on the screen, so pipe the passphrase in or name a file", path)
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading the passphrase: %w", err)
	}
	line, _, _ := bytes.Cut(raw, []byte("\n"))
	if bytes.HasSuffix(line, []byte("\r")) {
		return nil, fmt.Errorf("refused the passphrase file %s: its first line ends in a carriage return, which "+
			"OpenSSL keeps as part of the passphrase, so save the file with Unix line endings", path)
	}
	if len(line) == 0 {
		return nil, fmt.Errorf("refused the passphrase file %s: its first line is empty", path)
	}
	return line, nil
}

// readKey reads a private key file, opening an encrypted one with the passphrase.
func readKey(r io.Reader, passphrase phrase) (crypto.Signer, error) {
	block, err := readBlock(r)
	if err != nil {
		return nil, err
	}
	key, err := parseKeyBlock(block, passphrase)
	if err != nil {
		return nil, err
	}
	return signerOf(key)
}

// readBlock returns the first PEM block a key file holds.
func readBlock(r io.Reader) (*pem.Block, error) {
	raw, err := io.ReadAll(r)
	if err != nil {
		return nil, fmt.Errorf("reading the key file: %w", err)
	}
	block, _ := pem.Decode(raw)
	if block == nil {
		return nil, errors.New("refused a key file that holds no PEM block")
	}
	return block, nil
}

// parseKeyBlock parses a plain key block or opens a sealed one, refusing every other block.
func parseKeyBlock(block *pem.Block, passphrase phrase) (any, error) {
	switch block.Type {
	case privateKeyBlock:
		return parsePlain(block.Bytes)
	case sealedKeyBlock:
		return openSealed(block.Bytes, passphrase)
	default:
		return nil, fmt.Errorf("refused a PEM block of type %s: a key file holds %s or %s", block.Type,
			privateKeyBlock, sealedKeyBlock)
	}
}

// signerOf returns a parsed key as a signer, refusing every key but Ed25519 and P-256.
func signerOf(key any) (crypto.Signer, error) {
	signer, isSigner := key.(crypto.Signer)
	if !isSigner {
		return nil, fmt.Errorf("refused a key file holding a %T, which signs nothing", key)
	}
	if _, err := signature.Multikey(signer.Public()); err != nil {
		return nil, fmt.Errorf("refused a key file holding a %T: releasetool keys are Ed25519 or P-256", key)
	}
	return signer, nil
}

// parsePlain parses the PKCS#8 bytes of a plain key file.
func parsePlain(der []byte) (any, error) {
	key, err := x509.ParsePKCS8PrivateKey(der)
	if err != nil {
		return nil, fmt.Errorf("refused a private key that does not parse: %w", err)
	}
	return key, nil
}

// seal encrypts PKCS#8 bytes under the sealing as an EncryptedPrivateKeyInfo.
func seal(der []byte, s sealing) []byte {
	derivation, iv := sealed{salt: make([]byte, saltSize), iterations: s.iterations}, make([]byte, aes.BlockSize)
	_, _ = rand.Read(derivation.salt)
	_, _ = rand.Read(iv)
	key, _ := deriveKey(s.passphrase, derivation)
	padding := aes.BlockSize - len(der)%aes.BlockSize
	plain := append(bytes.Clone(der), bytes.Repeat([]byte{byte(padding)}, padding)...)
	data := make([]byte, len(plain))
	cipher.NewCBCEncrypter(mustCipher(key), iv).CryptBlocks(data, plain)
	kdf := pbkdf2Params{
		Salt:       derivation.salt,
		Iterations: s.iterations,
		KeyLength:  aesKeySize,
		PRF:        pkix.AlgorithmIdentifier{Algorithm: oidHMACSHA256, Parameters: asn1.NullRawValue},
	}
	params := pbes2Params{
		KDF:    pkix.AlgorithmIdentifier{Algorithm: oidPBKDF2, Parameters: rawValue(kdf)},
		Scheme: pkix.AlgorithmIdentifier{Algorithm: oidAES256CBC, Parameters: rawValue(iv)},
	}
	info := encryptedKeyInfo{
		Algorithm: pkix.AlgorithmIdentifier{Algorithm: oidPBES2, Parameters: rawValue(params)},
		Data:      data,
	}
	out, _ := asn1.Marshal(info)
	return out
}

// openSealed decrypts an EncryptedPrivateKeyInfo with the passphrase and parses the key it holds.
func openSealed(der []byte, passphrase phrase) (any, error) {
	if len(passphrase) == 0 {
		return nil, errors.New("refused to open a sealed key file without a passphrase: give -passphrase-file")
	}
	s, err := parseSealing(der)
	if err != nil {
		return nil, err
	}
	key, err := deriveKey(passphrase, s)
	if err != nil {
		return nil, fmt.Errorf("deriving the key that opens the file: %w", err)
	}
	plain, err := openCBC(key, s)
	if err != nil {
		return nil, err
	}
	parsed, err := x509.ParsePKCS8PrivateKey(plain)
	if err != nil {
		return nil, errWrongPassphrase
	}
	return parsed, nil
}

// openCBC decrypts a sealed key's bytes with AES-256-CBC and removes their padding.
func openCBC(key derivedKey, s sealed) ([]byte, error) {
	if len(s.data) == 0 || len(s.data)%aes.BlockSize != 0 {
		return nil, fmt.Errorf("refused %d encrypted bytes: AES-256-CBC writes whole blocks of %d", len(s.data),
			aes.BlockSize)
	}
	plain := make([]byte, len(s.data))
	cipher.NewCBCDecrypter(mustCipher(key), s.iv).CryptBlocks(plain, s.data)
	return unpad(plain)
}

// unpad removes PKCS#7 padding, refusing padding a wrong passphrase leaves behind.
func unpad(plain []byte) ([]byte, error) {
	n := int(plain[len(plain)-1])
	if n == 0 || n > aes.BlockSize {
		return nil, errWrongPassphrase
	}
	if !bytes.Equal(plain[len(plain)-n:], bytes.Repeat([]byte{byte(n)}, n)) {
		return nil, errWrongPassphrase
	}
	return plain[:len(plain)-n], nil
}

// parseSealing reads the salt, rounds and IV of a PBES2 sealing, refusing every other sealing by name.
func parseSealing(der []byte) (sealed, error) {
	var info encryptedKeyInfo
	if err := unmarshalWhole(der, &info); err != nil {
		return sealed{}, fmt.Errorf("refused a sealed key that does not parse: %w", err)
	}
	if !info.Algorithm.Algorithm.Equal(oidPBES2) {
		return sealed{}, fmt.Errorf("refused a key sealed with %s, not PBES2: %s", info.Algorithm.Algorithm, opensOnly)
	}
	var params pbes2Params
	if err := unmarshalWhole(info.Algorithm.Parameters.FullBytes, &params); err != nil {
		return sealed{}, fmt.Errorf("refused PBES2 parameters that do not parse: %w", err)
	}
	s, err := parseKDF(params.KDF)
	if err != nil {
		return sealed{}, err
	}
	s.iv, err = parseScheme(params.Scheme)
	s.data = info.Data
	return s, err
}

// parseKDF reads the salt and rounds of PBKDF2-HMAC-SHA256, refusing another derivation or PRF.
func parseKDF(kdf pkix.AlgorithmIdentifier) (sealed, error) {
	if !kdf.Algorithm.Equal(oidPBKDF2) {
		return sealed{}, fmt.Errorf("refused a key derived with %s, not PBKDF2: %s", kdf.Algorithm, opensOnly)
	}
	var params pbkdf2Params
	if err := unmarshalWhole(kdf.Parameters.FullBytes, &params); err != nil {
		return sealed{}, fmt.Errorf("refused PBKDF2 parameters that do not parse: %w", err)
	}
	if !params.PRF.Algorithm.Equal(oidHMACSHA256) {
		return sealed{}, fmt.Errorf("refused PBKDF2 with the PRF %q, not HMAC-SHA256: %s", params.PRF.Algorithm,
			opensOnly)
	}
	if params.KeyLength != 0 && params.KeyLength != aesKeySize {
		return sealed{}, fmt.Errorf("refused a key length of %d bytes: AES-256 takes %d", params.KeyLength, aesKeySize)
	}
	if params.Iterations < 1 {
		return sealed{}, fmt.Errorf("refused PBKDF2 of %d rounds", params.Iterations)
	}
	return sealed{salt: params.Salt, iterations: params.Iterations}, nil
}

// parseScheme reads the IV of AES-256-CBC, refusing another cipher.
func parseScheme(scheme pkix.AlgorithmIdentifier) ([]byte, error) {
	if !scheme.Algorithm.Equal(oidAES256CBC) {
		return nil, fmt.Errorf("refused a key encrypted with %s, not AES-256-CBC: %s", scheme.Algorithm, opensOnly)
	}
	var iv []byte
	if err := unmarshalWhole(scheme.Parameters.FullBytes, &iv); err != nil {
		return nil, fmt.Errorf("refused an IV that does not parse: %w", err)
	}
	if len(iv) != aes.BlockSize {
		return nil, fmt.Errorf("refused an IV of %d bytes: AES-256-CBC takes %d", len(iv), aes.BlockSize)
	}
	return iv, nil
}

// unmarshalWhole parses DER into v, refusing bytes left over after it.
func unmarshalWhole(der []byte, v any) error {
	rest, err := asn1.Unmarshal(der, v)
	if err != nil {
		return err
	}
	if len(rest) > 0 {
		return fmt.Errorf("%d bytes follow the structure", len(rest))
	}
	return nil
}

// deriveKey returns the AES-256 key PBKDF2-HMAC-SHA256 derives from a passphrase under a sealing's salt and rounds.
func deriveKey(passphrase phrase, s sealed) (derivedKey, error) {
	return pbkdf2.Key(sha256.New, string(passphrase), s.salt, s.iterations, aesKeySize)
}

// mustCipher returns the AES block cipher of a key, panicking on a key no derivation here gives.
func mustCipher(key derivedKey) cipher.Block {
	block, err := aes.NewCipher(key)
	if err != nil {
		panic(err)
	}
	return block
}

// rawValue returns the DER of a value the sealing writes, as a raw ASN.1 value.
func rawValue(v any) asn1.RawValue {
	der, _ := asn1.Marshal(v)
	return asn1.RawValue{FullBytes: der}
}
