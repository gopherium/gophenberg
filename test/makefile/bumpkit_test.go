// SPDX-License-Identifier: Apache-2.0

package makefile_test

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// inRepo runs the command in the directory and returns what it printed.
func inRepo(t *testing.T, dir, name string, args ...string) (string, error) {
	t.Helper()
	command := exec.Command(name, args...)
	command.Dir = dir
	out, err := command.CombinedOutput()
	return string(out), err
}

// kitRepo returns a git repository holding the Makefile and a kit manifest at the version, tagged when asked.
func kitRepo(t *testing.T, version string, tagged bool) string {
	t.Helper()
	for _, tool := range []string{"make", "npm", "node", "git"} {
		if _, err := exec.LookPath(tool); err != nil {
			t.Skipf("%s is not on PATH, so the kit bump cannot be exercised here", tool)
		}
	}
	root := t.TempDir()
	makefile, err := os.ReadFile(filepath.Join("..", "..", "Makefile"))
	if err != nil {
		t.Fatalf("reading the Makefile: %v", err)
	}
	kit := filepath.Join(root, "sdk", "astro")
	if err := os.MkdirAll(kit, 0o755); err != nil {
		t.Fatalf("making the kit directory: %v", err)
	}
	manifest := `{"name": "@gophenberg/astro", "version": "` + version + `"}`
	for path, body := range map[string][]byte{
		filepath.Join(root, "Makefile"):    makefile,
		filepath.Join(kit, "package.json"): []byte(manifest),
	} {
		if err := os.WriteFile(path, body, 0o644); err != nil {
			t.Fatalf("writing %s: %v", path, err)
		}
	}
	steps := [][]string{
		{"init", "-q"},
		{"add", "-A"},
		{"-c", "user.name=Maria Perez", "-c", "user.email=maria@example.com", "commit", "-q", "-m", "kit"},
	}
	if tagged {
		steps = append(steps, []string{"tag", "astro@" + version})
	}
	for _, step := range steps {
		if out, err := inRepo(t, root, "git", step...); err != nil {
			t.Fatalf("git %v: %v\n%s", step, err, out)
		}
	}
	return root
}

// kitVersion returns the version the kit manifest in the repository names.
func kitVersion(t *testing.T, root string) string {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join(root, "sdk", "astro", "package.json"))
	if err != nil {
		t.Fatalf("reading the kit manifest: %v", err)
	}
	var manifest struct {
		Version string `json:"version"`
	}
	if err := json.Unmarshal(raw, &manifest); err != nil {
		t.Fatalf("decoding the kit manifest: %v", err)
	}
	return manifest.Version
}

func TestBumpKitRefusesToReplaceAVersionThatWasNeverTagged(t *testing.T) {
	t.Parallel()

	root := kitRepo(t, "0.15.0", false)

	out, err := inRepo(t, root, "make", "bump-kit", "V=0.16.0")

	if err == nil {
		t.Fatalf("make bump-kit succeeded, want it refused while astro@0.15.0 carries no tag\n%s", out)
	}
	if !strings.Contains(out, "astro@0.15.0") {
		t.Errorf("output = %q, want it to name the untagged astro@0.15.0", out)
	}
	if got := kitVersion(t, root); got != "0.15.0" {
		t.Errorf("kit version = %q, want 0.15.0 left as it was", got)
	}
}

func TestBumpKitReplacesAVersionThatWasTagged(t *testing.T) {
	t.Parallel()

	root := kitRepo(t, "0.15.0", true)

	out, err := inRepo(t, root, "make", "bump-kit", "V=0.16.0")

	if err != nil {
		t.Fatalf("make bump-kit error = %v, want the tagged version replaced\n%s", err, out)
	}
	if got := kitVersion(t, root); got != "0.16.0" {
		t.Errorf("kit version = %q, want 0.16.0", got)
	}
}
