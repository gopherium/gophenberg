// SPDX-License-Identifier: Apache-2.0

package installer

import (
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"syscall"
	"testing"
	"time"
)

// lockedByAnother reports whether a run other than the one going on finds the installer folder of server locked.
func lockedByAnother(t *testing.T, server *fakeServer) bool {
	t.Helper()
	folder, err := os.Open(server.at(installerFolder))
	if err != nil {
		t.Fatalf("opening the installer folder: %v", err)
	}
	defer func() { _ = folder.Close() }()
	return errors.Is(syscall.Flock(int(folder.Fd()), syscall.LOCK_EX|syscall.LOCK_NB), syscall.EWOULDBLOCK)
}

// probeLock has each run of the program called name on server note whether the installer folder is locked.
func probeLock(t *testing.T, server *fakeServer, name string) *[]bool {
	t.Helper()
	var seen []bool
	answering := server.effects[name]
	server.effects[name] = func(program Program) answer {
		seen = append(seen, lockedByAnother(t, server))
		return answering(program)
	}
	return &seen
}

// wantLockedThenLetGo fails t unless each probe in seen found the installer folder locked and server now finds it free.
func wantLockedThenLetGo(t *testing.T, server *fakeServer, seen []bool) {
	t.Helper()
	if len(seen) == 0 || slices.Contains(seen, false) {
		t.Errorf("the run found the installer folder locked %v while systemctl ran, want it locked each time", seen)
	}
	if lockedByAnother(t, server) {
		t.Error("the installer folder stays locked once the run ended, want it let go")
	}
}

func TestInstallHoldsTheInstallerFolderLockedWhileItRunsAndLetsItGoOnceItEnds(t *testing.T) {
	t.Parallel()

	scenario := &installScenario{t: t, server: newFakeServer(t)}
	scenario.installEarlier()
	seen := probeLock(t, scenario.server, "systemctl")

	_, _, err := runInstall(scenario.server, Request{})

	if err != nil {
		t.Fatalf("Install() = %v, want the later run through", err)
	}
	wantLockedThenLetGo(t, scenario.server, *seen)
}

func TestUninstallWithApplyHoldsTheInstallerFolderLockedWhileItRunsAndLetsItGoOnceItEnds(t *testing.T) {
	t.Parallel()

	server := firstInstalled(t)
	seen := probeLock(t, server, "systemctl")

	_, _, err := uninstallOn(server, nil, true, false)

	if err != nil {
		t.Fatalf("Uninstall() = %v, want the run through", err)
	}
	wantLockedThenLetGo(t, server, *seen)
}

func TestHeldAloneRefusesAFolderTheFileSystemWillNotLockAndLetsItGo(t *testing.T) {
	t.Parallel()

	folder, err := os.Open(t.TempDir())
	if err != nil {
		t.Fatalf("opening the installer folder: %v", err)
	}

	unlock, err := heldAlone(folder, func(int, int) error { return syscall.ENOLCK })

	wantRefused(t, err, "installer: lock /var/lib/gophenberg-installer: no locks available")
	if unlock != nil || !errors.Is(err, syscall.ENOLCK) {
		t.Errorf("heldAlone() = %v, want the lock the file system refused, with nothing to let go", err)
	}
	if err := folder.Close(); !errors.Is(err, os.ErrClosed) {
		t.Errorf("closing the folder again = %v, want it closed already when the lock failed", err)
	}
}

func TestHeldAloneLetsGoWhileACopyOfTheFolderStaysOpen(t *testing.T) {
	t.Parallel()

	server := newFakeServer(t)
	server.folder(installerFolder)
	folder, err := os.Open(server.at(installerFolder))
	if err != nil {
		t.Fatalf("opening the installer folder: %v", err)
	}
	unlock, err := heldAlone(folder, syscall.Flock)
	if err != nil {
		t.Fatalf("heldAlone() = %v, want the lock", err)
	}
	copied, err := syscall.Dup(int(folder.Fd()))
	if err != nil {
		t.Fatalf("copying the descriptor of the installer folder: %v", err)
	}
	defer func() { _ = syscall.Close(copied) }()

	unlock()

	if lockedByAnother(t, server) {
		t.Error("the installer folder stays locked while a copy of it is open, want it let go")
	}
}

// pipeAt puts a named pipe at the installer folder of server.
func pipeAt(t *testing.T, server *fakeServer) {
	t.Helper()
	server.folder(filepath.Dir(installerFolder))
	if err := syscall.Mkfifo(server.at(installerFolder), 0o700); err != nil {
		t.Fatalf("making a pipe at the installer folder: %v", err)
	}
}

// plainFileAt puts a plain file at the installer folder of server.
func plainFileAt(_ *testing.T, server *fakeServer) {
	server.write(installerFolder, "")
}

func TestInstallStopsAtAPipeOrAPlainFileAtTheInstallerFolderWithoutWaitingForAWriter(t *testing.T) {
	t.Parallel()

	for what, put := range map[string]func(*testing.T, *fakeServer){"a pipe": pipeAt, "a plain file": plainFileAt} {
		for _, dryRun := range []bool{false, true} {
			server := newFakeServer(t)
			put(t, server)
			ended := make(chan error, 1)
			go func() {
				_, _, err := runInstall(server, Request{Domain: "example.com", DryRun: dryRun})
				ended <- err
			}()

			select {
			case err := <-ended:
				wantRefused(t, err, installerFolderOpen)
			case <-time.After(5 * time.Second):
				t.Fatalf("Install() still waits on %s at the installer folder after 5s, want it to return at once", what)
			}
			if len(server.ran) != 0 {
				t.Errorf("the run at %s ran %q, want nothing run", what, server.ran)
			}
		}
	}
}

func TestInstallRefusesAnInstallerFolderItCannotOpenToLock(t *testing.T) {
	t.Parallel()
	skipAsRoot(t)

	for euid, want := range map[int]string{
		0:    "installer: open %s: permission denied",
		1000: "server:install must run as root, such as with sudo, so nothing was changed",
	} {
		server := newFakeServer(t)
		server.euid = euid
		server.folder(installerFolder)
		server.chmod(installerFolder, 0o300)
		t.Cleanup(func() { server.chmod(installerFolder, 0o700) })

		_, _, err := runInstall(server, Request{})

		wantRefused(t, err, strings.ReplaceAll(want, "%s", server.at(installerFolder)))
		if len(server.ran) != 0 {
			t.Errorf("the run ran %q, want nothing run", server.ran)
		}
	}
}
