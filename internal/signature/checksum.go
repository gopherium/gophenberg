// SPDX-License-Identifier: Apache-2.0

package signature

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
)

// checksumPrefix names the hash a checksum carries, as FAIR records write it.
const checksumPrefix = "sha256:"

// Checksum returns the checksum of what r holds: sha256: and 64 lowercase hexadecimal characters.
func Checksum(r io.Reader) (string, error) {
	hash := sha256.New()
	if _, err := io.Copy(hash, r); err != nil {
		return "", fmt.Errorf("signature: reading the bytes to checksum: %w", err)
	}
	return checksumPrefix + hex.EncodeToString(hash.Sum(nil)), nil
}
