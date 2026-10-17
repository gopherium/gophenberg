// SPDX-License-Identifier: Apache-2.0

//go:build !embedweb

package webui_test

import (
	"io/fs"
	"strings"
	"testing"

	"github.com/gopherium/gophenberg/internal/webui"
)

func TestFSServesThePlaceholderPage(t *testing.T) {
	t.Parallel()

	page, err := fs.ReadFile(webui.FS(), "index.html")

	if err != nil {
		t.Fatalf("reading index.html from the admin files: %v", err)
	}
	for _, want := range []string{"carries no admin app", "make build-release", "GOPHENBERG_WEB_DIR"} {
		if !strings.Contains(string(page), want) {
			t.Errorf("index.html = %q, want the placeholder page naming %q", page, want)
		}
	}
	if strings.Contains(string(page), "<script") {
		t.Errorf("index.html = %q, want a page that runs no script", page)
	}
}
