// SPDX-License-Identifier: Apache-2.0

package sdk

import (
	"errors"
	"strings"
	"testing"
)

func TestMisuseMarksAMisusedCommandLine(t *testing.T) {
	t.Parallel()

	cause := errors.New("feed:refresh takes no arguments")

	err := Misuse(cause)

	if !errors.Is(err, ErrMisused) {
		t.Errorf("Misuse() = %v, want it marked as a misused command line", err)
	}
	if !errors.Is(err, cause) || err.Error() != cause.Error() {
		t.Errorf("Misuse() = %v, want the cause kept and its text unchanged", err)
	}
}

func TestMisuseOfNoErrorIsNoError(t *testing.T) {
	t.Parallel()

	if err := Misuse(nil); err != nil {
		t.Errorf("Misuse(nil) = %v, want nil", err)
	}
}

func TestEncodeWritesOneIndentedDocument(t *testing.T) {
	t.Parallel()

	var stdout strings.Builder
	call := Call{Stdout: &stdout}

	err := call.Encode(map[string]any{"title": "Tom & Jerry <live>", "seats": 3})

	want := "{\n  \"seats\": 3,\n  \"title\": \"Tom & Jerry <live>\"\n}\n"
	if err != nil || stdout.String() != want {
		t.Errorf("Encode() = %v and wrote %q, want nil and %q", err, stdout.String(), want)
	}
}

func TestAPlainErrorIsNoMisuse(t *testing.T) {
	t.Parallel()

	if errors.Is(errors.New("the feed could not be read"), ErrMisused) {
		t.Error("a plain error matches ErrMisused, want only a marked one to")
	}
}
