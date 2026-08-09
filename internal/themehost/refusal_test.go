// SPDX-License-Identifier: Apache-2.0

package themehost_test

import (
	"bytes"
	"errors"
	"strings"
	"testing"

	"github.com/gopherium/gophenberg/internal/themehost"
)

func TestInstallRefusalsCarryTheReasonAnOperatorReads(t *testing.T) {
	t.Parallel()

	manifest := entry{path: "theme.json", body: `{"name":"aurora","version":"1.0.0","kit":"0.1.0"}`}
	server := entry{path: "server/entry.mjs", body: "export default {}\n"}
	client := entry{path: "client/app.css", body: "body{}\n"}
	cases := []struct {
		flaw    string
		archive []byte
		reason  string
	}{
		{
			flaw:    "carries no theme.json",
			archive: archiveOf(t, server, client),
			reason:  "the manifest is missing",
		},
		{
			flaw:    "declares another name",
			archive: validArchive(t, "driftwood"),
			reason:  "the name does not match",
		},
		{
			flaw:    "holds no server entry",
			archive: archiveOf(t, manifest, client),
			reason:  "the server entry is missing",
		},
		{
			flaw:    "holds no client directory",
			archive: archiveOf(t, manifest, server),
			reason:  "the client assets are missing",
		},
		{
			flaw:    "contains a symbolic link",
			archive: archiveOf(t, manifest, server, client, entry{path: "client/escape", body: "/etc/passwd", symlink: true}),
			reason:  "symlinks are not allowed",
		},
		{
			flaw: "unpacks to more than the size cap",
			archive: archiveOf(t, manifest, server,
				entry{path: "client/app.css", body: strings.Repeat("a", int(themehost.MaxSize)+1)}),
			reason: "the theme is too large",
		},
		{
			flaw:    "contains an entry escaping its directory",
			archive: archiveOf(t, manifest, server, client, entry{path: "../escaped.txt", body: "owned"}),
			reason:  "the archive is unsafe",
		},
		{
			flaw:    "is not an archive at all",
			archive: []byte("this is not a zip"),
			reason:  "the archive could not be read",
		},
	}
	for _, testCase := range cases {
		t.Run(testCase.flaw, func(t *testing.T) {
			t.Parallel()

			_, err := install(t, testCase.archive)

			var refusal *themehost.Refusal
			if !errors.As(err, &refusal) {
				t.Fatalf("Install() = %v, want a refusal naming the reason", err)
			}
			if refusal.Reason != testCase.reason {
				t.Errorf("Reason = %q, want %q", refusal.Reason, testCase.reason)
			}
		})
	}
}

func TestARefusalStillReportsWhatWentWrong(t *testing.T) {
	t.Parallel()

	archive := validArchive(t, "driftwood")

	_, err := install(t, archive)

	if !strings.Contains(err.Error(), "the name does not match") {
		t.Errorf("Error() = %q, want the reason in the message", err)
	}
	if !strings.Contains(err.Error(), "driftwood") {
		t.Errorf("Error() = %q, want the detail an operator debugs with", err)
	}
}

func TestAMissingManifestIsStillAThemeThatIsNotInstalled(t *testing.T) {
	t.Parallel()

	themesDir := t.TempDir()
	archive := archiveOf(t,
		entry{path: "server/entry.mjs", body: "export default {}\n"},
		entry{path: "client/app.css", body: "body{}\n"},
	)

	_, err := themehost.Install(themesDir, "aurora", bytes.NewReader(archive), int64(len(archive)))

	if !errors.Is(err, themehost.ErrNotInstalled) {
		t.Errorf("Install() = %v, want it to keep reporting a theme that is not installed", err)
	}
}
