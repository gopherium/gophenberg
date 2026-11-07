// SPDX-License-Identifier: Apache-2.0

package installer_test

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gopherium/gophenberg/internal/installer"
)

// program writes a shell script called name into folder that prints line, and returns its path.
func program(t *testing.T, folder, name, line string) string {
	t.Helper()
	path := filepath.Join(folder, name)
	if err := os.WriteFile(path, []byte("#!/bin/sh\necho '"+line+"'\n"), 0o755); err != nil {
		t.Fatalf("writing the program %s: %v", name, err)
	}
	return path
}

func TestTheRunnerHandsAProgramOnlyTheSettingsItNames(t *testing.T) {
	t.Setenv("GOPHENBERG_INSTALL_ROOT", "/etc")
	t.Setenv("APT_CONFIG", "/home/maria/apt.conf")

	printed, err := installer.RealMachine().Run(t.Context(), installer.Program{
		Name: "env", Env: []string{"GOPHENBERG_EXAMPLE=Maria Perez"},
	})

	want := "PATH=/usr/sbin:/usr/bin:/sbin:/bin\nLC_ALL=C\nGOPHENBERG_EXAMPLE=Maria Perez\n"
	if err != nil || printed != want {
		t.Errorf("running env printed %q with %v, want only %q", printed, err, want)
	}
}

func TestTheRunnerFindsAProgramOnlyInItsOwnPath(t *testing.T) {
	planted := t.TempDir()
	program(t, planted, "gophenberg-planted", "planted")
	t.Setenv("PATH", planted+string(os.PathListSeparator)+os.Getenv("PATH"))

	printed, err := installer.RealMachine().Run(t.Context(), installer.Program{Name: "gophenberg-planted"})

	want := "installer: no program gophenberg-planted in /usr/sbin:/usr/bin:/sbin:/bin"
	if err == nil || err.Error() != want || printed != "" {
		t.Errorf("running a program only the caller's PATH holds = %q, %v, want %q", printed, err, want)
	}
}

func TestTheRunnerRunsAProgramNamedByItsAbsolutePathAsItIs(t *testing.T) {
	t.Parallel()

	path := program(t, t.TempDir(), "gophenberg", "Gophenberg 0.20.0")

	printed, err := installer.RealMachine().Run(t.Context(), installer.Program{Name: path})

	if err != nil || printed != "Gophenberg 0.20.0\n" {
		t.Errorf("running %s printed %q with %v, want its line", path, printed, err)
	}
}

func TestTheRunnerRunsAProgramInTheFolderItNames(t *testing.T) {
	t.Parallel()

	folder, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatalf("resolving the test folder: %v", err)
	}

	printed, err := installer.RealMachine().Run(t.Context(), installer.Program{Name: "pwd", Dir: folder})

	if err != nil || printed != folder+"\n" {
		t.Errorf("running pwd in %s printed %q with %v, want that folder", folder, printed, err)
	}
}

func TestTheRunnerRunsAProgramAtTheTopOfTheFileSystemWhenNoFolderIsNamed(t *testing.T) {
	t.Parallel()

	printed, err := installer.RealMachine().Run(t.Context(), installer.Program{Name: "pwd"})

	if err != nil || printed != "/\n" {
		t.Errorf("running pwd printed %q with %v, want /", printed, err)
	}
}

func TestTheRunnerFeedsAProgramItsStandardInput(t *testing.T) {
	t.Parallel()

	printed, err := installer.RealMachine().Run(t.Context(), installer.Program{Name: "cat", Stdin: "Maria Perez\n"})

	if err != nil || printed != "Maria Perez\n" {
		t.Errorf("running cat printed %q with %v, want its standard input back", printed, err)
	}
}

func TestTheRunnerStartsAProgramInASessionOfItsOwnWithoutATerminal(t *testing.T) {
	t.Parallel()

	printed, err := installer.RealMachine().Run(t.Context(), installer.Program{
		Name: "cat", Args: []string{"/proc/self/stat"},
	})

	pid, rest, _ := strings.Cut(printed, " ")
	_, after, _ := strings.Cut(rest, ") ")
	fields := strings.Fields(after)
	if err != nil || len(fields) < 5 {
		t.Fatalf("running cat on /proc/self/stat printed %q with %v, want the stat of cat", printed, err)
	}
	if got := [2]string{fields[3], fields[4]}; got != [2]string{pid, "0"} {
		t.Errorf("the program ran in the session %s on the terminal %s, want a session of its own, %s, and no "+
			"terminal, 0", got[0], got[1], pid)
	}
}

// stopOnLast is an output that keeps what reaches it and stops the run once that ends with the line last.
type stopOnLast struct {
	last   string
	cancel context.CancelFunc
	copied strings.Builder
}

// Write keeps p and stops the run once the copy ends with the line last.
func (s *stopOnLast) Write(p []byte) (int, error) {
	n, err := s.copied.Write(p)
	if strings.HasSuffix(s.copied.String(), s.last) {
		s.cancel()
	}
	return n, err
}

func TestTheRunnerCopiesWhatAProgramPrintsToItsOutputWhileTheProgramRuns(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	output := &stopOnLast{last: "184467235\n", cancel: cancel}

	printed, err := installer.RealMachine().Run(ctx, installer.Program{
		Name: "sh", Args: []string{"-c", "echo Maria Perez; echo 184467235 >&2; exec sleep 10"}, Output: output,
	})

	if err == nil || err.Error() != "installer: run sh: signal: killed" {
		t.Errorf("running sh = %v, want it killed by the run its output stopped while it ran", err)
	}
	if want := "Maria Perez\n184467235\n"; printed != want || output.copied.String() != want {
		t.Errorf("running sh printed %q and copied %q to its output, want %q both times", printed,
			output.copied.String(), want)
	}
}

func TestTheRunnerReturnsWhatAFailedProgramPrintedAndHowItExited(t *testing.T) {
	t.Parallel()

	printed, err := installer.RealMachine().Run(t.Context(), installer.Program{
		Name: "ls", Args: []string{"/gophenberg-missing"},
	})

	var exited *exec.ExitError
	if !errors.As(err, &exited) || exited.ExitCode() != 2 || !strings.HasPrefix(err.Error(), "installer: run ls: ") {
		t.Errorf("running ls on a missing path = %v, want its exit status 2 named", err)
	}
	if want := "/usr/bin/ls: cannot access '/gophenberg-missing': No such file or directory\n"; printed != want {
		t.Errorf("running ls on a missing path printed %q, want its error in plain C words %q", printed, want)
	}
}
