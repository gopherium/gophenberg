// SPDX-License-Identifier: Apache-2.0

package signature_test

import (
	"errors"
	"regexp"
	"strings"
	"testing"
	"testing/iotest"

	"github.com/gopherium/gophenberg/internal/signature"
)

func TestChecksumWritesSha256Hex(t *testing.T) {
	t.Parallel()

	seed := loadVectors(t).Seed

	got, err := signature.Checksum(strings.NewReader(seed.Message))

	if err != nil {
		t.Fatalf("Checksum: %v", err)
	}
	if got != seed.Checksum {
		t.Errorf("Checksum = %q, want %q", got, seed.Checksum)
	}
	if !regexp.MustCompile(`^sha256:[0-9a-f]{64}$`).MatchString(got) {
		t.Errorf("Checksum = %q, want sha256: and 64 lowercase hexadecimal characters", got)
	}
}

func TestChecksumReportsAReaderThatFails(t *testing.T) {
	t.Parallel()

	failure := errors.New("the disk went away")

	_, err := signature.Checksum(iotest.ErrReader(failure))

	if !errors.Is(err, failure) {
		t.Errorf("Checksum over a failing reader = %v, want %v", err, failure)
	}
}
