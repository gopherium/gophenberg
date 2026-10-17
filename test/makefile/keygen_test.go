// SPDX-License-Identifier: Apache-2.0

package makefile_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// keygenRepo returns a directory holding the Makefile and a releasetool that prints the arguments it is given.
func keygenRepo(t *testing.T) string {
	t.Helper()
	for _, tool := range []string{"make", "go"} {
		if _, err := exec.LookPath(tool); err != nil {
			t.Skipf("%s is not on PATH, so make keygen cannot be exercised here", tool)
		}
	}
	root := t.TempDir()
	makefile, err := os.ReadFile(filepath.Join("..", "..", "Makefile"))
	if err != nil {
		t.Fatalf("reading the Makefile: %v", err)
	}
	files := map[string]string{
		"Makefile": string(makefile),
		"go.mod":   "module example.com/site\n\ngo 1.27.1\n",
		"cmd/releasetool/main.go": "package main\n\nimport (\n\t\"fmt\"\n\t\"os\"\n\t\"strings\"\n)\n\n" +
			"func main() { fmt.Println(\"releasetool got: \" + strings.Join(os.Args[1:], \" \")) }\n",
	}
	for path, body := range files {
		writeFile(t, filepath.Join(root, path), body)
	}
	return root
}

func TestMakeKeygenSealsARootKeyAtTheRoundsOfTheSetting(t *testing.T) {
	t.Parallel()

	cases := map[string]struct {
		args []string
		want string
	}{
		"the default rounds": {
			args: []string{"keygen", "ROOT_KEY=root1.pem"},
			want: "releasetool got: keygen -curve p256 -out root1.pem -public root1.pub.pem " +
				"-passphrase-file /dev/stdin -iterations 600000",
		},
		"rounds and a passphrase file of its own": {
			args: []string{"keygen", "ROOT_KEY=keys/root2.pem", "RELEASE_KEY_ITERATIONS=900000",
				"ROOT_KEY_PASSPHRASE_FILE=pass.txt"},
			want: "releasetool got: keygen -curve p256 -out keys/root2.pem -public keys/root2.pub.pem " +
				"-passphrase-file pass.txt -iterations 900000",
		},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			out, err := makeIn(t, keygenRepo(t), false, tc.args...)

			if err != nil || !strings.Contains(out, tc.want) {
				t.Errorf("make %v = %v with output %q, want releasetool given %q", tc.args, err, out, tc.want)
			}
		})
	}
}

func TestMakeKeygenAsksForTheRootKeyFile(t *testing.T) {
	t.Parallel()

	out, err := makeIn(t, keygenRepo(t), false, "keygen")

	if err == nil || !strings.Contains(out, "ROOT_KEY=") || strings.Contains(out, "releasetool got:") {
		t.Errorf("make keygen = %v with output %q, want it refused, asking for ROOT_KEY", err, out)
	}
	if want := `read -rs pass && printf "%s\n" "$pass" | make keygen`; !strings.Contains(out, want) {
		t.Errorf("make keygen printed %q, want the usage to show %q, which keeps the passphrase off the screen", out,
			want)
	}
}
