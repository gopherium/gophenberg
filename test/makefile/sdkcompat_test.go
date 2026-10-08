// SPDX-License-Identifier: Apache-2.0

package makefile_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// baseSDK is the sdk a pull request starts from.
const baseSDK = "package sdk\n\nfunc Greet() string { return \"hello\" }\n"

// apidiffPath returns the apidiff binary the repository declares as a tool.
func apidiffPath(t *testing.T) string {
	t.Helper()
	command := exec.Command("go", "tool", "-n", "apidiff")
	command.Dir = filepath.Join("..", "..")
	command.Env = append(os.Environ(), "GOWORK=off", "GOFLAGS=")
	out, err := command.Output()
	if err != nil {
		t.Fatalf("go tool -n apidiff = %v, want the repository to declare apidiff as a tool", err)
	}
	return strings.TrimSpace(string(out))
}

// compatRepo returns a directory requiring the sdk at the version, with the base sdk, the head sdk and more files.
func compatRepo(t *testing.T, version, head string, more map[string]string) string {
	t.Helper()
	for _, tool := range []string{"make", "go"} {
		if _, err := exec.LookPath(tool); err != nil {
			t.Skipf("%s is not on PATH, so the compatibility check cannot be exercised here", tool)
		}
	}
	root := t.TempDir()
	makefile, err := os.ReadFile(filepath.Join("..", "..", "Makefile"))
	if err != nil {
		t.Fatalf("reading the Makefile: %v", err)
	}
	sdkModule := "module github.com/gopherium/gophenberg/sdk\n\ngo 1.27.1\n"
	files := map[string]string{
		"Makefile": string(makefile),
		"go.mod": "module example.com/site\n\ngo 1.27.1\n\nrequire github.com/gopherium/gophenberg/sdk " + version +
			"\n\nreplace github.com/gopherium/gophenberg/sdk => ./sdk\n",
		"sdk/go.mod":      sdkModule,
		"sdk/sdk.go":      head,
		"base/sdk/go.mod": sdkModule,
		"base/sdk/sdk.go": baseSDK,
	}
	for path, body := range more {
		files[path] = body
	}
	for path, body := range files {
		writeFile(t, filepath.Join(root, path), body)
	}
	return root
}

// compatIn runs make sdk-compat with the apidiff binary against base/sdk, labeled as asked, and returns its output.
func compatIn(t *testing.T, root string, labeled bool, apidiff string) (string, error) {
	t.Helper()
	label := "SDK_BREAK_LABELED=false"
	if labeled {
		label = "SDK_BREAK_LABELED=true"
	}
	return makeIn(t, root, false, "sdk-compat", "SDK_BASE=base/sdk", "APIDIFF="+apidiff, label)
}

func TestSDKCompatPassesAnAddition(t *testing.T) {
	t.Parallel()

	head := baseSDK + "\nfunc Wave() string { return \"wave\" }\n"

	out, err := compatIn(t, compatRepo(t, "v0.1.0", head, nil), false, apidiffPath(t))

	if err != nil || !strings.Contains(out, "keeps compatibility") {
		t.Errorf("make sdk-compat = %v with output %q, want an added function to keep compatibility", err, out)
	}
}

func TestSDKCompatRefusesABreakWithoutTheLabel(t *testing.T) {
	t.Parallel()

	out, err := compatIn(t, compatRepo(t, "v0.1.0", "package sdk\n", nil), false, apidiffPath(t))

	if err == nil || !strings.Contains(out, "Greet") || !strings.Contains(out, "v0.1.0") ||
		!strings.Contains(out, "sdk-break") {
		t.Errorf("make sdk-compat = %v with output %q, want the removed Greet refused at v0.1.0, naming the label",
			err, out)
	}
}

func TestSDKCompatRefusesABreakInASubpackage(t *testing.T) {
	t.Parallel()

	more := map[string]string{
		"base/sdk/hooks/hooks.go": "package hooks\n\nfunc Fire() {}\n",
		"sdk/hooks/hooks.go":      "package hooks\n",
	}

	out, err := compatIn(t, compatRepo(t, "v0.1.0", baseSDK, more), false, apidiffPath(t))

	if err == nil || !strings.Contains(out, "Fire") {
		t.Errorf("make sdk-compat = %v with output %q, want the removed hooks.Fire refused", err, out)
	}
}

func TestSDKCompatAllowsALabeledBreakBelowOne(t *testing.T) {
	t.Parallel()

	out, err := compatIn(t, compatRepo(t, "v0.1.0", "package sdk\n", nil), true, apidiffPath(t))

	if err != nil || !strings.Contains(out, "Greet") || !strings.Contains(out, "allowed") {
		t.Errorf("make sdk-compat = %v with output %q, want the labeled break of Greet allowed below 1.0", err, out)
	}
}

func TestSDKCompatRefusesALabeledBreakFromOne(t *testing.T) {
	t.Parallel()

	out, err := compatIn(t, compatRepo(t, "v1.0.0", "package sdk\n", nil), true, apidiffPath(t))

	if err == nil || !strings.Contains(out, "Greet") || !strings.Contains(out, "v1.0.0") {
		t.Errorf("make sdk-compat = %v with output %q, want the break of Greet refused at v1.0.0 whatever the label",
			err, out)
	}
}

func TestSDKCompatRefusesAHeadThatDoesNotLoad(t *testing.T) {
	t.Parallel()

	head := "package sdk\n\nfunc Greet() int { return \"hello\" }\n"

	out, err := compatIn(t, compatRepo(t, "v0.1.0", head, nil), true, apidiffPath(t))

	if err == nil || strings.Contains(out, "keeps compatibility") || strings.Contains(out, "allowed") {
		t.Errorf("make sdk-compat = %v with output %q, want an sdk that does not load refused, labeled or not",
			err, out)
	}
}

func TestSDKCompatRunsAnApidiffInAFolderWithASpace(t *testing.T) {
	t.Parallel()

	binary, err := os.ReadFile(apidiffPath(t))
	if err != nil {
		t.Fatalf("reading the apidiff binary: %v", err)
	}
	spaced := filepath.Join(t.TempDir(), "tool dir", "apidiff")
	writeFile(t, spaced, string(binary))
	if err := os.Chmod(spaced, 0o755); err != nil {
		t.Fatalf("making %s runnable: %v", spaced, err)
	}

	out, err := compatIn(t, compatRepo(t, "v0.1.0", baseSDK, nil), false, spaced)

	if err != nil || !strings.Contains(out, "keeps compatibility") {
		t.Errorf("make sdk-compat = %v with output %q, want apidiff found in a folder whose name holds a space", err, out)
	}
}

func TestSDKCompatAsksForTheBase(t *testing.T) {
	t.Parallel()

	root := compatRepo(t, "v0.1.0", baseSDK, nil)

	out, err := makeIn(t, root, false, "sdk-compat", "SDK_BASE=")

	if err == nil || !strings.Contains(out, "usage: make sdk-compat SDK_BASE=") {
		t.Errorf("make sdk-compat = %v with output %q, want the usage when no base is named", err, out)
	}
}
