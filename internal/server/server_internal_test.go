// SPDX-License-Identifier: Apache-2.0

package server

import (
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/gopherium/gophenberg/internal/content"
	"github.com/gopherium/gophenberg/internal/themehost"
)

func TestContentAPIGenerationReadsTheMajorOfTheNewestServedKit(t *testing.T) {
	t.Parallel()

	newest := themehost.NewestKit()

	got := contentAPIGeneration()

	if !strings.HasPrefix(newest, strconv.Itoa(got)+".") {
		t.Errorf("contentAPIGeneration() = %d, want the major of the newest served kit %q", got, newest)
	}
}

func TestEveryServedKitCarriesANumericMajor(t *testing.T) {
	t.Parallel()

	for _, served := range themehost.ServedKits() {
		major, _, _ := strings.Cut(served, ".")

		if _, err := strconv.Atoi(major); err != nil {
			t.Errorf("Atoi(the major of %q) = %v, want a number contentAPIGeneration can report", served, err)
		}
	}
}

func TestSameRelationsReadsTwoSetsOfTargets(t *testing.T) {
	t.Parallel()

	oneTarget, otherTarget := uuid.New(), uuid.New()

	for _, testCase := range []struct {
		name  string
		held  content.Relations
		asked content.Relations
		want  bool
	}{
		{name: "two empty sets", held: content.Relations{}, asked: content.Relations{}, want: true},
		{
			name:  "the same key holding the same target",
			held:  content.Relations{"author": {oneTarget}},
			asked: content.Relations{"author": {oneTarget}},
			want:  true,
		},
		{
			name:  "the same length holding a different target",
			held:  content.Relations{"author": {oneTarget}},
			asked: content.Relations{"author": {otherTarget}},
			want:  false,
		},
		{
			name:  "the same length holding the same targets in a different order",
			held:  content.Relations{"author": {oneTarget, otherTarget}},
			asked: content.Relations{"author": {otherTarget, oneTarget}},
			want:  false,
		},
		{
			name:  "the same length under a different key",
			held:  content.Relations{"author": {oneTarget}},
			asked: content.Relations{"editor": {oneTarget}},
			want:  false,
		},
		{
			name:  "the same key holding one target against two",
			held:  content.Relations{"author": {oneTarget}},
			asked: content.Relations{"author": {oneTarget, otherTarget}},
			want:  false,
		},
		{
			name:  "a different number of keys",
			held:  content.Relations{"author": {oneTarget}},
			asked: content.Relations{"author": {oneTarget}, "editor": {otherTarget}},
			want:  false,
		},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			if got := sameRelations(testCase.held, testCase.asked); got != testCase.want {
				t.Errorf("sameRelations(%v, %v) = %t, want %t", testCase.held, testCase.asked, got, testCase.want)
			}
		})
	}
}

// themeAnswer is the body only the theme upstream writes.
const themeAnswer = "answered by the theme"

// rendererAnswer is the body only the built-in renderer writes.
const rendererAnswer = "answered by the renderer"

// readyTheme is a theme reporting itself ready at a fixed address.
type readyTheme struct {
	target string
}

// Healthy reports whether the ready theme is serving.
func (r readyTheme) Healthy() bool { return true }

// Target returns the address the ready theme serves on.
func (r readyTheme) Target() string { return r.target }

// withheldHeaders returns a theme upstream holding its headers back for the given delay.
func withheldHeaders(t *testing.T, delay time.Duration) *httptest.Server {
	t.Helper()

	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		time.Sleep(delay)
		_, _ = w.Write([]byte(themeAnswer))
	}))
	t.Cleanup(upstream.Close)
	return upstream
}

// fallbackRenderer returns the built-in renderer standing behind the theme.
func fallbackRenderer() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(rendererAnswer))
	})
}

func TestThemedSiteWaitsTheDefaultWhenTheWaitArrivesAsZero(t *testing.T) {
	t.Parallel()

	const silence = 200 * time.Millisecond

	theme := readyTheme{target: withheldHeaders(t, silence).URL}
	renderer := fallbackRenderer()

	defaulted := httptest.NewRecorder()
	themedSite(theme, renderer, 0).ServeHTTP(defaulted, httptest.NewRequest(http.MethodGet, "/", nil))
	impatient := httptest.NewRecorder()
	themedSite(theme, renderer, time.Millisecond).ServeHTTP(impatient, httptest.NewRequest(http.MethodGet, "/", nil))

	if got := defaulted.Body.String(); got != themeAnswer {
		t.Errorf("themedSite(theme, renderer, 0) body = %q, want %q after %v of silence", got, themeAnswer, silence)
	}
	if got := impatient.Body.String(); got != rendererAnswer {
		t.Errorf("themedSite(theme, renderer, 1ms) body = %q, want %q", got, rendererAnswer)
	}
}
