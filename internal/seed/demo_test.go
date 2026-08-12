// SPDX-License-Identifier: Apache-2.0

package seed

import (
	"strings"
	"testing"

	"github.com/gopherium/gophenberg/internal/content"
	"github.com/gopherium/gophenberg/internal/publichtml"
)

// publishedDemoPosts returns the scripted posts a visitor can read.
func publishedDemoPosts() []demoPost {
	published := make([]demoPost, 0, len(demoPosts()))
	for _, scripted := range demoPosts() {
		if scripted.status == content.StatusPublished {
			published = append(published, scripted)
		}
	}
	return published
}

func TestTheDemoShowsEveryBlockTheRendererDraws(t *testing.T) {
	t.Parallel()

	var content strings.Builder
	for _, scripted := range publishedDemoPosts() {
		content.WriteString(scripted.content)
	}
	shown := content.String()

	for _, block := range []string{
		"wp:heading", "wp:paragraph", "wp:quote", "wp:list", "wp:list-item",
		"wp:group", "wp:columns", "wp:column", "wp:image",
	} {
		if !strings.Contains(shown, "<!-- "+block+" ") && !strings.Contains(shown, "<!-- "+block+" -->") {
			t.Errorf("the published demo carries no %s block, which the renderer and the theme both draw", block)
		}
	}
}

func TestTheDemoDrawsAnImageFromAnAddressRatherThanAnUpload(t *testing.T) {
	t.Parallel()

	var content strings.Builder
	for _, scripted := range publishedDemoPosts() {
		content.WriteString(scripted.content)
	}

	if !strings.Contains(content.String(), `src="https://`) {
		t.Error("the demo image is not sourced from an address, which is what this cycle serves")
	}
}

func TestTheDemoSurvivesTheSanitizerWithoutLoss(t *testing.T) {
	t.Parallel()

	for _, scripted := range demoPosts() {
		t.Run(scripted.title, func(t *testing.T) {
			t.Parallel()

			if got := publichtml.Sanitize(scripted.content); got != scripted.content {
				t.Errorf("Sanitize() changed the demo content.\n got: %s\nwant: %s", got, scripted.content)
			}
		})
	}
}

func TestTheDemoKeepsTheAddressesTheEndToEndTestsVisit(t *testing.T) {
	t.Parallel()

	titles := make(map[string]bool, len(demoPosts()))
	for _, scripted := range demoPosts() {
		titles[scripted.title] = true
	}

	for _, want := range []string{"Welcome to Gophenberg", "Writing with Blocks"} {
		if !titles[want] {
			t.Errorf("the demo lost %q, whose slug the end to end tests visit", want)
		}
	}
}

func TestTheDemoPublishesMoreThanItDidBefore(t *testing.T) {
	t.Parallel()

	if got := len(publishedDemoPosts()); got < 3 {
		t.Errorf("published demo posts = %d, want the third this item adds", got)
	}
}

func TestEveryDemoPostIsScriptedOnce(t *testing.T) {
	t.Parallel()

	seen := make(map[string]bool, len(demoPosts()))
	for _, scripted := range demoPosts() {
		if seen[scripted.id] {
			t.Errorf("post id %s is scripted twice, so a re-seed would not be idempotent", scripted.id)
		}
		seen[scripted.id] = true
		if seen[scripted.title] {
			t.Errorf("title %q is scripted twice, so the two would fight over one slug", scripted.title)
		}
		seen[scripted.title] = true
	}
}
