// SPDX-License-Identifier: Apache-2.0

//go:build embedweb

package webui_test

import (
	"io/fs"
	"path"
	"regexp"
	"strings"
	"testing"

	"github.com/gopherium/gophenberg/internal/webui"
)

// adminScript matches the module script a built admin page loads from the admin's assets.
var adminScript = regexp.MustCompile(`<script type="module"[^>]* src="/admin/assets/[^"]+\.js"`)

// hashedName matches a file name carrying the content hash the admin build appends.
var hashedName = regexp.MustCompile(`-[A-Za-z0-9_-]{8}\.[A-Za-z0-9]+$`)

func TestFSCarriesTheBuiltAdmin(t *testing.T) {
	t.Parallel()

	admin := webui.FS()
	page, err := fs.ReadFile(admin, "index.html")

	if err != nil {
		t.Fatalf("reading index.html from the built admin: %v", err)
	}
	if !adminScript.Match(page) {
		t.Errorf("index.html loads no script under /admin/assets/, want the built admin page")
	}
	if strings.Contains(string(page), "carries no admin app") {
		t.Errorf("index.html is the placeholder page, want the built admin page")
	}
	for _, name := range []string{"gophenberg/blocks.css", "gophenberg/presets.css"} {
		if _, err := fs.Stat(admin, name); err != nil {
			t.Errorf("the built admin holds no %s: %v", name, err)
		}
	}
}

func TestFSCarriesOnlyHashedAdminAssets(t *testing.T) {
	t.Parallel()

	assets, err := fs.Glob(webui.FS(), "assets/*")

	if err != nil || len(assets) == 0 {
		t.Fatalf("listing the built admin's assets = %d files, %v, want the files the admin build wrote", len(assets),
			err)
	}
	for _, name := range assets {
		if !hashedName.MatchString(path.Base(name)) {
			t.Errorf("%s carries no content hash, so a year in a browser cache would keep a stale copy", name)
		}
	}
}
