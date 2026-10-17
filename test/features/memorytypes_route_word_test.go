package features_test

import (
	"testing"
	"time"
)

func TestMemoryTypesHandRootOverUsesTypeKeyForNonLatinLabel(t *testing.T) {
	types := newMemoryTypes(nil)
	types.types[0].PluralLabel = "Страницы"

	carried, err := types.handRootOver(time.Now().UTC())
	if err != nil {
		t.Fatal(err)
	}

	want := types.types[0].Key
	if got := types.types[0].RouteWord; got != want {
		t.Errorf("handRootOver route word = %q, want type key %q", got, want)
	}
	if carried.now != want {
		t.Errorf("handRootOver carried route word = %q, want type key %q", carried.now, want)
	}
}
