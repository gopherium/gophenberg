// SPDX-License-Identifier: Apache-2.0

package makefile_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// sdkTargets are the make targets that run the sdk module's tests.
var sdkTargets = []string{"test", "test-race", "cover"}

// moduleRepo returns a directory holding the Makefile, a root module and an sdk module whose test passes or fails.
func moduleRepo(t *testing.T, sdkPasses, workspace bool) string {
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
	rootModule := "module example.com/site\n\ngo 1.27.1\n\nrequire example.com/site/sdk v0.0.0\n\n" +
		"replace example.com/site/sdk => ./sdk\n"
	contract := "package sdk\n\nimport \"testing\"\n\nfunc TestContract(t *testing.T) {\n\tReady()\n\t" + verdict + "\n}\n"
	files := map[string]string{
		"Makefile":               string(makefile),
		"go.mod":                 rootModule,
		"site_test.go":           "package site\n\nimport \"testing\"\n\nfunc TestSite(t *testing.T) {}\n",
		"cmd/gophenberg/main.go": "package main\n\nfunc main() {}\n",
		"cmd/doclint/main.go":    "package main\n\nfunc main() {}\n",
		"cmd/pluginwire/main.go": "package main\n\nfunc main() {}\n",
		"internal/app/app.go":    "package app\n\nfunc Ready() bool { return true }\n",
		"sdk/go.mod":             "module example.com/site/sdk\n\ngo 1.27.1\n",
		"sdk/sdk.go":             "package sdk\n\nfunc Ready() bool { return true }\n",
		"sdk/contract_test.go":   contract,
	}
	if workspace {
		files["go.work"] = "go 1.27.1\n\nuse .\n"
	}
	for path, body := range files {
		if err := os.MkdirAll(filepath.Dir(filepath.Join(root, path)), 0o755); err != nil {
			t.Fatalf("making the folder of %s: %v", path, err)
		}
		if err := os.WriteFile(filepath.Join(root, path), []byte(body), 0o644); err != nil {
			t.Fatalf("writing %s: %v", path, err)
		}
	}
	return root
}

// makeIn runs make with the arguments in the directory, free of the outer make's flags, and returns what it printed.
func makeIn(t *testing.T, dir string, workspace bool, args ...string) (string, error) {
	t.Helper()
	gowork := "GOWORK=off"
	if workspace {
		gowork = "GOWORK="
	}
	command := exec.Command("make", args...)
	command.Dir = dir
	command.Env = append(os.Environ(), gowork, "GOFLAGS=", "MAKEFLAGS=", "MFLAGS=", "MAKELEVEL=")
	out, err := command.CombinedOutput()
	return string(out), err
}

// skipWithoutRace skips the test when the target is test-race and the race detector cannot build here.
func skipWithoutRace(t *testing.T, target string) {
	t.Helper()
	if target != "test-race" {
		return
	}
	out, err := exec.Command("go", "env", "CGO_ENABLED", "CC").Output()
	cgo, compiler, _ := strings.Cut(strings.TrimSpace(string(out)), "\n")
	if err != nil || cgo != "1" {
		t.Skipf("cgo is off (%q, %v), so make test-race cannot run here", cgo, err)
	}
	if fields := strings.Fields(compiler); len(fields) == 0 {
		t.Skip("go names no C compiler, so make test-race cannot run here")
	} else if _, err := exec.LookPath(fields[0]); err != nil {
		t.Skipf("the C compiler %s is not on PATH, so make test-race cannot run here", fields[0])
	}
}

func TestMakeRunsTheSDKModule(t *testing.T) {
	t.Parallel()

	for _, target := range sdkTargets {
		for _, workspace := range []bool{false, true} {
			name := target
			if workspace {
				name += " beside a workspace file"
			}
			t.Run(name, func(t *testing.T) {
				t.Parallel()
				skipWithoutRace(t, target)

				out, err := makeIn(t, moduleRepo(t, true, workspace), workspace, target)

				if err != nil || !strings.Contains(out, "example.com/site/sdk") {
					t.Errorf("make %s = %v with output %q, want the sdk module's tests run and passed", target, err, out)
				}
			})
		}
	}
}

func TestMakeFailsWhenTheSDKModuleFails(t *testing.T) {
	t.Parallel()

	for _, target := range sdkTargets {
		t.Run(target, func(t *testing.T) {
			t.Parallel()
			skipWithoutRace(t, target)

			out, err := makeIn(t, moduleRepo(t, false, false), false, target)

			if err == nil || !strings.Contains(out, "the sdk module ran and failed") {
				t.Errorf("make %s = %v with output %q, want it to fail on the sdk module's failing test", target, err, out)
			}
		})
	}
}

func TestMakeCoverCountsTheSDKModule(t *testing.T) {
	t.Parallel()

	root := moduleRepo(t, true, false)

	out, err := makeIn(t, root, false, "cover")

	profile, readErr := os.ReadFile(filepath.Join(root, ".covdata", "cover.out"))
	if err != nil || readErr != nil || !strings.Contains(string(profile), "example.com/site/sdk/sdk.go") {
		t.Errorf("make cover = %v, profile %v with output %q, want the sdk module's lines in the merged profile",
			err, readErr, out)
	}
}
