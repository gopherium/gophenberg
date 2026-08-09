// SPDX-License-Identifier: Apache-2.0

package main

import (
	"context"
	"errors"
	"testing"
)

// storedChoice answers with a fixed value and error, standing in for the store.
type storedChoice struct {
	value string
	err   error
}

// Get returns the stored theme choice.
func (s storedChoice) Get(context.Context, string) (string, error) {
	return s.value, s.err
}

func TestThemeChoicePrefersTheOperatorPin(t *testing.T) {
	choice, err := themeChoice(t.Context(), "driftwood", storedChoice{value: "aurora"})

	if err != nil {
		t.Fatalf("resolve: %v", err)
	}
	if choice.name != "driftwood" {
		t.Errorf("got %q, want the pinned theme", choice.name)
	}
	if !choice.pinned {
		t.Error("want the choice reported as pinned")
	}
}

func TestThemeChoiceReadsTheStoreWhenUnpinned(t *testing.T) {
	choice, err := themeChoice(t.Context(), "", storedChoice{value: "aurora"})

	if err != nil {
		t.Fatalf("resolve: %v", err)
	}
	if choice.name != "aurora" {
		t.Errorf("got %q, want the stored theme", choice.name)
	}
	if choice.pinned {
		t.Error("want the choice reported as unpinned")
	}
}

func TestThemeChoiceLeavesTheRendererWhenNothingIsChosen(t *testing.T) {
	choice, err := themeChoice(t.Context(), "", storedChoice{})

	if err != nil {
		t.Fatalf("resolve: %v", err)
	}
	if choice.name != "" {
		t.Errorf("got %q, want no theme", choice.name)
	}
}

func TestThemeChoiceDoesNotReadTheStoreWhenPinned(t *testing.T) {
	choice, err := themeChoice(t.Context(), "driftwood", storedChoice{err: errors.New("unreachable")})

	if err != nil {
		t.Fatalf("want the pin to answer without the store, got %v", err)
	}
	if choice.name != "driftwood" {
		t.Errorf("got %q, want the pinned theme", choice.name)
	}
}

func TestThemeChoiceReportsAStoreFailure(t *testing.T) {
	_, err := themeChoice(t.Context(), "", storedChoice{err: errors.New("database down")})

	if err == nil {
		t.Fatal("want the store failure reported")
	}
}
