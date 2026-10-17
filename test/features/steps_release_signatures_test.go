// SPDX-License-Identifier: Apache-2.0

package features_test

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/cucumber/godog"

	"github.com/gopherium/gophenberg/internal/signature"
)

// liveSignersPath names the test data that holds the live FAIR signers.
const liveSignersPath = "../../internal/signature/testdata/signers.json"

// signingKey keys the signing scenario in a scenario's context.
type signingKey struct{}

// signingScenario holds the key, the file and the signature one release signatures scenario works on.
type signingScenario struct {
	public    ed25519.PublicKey
	private   ed25519.PrivateKey
	file      []byte
	digest    [signature.DigestSize]byte
	signature string
	checked   error
}

// liveSigner is one live FAIR signer as the test data holds it, in hexadecimal and base64url.
type liveSigner struct {
	Name      string `json:"name"`
	PublicKey string `json:"public_key"`
	Digest    string `json:"digest"`
	Signature string `json:"signature"`
}

// provisionSigning gives a scenario an empty signing scenario.
func provisionSigning(ctx context.Context, _ *godog.Scenario) (context.Context, error) {
	return context.WithValue(ctx, signingKey{}, &signingScenario{}), nil
}

// signingOf returns the signing scenario the context carries.
func signingOf(ctx context.Context) (*signingScenario, error) {
	s, ok := ctx.Value(signingKey{}).(*signingScenario)
	if !ok {
		return nil, errors.New("the scenario has no signing state")
	}
	return s, nil
}

// aFreshReleaseKey makes a new Ed25519 release key for the scenario.
func aFreshReleaseKey(ctx context.Context) error {
	s, err := signingOf(ctx)
	if err != nil {
		return err
	}
	s.public, s.private, err = ed25519.GenerateKey(nil)
	return err
}

// aReleaseFileHolding gives the scenario a release file with the text as its bytes.
func aReleaseFileHolding(ctx context.Context, text string) error {
	s, err := signingOf(ctx)
	if err != nil {
		return err
	}
	s.file = []byte(text)
	return nil
}

// theReleaseKeySignsTheFile signs the digest of the scenario's file with its release key.
func theReleaseKeySignsTheFile(ctx context.Context) error {
	s, err := signingOf(ctx)
	if err != nil {
		return err
	}
	digest, err := signature.Digest(bytes.NewReader(s.file))
	if err != nil {
		return err
	}
	s.signature = signature.Sign(s.private, digest)
	return nil
}

// oneByteOfTheFileChanges flips the lowest bit of the file's first byte.
func oneByteOfTheFileChanges(ctx context.Context) error {
	s, err := signingOf(ctx)
	if err != nil {
		return err
	}
	s.file[0] ^= 1
	return nil
}

// checkTheFile checks the signature against the digest of the file as it now stands.
func checkTheFile(s *signingScenario) error {
	digest, err := signature.Digest(bytes.NewReader(s.file))
	if err != nil {
		return err
	}
	return signature.Verify(s.public, digest, s.signature)
}

// theSignatureHoldsForTheFileUnderTheReleaseKey asserts the file's signature verifies under the release key.
func theSignatureHoldsForTheFileUnderTheReleaseKey(ctx context.Context) error {
	s, err := signingOf(ctx)
	if err != nil {
		return err
	}
	if err := checkTheFile(s); err != nil {
		return fmt.Errorf("the signature does not hold: %w", err)
	}
	return nil
}

// theCheckRefusesTheFile asserts the signature no longer holds for the changed file.
func theCheckRefusesTheFile(ctx context.Context) error {
	s, err := signingOf(ctx)
	if err != nil {
		return err
	}
	if err := checkTheFile(s); !errors.Is(err, signature.ErrMismatch) {
		return fmt.Errorf("checking the changed file gave %v, want %v", err, signature.ErrMismatch)
	}
	return nil
}

// theLiveFAIRSigner loads the named live signer's key, digest and signature into the scenario.
func theLiveFAIRSigner(ctx context.Context, name string) error {
	s, err := signingOf(ctx)
	if err != nil {
		return err
	}
	signer, err := liveSignerNamed(name)
	if err != nil {
		return err
	}
	key, err := hex.DecodeString(signer.PublicKey)
	if err != nil {
		return fmt.Errorf("reading the key of %s: %w", name, err)
	}
	digest, err := hex.DecodeString(signer.Digest)
	if err != nil {
		return fmt.Errorf("reading the digest of %s: %w", name, err)
	}
	s.public = key
	copy(s.digest[:], digest)
	s.signature = signer.Signature
	return nil
}

// liveSignerNamed reads the test data and returns the live signer under the name.
func liveSignerNamed(name string) (liveSigner, error) {
	raw, err := os.ReadFile(liveSignersPath)
	if err != nil {
		return liveSigner{}, err
	}
	var data struct {
		Live []liveSigner `json:"live"`
	}
	if err := json.Unmarshal(raw, &data); err != nil {
		return liveSigner{}, fmt.Errorf("reading %s: %w", liveSignersPath, err)
	}
	for _, signer := range data.Live {
		if signer.Name == name {
			return signer, nil
		}
	}
	return liveSigner{}, fmt.Errorf("the test data holds no live signer named %q", name)
}

// itsSignatureIsRewrittenInStandardBase64WithPadding writes the scenario's signature in standard base64.
func itsSignatureIsRewrittenInStandardBase64WithPadding(ctx context.Context) error {
	s, err := signingOf(ctx)
	if err != nil {
		return err
	}
	raw, err := base64.RawURLEncoding.DecodeString(s.signature)
	if err != nil {
		return fmt.Errorf("reading the live signature: %w", err)
	}
	s.signature = base64.StdEncoding.EncodeToString(raw)
	return nil
}

// itsSignatureIsCheckedAgainstItsDigestUnderItsKey checks the scenario's signature and keeps the answer.
func itsSignatureIsCheckedAgainstItsDigestUnderItsKey(ctx context.Context) error {
	s, err := signingOf(ctx)
	if err != nil {
		return err
	}
	s.checked = signature.Verify(s.public, s.digest, s.signature)
	return nil
}

// theCheckHolds asserts the kept check found the signature good.
func theCheckHolds(ctx context.Context) error {
	s, err := signingOf(ctx)
	if err != nil {
		return err
	}
	if s.checked != nil {
		return fmt.Errorf("the check refused the signature: %w", s.checked)
	}
	return nil
}

// theCheckRefusesTheSignatureNamingTheEncoding asserts the kept check refused the encoding, not the bytes.
func theCheckRefusesTheSignatureNamingTheEncoding(ctx context.Context) error {
	s, err := signingOf(ctx)
	if err != nil {
		return err
	}
	if s.checked == nil || errors.Is(s.checked, signature.ErrMismatch) {
		return fmt.Errorf("the check gave %v, want the encoding refused", s.checked)
	}
	if !strings.Contains(s.checked.Error(), "base64url without padding") {
		return fmt.Errorf("the error %q names no base64url without padding", s.checked)
	}
	return nil
}

// initializeReleaseSignatures registers the steps of the release signatures feature.
func initializeReleaseSignatures(sc *godog.ScenarioContext) {
	sc.Before(provisionSigning)
	sc.Given(`^a fresh release key$`, aFreshReleaseKey)
	sc.Given(`^a release file holding "([^"]*)"$`, aReleaseFileHolding)
	sc.Given(`^the live FAIR signer "([^"]*)"$`, theLiveFAIRSigner)
	sc.Step(`^the release key signs the file$`, theReleaseKeySignsTheFile)
	sc.When(`^one byte of the file changes$`, oneByteOfTheFileChanges)
	sc.When(`^its signature is rewritten in standard base64 with padding$`,
		itsSignatureIsRewrittenInStandardBase64WithPadding)
	sc.When(`^its signature is checked against its digest under its key$`,
		itsSignatureIsCheckedAgainstItsDigestUnderItsKey)
	sc.Then(`^the signature holds for the file under the release key$`, theSignatureHoldsForTheFileUnderTheReleaseKey)
	sc.Then(`^the check refuses the file$`, theCheckRefusesTheFile)
	sc.Then(`^the check holds$`, theCheckHolds)
	sc.Then(`^the check refuses the signature, naming base64url without padding$`,
		theCheckRefusesTheSignatureNamingTheEncoding)
	initializeReleaseFiles(sc)
}
