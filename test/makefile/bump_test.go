// SPDX-License-Identifier: Apache-2.0

package makefile_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// author is the identity the fixture repositories commit and tag as.
var author = []string{"-c", "user.name=Maria Perez", "-c", "user.email=maria@example.com"}

// tagStyles are the git steps that tag sdk/v0.1.0, as a bare name or as an annotated tag.
var tagStyles = map[string][]string{
	"lightweight": {"tag", "sdk/v0.1.0"},
	"annotated":   append(append([]string{}, author...), "tag", "-a", "-m", "sdk v0.1.0", "sdk/v0.1.0"),
}

// sdkChanges are the ways the sdk moves on after its tag, each applied to a repository root.
var sdkChanges = map[string]func(t *testing.T, root string){
	"in a commit": func(t *testing.T, root string) {
		writeFile(t, filepath.Join(root, "sdk", "sdk.go"), "package sdk\n\nconst Added = true\n")
		runGit(t, root, commitStep())
	},
	"in the working files": func(t *testing.T, root string) {
		writeFile(t, filepath.Join(root, "sdk", "sdk.go"), "package sdk\n\nconst Added = true\n")
	},
	"in a new untracked file": func(t *testing.T, root string) {
		writeFile(t, filepath.Join(root, "sdk", "added.go"), "package sdk\n\nconst Added = true\n")
	},
}

// commitStep returns the git step that commits every tracked change.
func commitStep() []string {
	return append(append([]string{}, author...), "commit", "-q", "-am", "sdk")
}

// sdkRepo returns a git repository requiring the sdk at v0.1.0, tagged by the step when one is given.
func sdkRepo(t *testing.T, tag []string) string {
	t.Helper()
	for _, tool := range []string{"make", "go", "git"} {
		if _, err := exec.LookPath(tool); err != nil {
			t.Skipf("%s is not on PATH, so the release bump cannot be exercised here", tool)
		}
	}
	root := t.TempDir()
	makefile, err := os.ReadFile(filepath.Join("..", "..", "Makefile"))
	if err != nil {
		t.Fatalf("reading the Makefile: %v", err)
	}
	files := map[string]string{
		"Makefile": string(makefile),
		"go.mod": "module example.com/site\n\ngo 1.27.1\n\nrequire github.com/gopherium/gophenberg/sdk v0.1.0\n\n" +
			"replace github.com/gopherium/gophenberg/sdk => ./sdk\n",
		"sdk/go.mod":               "module github.com/gopherium/gophenberg/sdk\n\ngo 1.27.1\n",
		"sdk/sdk.go":               "package sdk\n",
		"internal/version/VERSION": "0.18.0\n",
	}
	for path, body := range files {
		writeFile(t, filepath.Join(root, path), body)
	}
	runGit(t, root, []string{"init", "-q"}, []string{"add", "-A"}, commitStep())
	if tag != nil {
		runGit(t, root, tag)
	}
	return root
}

// writeFile writes body to path, making its folder first.
func writeFile(t *testing.T, path, body string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("making the folder of %s: %v", path, err)
	}
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatalf("writing %s: %v", path, err)
	}
}

// runGit runs each git step in the repository and stops the test at the first that fails.
func runGit(t *testing.T, root string, steps ...[]string) {
	t.Helper()
	for _, step := range steps {
		if out, err := inRepo(t, root, "git", step...); err != nil {
			t.Fatalf("git %v: %v\n%s", step, err, out)
		}
	}
}

// coreVersion returns the version the core's VERSION file in the repository names.
func coreVersion(t *testing.T, root string) string {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join(root, "internal", "version", "VERSION"))
	if err != nil {
		t.Fatalf("reading the core version: %v", err)
	}
	return strings.TrimSpace(string(raw))
}

func TestBumpRefusesWhileTheRequiredSDKVersionCarriesNoTag(t *testing.T) {
	t.Parallel()

	root := sdkRepo(t, nil)

	out, err := makeIn(t, root, false, "bump", "V=0.19.0")

	if err == nil || !strings.Contains(out, "sdk/v0.1.0") {
		t.Errorf("make bump = %v with output %q, want it refused, naming the untagged sdk/v0.1.0", err, out)
	}
	if got := coreVersion(t, root); got != "0.18.0" {
		t.Errorf("core version = %q, want 0.18.0 left as it was", got)
	}
}

func TestBumpRefusesWhenTheSDKChangedSinceItsTag(t *testing.T) {
	t.Parallel()

	for style, tag := range tagStyles {
		for change, apply := range sdkChanges {
			t.Run(style+" tag, changed "+change, func(t *testing.T) {
				t.Parallel()
				root := sdkRepo(t, tag)
				apply(t, root)

				out, err := makeIn(t, root, false, "bump", "V=0.19.0")

				if err == nil || !strings.Contains(out, "changed since sdk/v0.1.0") {
					t.Errorf("make bump = %v with output %q, want it refused, naming the sdk changed since sdk/v0.1.0",
						err, out)
				}
				if got := coreVersion(t, root); got != "0.18.0" {
					t.Errorf("core version = %q, want 0.18.0 left as it was", got)
				}
			})
		}
	}
}

func TestBumpWritesTheVersionWhenTheSDKMatchesItsTag(t *testing.T) {
	t.Parallel()

	for style, tag := range tagStyles {
		t.Run(style+" tag", func(t *testing.T) {
			t.Parallel()
			root := sdkRepo(t, tag)

			out, err := makeIn(t, root, false, "bump", "V=0.19.0")

			if err != nil {
				t.Fatalf("make bump = %v with output %q, want the version written", err, out)
			}
			if got := coreVersion(t, root); got != "0.19.0" {
				t.Errorf("core version = %q, want 0.19.0", got)
			}
		})
	}
}
