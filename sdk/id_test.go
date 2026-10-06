// SPDX-License-Identifier: Apache-2.0

package sdk

import (
	"encoding/json"
	"testing"
)

func TestIDPrintsTheCanonicalForm(t *testing.T) {
	t.Parallel()

	id := ID{0x01, 0x9f, 0xb0, 0x00, 0x00, 0x00, 0x70, 0x00, 0x80, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x01}

	if got := id.String(); got != "019fb000-0000-7000-8000-000000000001" {
		t.Errorf("String() = %q, want the 36 character lowercase form with hyphens", got)
	}
}

func TestIDPrintsEveryHexDigit(t *testing.T) {
	t.Parallel()

	id := ID{0xab, 0xcd, 0xef, 0x01, 0x23, 0x45, 0x67, 0x89, 0xfe, 0xdc, 0xba, 0x98, 0x76, 0x54, 0x32, 0x10}

	if got := id.String(); got != "abcdef01-2345-6789-fedc-ba9876543210" {
		t.Errorf("String() = %q, want every byte as two lowercase hex digits", got)
	}
}

func TestIDEncodesAsItsCanonicalText(t *testing.T) {
	t.Parallel()

	item := Item{ID: ID{0x01, 0x9f, 0xb0, 0x00, 0x00, 0x00, 0x70, 0x00, 0x80, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x01}}

	encoded, err := json.Marshal(map[string]any{"id": item.ID, "keyed": map[ID]int{item.ID: 1}})

	want := `{"id":"019fb000-0000-7000-8000-000000000001","keyed":{"019fb000-0000-7000-8000-000000000001":1}}`
	if err != nil || string(encoded) != want {
		t.Errorf("Marshal() = %s, %v, want the id as its canonical text, as a value and as a key", encoded, err)
	}
}

func TestIDReadsItsCanonicalText(t *testing.T) {
	t.Parallel()

	var read struct{ ID ID }

	err := json.Unmarshal([]byte(`{"ID":"ABCDEF01-2345-6789-FEDC-BA9876543210"}`), &read)

	want := ID{0xab, 0xcd, 0xef, 0x01, 0x23, 0x45, 0x67, 0x89, 0xfe, 0xdc, 0xba, 0x98, 0x76, 0x54, 0x32, 0x10}
	if err != nil || read.ID != want {
		t.Errorf("Unmarshal() = %v, %v, want %v read in either case", read.ID, err, want)
	}
}

func TestIDRefusesTextInAnotherForm(t *testing.T) {
	t.Parallel()

	for _, text := range []string{
		"", "019fb000000070008000000000000001", "019fb000-0000-7000-8000-00000000000",
		"019fb000-0000-7000-8000-0000000000011", "019fb0000-000-7000-8000-000000000001",
		"019fb000-0000-7000-8000-00000000000g", "{019fb000-0000-7000-8000-000000000001}",
	} {
		t.Run(text, func(t *testing.T) {
			t.Parallel()

			held := ID{7}

			err := held.UnmarshalText([]byte(text))

			if err == nil || held != (ID{7}) {
				t.Errorf("UnmarshalText(%q) = %v and left %v, want a refusal and the id unchanged", text, err, held)
			}
		})
	}
}

func TestTheZeroIDPrintsAllZeros(t *testing.T) {
	t.Parallel()

	if got := (ID{}).String(); got != "00000000-0000-0000-0000-000000000000" {
		t.Errorf("String() = %q, want the nil form", got)
	}
}
