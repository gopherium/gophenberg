// SPDX-License-Identifier: Apache-2.0

package main

import (
	"crypto"
	"crypto/ecdsa"
	"crypto/ed25519"
	"crypto/elliptic"
	"crypto/rand"
	"errors"
	"fmt"
	"io"
	"os"

	"github.com/gopherium/gophenberg/internal/signature"
)

// keyMode is the file mode of a private key file.
const keyMode = 0o600

// publicMode is the file mode of a public key file.
const publicMode = 0o644

// minIterations is the fewest PBKDF2 rounds keygen seals a key with, a security floor.
const minIterations = 600000

// curve is a kind of key keygen makes, and whether its file must be sealed.
type curve struct {
	generate func() (crypto.Signer, []byte)
	sealed   bool
}

// curves maps each curve name keygen takes to what makes a key on it.
var curves = map[string]curve{
	"ed25519": {generate: newEd25519},
	"p256":    {generate: newP256, sealed: true},
}

// keygenOptions holds the flags of one keygen run.
type keygenOptions struct {
	curve          string
	out            string
	public         string
	passphraseFile string
	iterations     int
}

// keygen makes a release or root key, writes it, and prints its Multikey and its raw public key.
func keygen(args []string, stdout, stderr io.Writer) error {
	opts, err := parseKeygen(args, stderr)
	if err != nil {
		return err
	}
	seal, err := opts.sealing()
	if err != nil {
		return err
	}
	outputs, err := openOutputs(opts, stdout)
	if err != nil {
		return err
	}
	key, raw := curves[opts.curve].generate()
	if err := outputs.write(key, seal); err != nil {
		return err
	}
	report := stdout
	if opts.out == standardStream {
		report = stderr
	}
	multikey, _ := signature.Multikey(key.Public())
	_, _ = fmt.Fprintf(report, "multikey %s\npublic-key-hex %x\n", multikey, raw)
	return nil
}

// parseKeygen reads keygen's flags and refuses a run they do not describe.
func parseKeygen(args []string, stderr io.Writer) (keygenOptions, error) {
	var opts keygenOptions
	flags := newFlagSet("keygen", stderr)
	flags.StringVar(&opts.curve, "curve", "", "the curve of the key: ed25519 for a release key, p256 for a root key")
	flags.StringVar(&opts.out, "out", "", "the file the private key is written to, or - for standard output")
	flags.StringVar(&opts.public, "public", "", "the file the public key is written to as PEM")
	flags.StringVar(&opts.passphraseFile, "passphrase-file", "", "a file whose first line seals the private key")
	flags.IntVar(&opts.iterations, "iterations", 0, "the PBKDF2 rounds that seal the private key")
	if err := parseFlags(flags, args); err != nil {
		return opts, err
	}
	return opts, opts.validate()
}

// validate refuses a curve keygen does not make, a run with no -out, and a sealing it does not allow.
func (o keygenOptions) validate() error {
	if _, known := curves[o.curve]; !known {
		return usageError{fmt.Sprintf("keygen -curve %q is not a curve it makes: give ed25519 or p256", o.curve)}
	}
	if o.out == "" {
		return usageError{"keygen needs -out, a file for the private key or - for standard output"}
	}
	return o.validateSealing()
}

// validateSealing refuses a root key with no passphrase, and rounds the floor does not allow.
func (o keygenOptions) validateSealing() error {
	if o.passphraseFile == "" {
		return o.validatePlain()
	}
	if o.iterations == 0 {
		return usageError{"keygen -passphrase-file needs -iterations, the PBKDF2 rounds that seal the key"}
	}
	if o.iterations < minIterations {
		return usageError{fmt.Sprintf("keygen -iterations %d is below the floor of %d rounds", o.iterations,
			minIterations)}
	}
	return nil
}

// validatePlain refuses a plain file for a root key, and rounds with no passphrase to seal under.
func (o keygenOptions) validatePlain() error {
	if curves[o.curve].sealed {
		return usageError{fmt.Sprintf("keygen -curve %s makes a root key, which needs -passphrase-file", o.curve)}
	}
	if o.iterations != 0 {
		return usageError{"keygen -iterations seals a key only with -passphrase-file"}
	}
	return nil
}

// sealing reads the passphrase file, when one is named, into the sealing that protects the key.
func (o keygenOptions) sealing() (sealing, error) {
	if o.passphraseFile == "" {
		return sealing{}, nil
	}
	passphrase, err := readPassphrase(o.passphraseFile)
	if err != nil {
		return sealing{}, err
	}
	return sealing{passphrase: passphrase, iterations: o.iterations}, nil
}

// keyOutputs holds where one keygen run writes its keys, and the files it opened to do so.
type keyOutputs struct {
	private io.Writer
	public  io.Writer
	opened  []*os.File
}

// openOutputs opens the private key's destination and the public key's file, refusing any file that exists.
func openOutputs(opts keygenOptions, stdout io.Writer) (keyOutputs, error) {
	outputs := keyOutputs{private: stdout, public: io.Discard}
	if opts.out != standardStream {
		file, err := createNew(opts.out, keyMode)
		if err != nil {
			return outputs, err
		}
		outputs.private, outputs.opened = file, append(outputs.opened, file)
	}
	if opts.public == "" {
		return outputs, nil
	}
	file, err := createNew(opts.public, publicMode)
	if err != nil {
		return outputs, errors.Join(err, outputs.discard())
	}
	outputs.public, outputs.opened = file, append(outputs.opened, file)
	return outputs, nil
}

// write writes the private key under the sealing and its public key, then closes every file it opened, removing
// them all when a write fails.
func (o keyOutputs) write(key crypto.Signer, s sealing) error {
	if err := errors.Join(writeKey(o.private, key, s), writePublicKey(o.public, key.Public())); err != nil {
		return errors.Join(err, o.discard())
	}
	var err error
	for _, file := range o.opened {
		err = errors.Join(err, file.Close())
	}
	return err
}

// discard closes and removes every file the outputs opened.
func (o keyOutputs) discard() error {
	var err error
	for _, file := range o.opened {
		err = errors.Join(err, file.Close(), os.Remove(file.Name()))
	}
	return err
}

// createNew creates a file with the mode, refusing one that exists.
func createNew(path string, mode os.FileMode) (*os.File, error) {
	file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, mode)
	if err != nil {
		return nil, fmt.Errorf("refused to write %s: %w", path, err)
	}
	return file, nil
}

// newEd25519 makes an Ed25519 release key and returns it with its 32 public bytes.
func newEd25519() (crypto.Signer, []byte) {
	public, private, _ := ed25519.GenerateKey(nil)
	return private, public
}

// newP256 makes a P-256 root key and returns it with its 33-byte compressed public point.
func newP256() (crypto.Signer, []byte) {
	key, _ := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	point, _ := key.PublicKey.Bytes()
	x, lastOfY := point[1:1+len(point)/2], point[len(point)-1]
	return key, append([]byte{2 | lastOfY&1}, x...)
}
