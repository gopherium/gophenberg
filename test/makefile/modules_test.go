// SPDX-License-Identifier: Apache-2.0

package makefile_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// moduleRepo returns a directory holding the Makefile, a root module and an sdk module whose test passes or fails.
func moduleRepo(t *testing.T, sdkPasses bool) string {
	t.Helper()
	for _, tool := range []string{"make", "go"} {
		if _, err := exec.LookPath(tool); err != nil {
			t.Skipf("%s is not on PATH, so the module runs cannot be exercised here", tool)
		}
	}
	root := t.TempDir()
	makefile, err := os.ReadFile(filepath.Join("..", "..", "Makefile"))
	if err != nil {
		t.Fatalf("reading the Makefile: %v", err)
	}
	verdict := `t.Error("the sdk module ran and failed")`
	if sdkPasses {
		verdict = `t.Log("the sdk module ran")`
	}
	if err := os.MkdirAll(filepath.Join(root, "sdk"), 0o755); err != nil {
		t.Fatalf("making the sdk directory: %v", err)
	}
	for path, body := range map[string]string{
		"Makefile":             string(makefile),
		"go.mod":               "module example.com/site\n\ngo 1.27.1\n",
		"site_test.go":         "package site\n\nimport \"testing\"\n\nfunc TestSite(t *testing.T) {}\n",
		"sdk/go.mod":           "module example.com/site/sdk\n\ngo 1.27.1\n",
		"sdk/contract_test.go": "package sdk\n\nimport \"testing\"\n\nfunc TestContract(t *testing.T) { " + verdict + " }\n",
	} {
		if err := os.WriteFile(filepath.Join(root, path), []byte(body), 0o644); err != nil {
			t.Fatalf("writing %s: %v", path, err)
		}
	}
	return root
}

// makeIn runs the make target in the directory with no workspace file and returns what it printed.
func makeIn(t *testing.T, dir, target string) (string, error) {
	t.Helper()
	command := exec.Command("make", target)
	command.Dir = dir
	command.Env = append(os.Environ(), "GOWORK=off", "GOFLAGS=")
	out, err := command.CombinedOutput()
	return string(out), err
}

func TestMakeTestRunsTheSDKModule(t *testing.T) {
	t.Parallel()

	for _, target := range []string{"test", "test-race"} {
		t.Run(target, func(t *testing.T) {
			t.Parallel()

			out, err := makeIn(t, moduleRepo(t, true), target)

			if err != nil || !strings.Contains(out, "example.com/site/sdk") {
				t.Errorf("make %s = %v with output %q, want the sdk module's tests run and passed", target, err, out)
			}
		})
	}
}

func TestMakeTestFailsWhenTheSDKModuleFails(t *testing.T) {
	t.Parallel()

	for _, target := range []string{"test", "test-race"} {
		t.Run(target, func(t *testing.T) {
			t.Parallel()

			out, err := makeIn(t, moduleRepo(t, false), target)

			if err == nil || !strings.Contains(out, "the sdk module ran and failed") {
				t.Errorf("make %s = %v with output %q, want it to fail on the sdk module's failing test", target, err, out)
			}
		})
	}
}
