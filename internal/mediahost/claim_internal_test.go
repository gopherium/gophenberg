// SPDX-License-Identifier: Apache-2.0

package mediahost

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"syscall"
	"testing"
)

// capFileSize forbids the process any file larger than limit bytes for the rest of the test.
func capFileSize(t *testing.T, limit uint64) {
	t.Helper()
	var original syscall.Rlimit
	if err := syscall.Getrlimit(syscall.RLIMIT_FSIZE, &original); err != nil {
		t.Skipf("reading the file size limit: %v", err)
	}
	forbidden := original
	forbidden.Cur = limit
	if err := syscall.Setrlimit(syscall.RLIMIT_FSIZE, &forbidden); err != nil {
		t.Skipf("capping the file size limit: %v", err)
	}
	t.Cleanup(func() {
		if err := syscall.Setrlimit(syscall.RLIMIT_FSIZE, &original); err != nil {
			t.Errorf("restoring the file size limit: %v", err)
		}
	})
}

func TestClaimGivesUpOnceEveryNameIsTaken(t *testing.T) {
	t.Parallel()

	library := New(Config{Dir: t.TempDir()})
	month := "2006/01"
	crowded := library.abs(month)
	if err := os.MkdirAll(crowded, 0o755); err != nil {
		t.Fatalf("making the month directory: %v", err)
	}
	for attempt := 1; attempt <= maxNameAttempts; attempt++ {
		taken := filepath.Join(crowded, numberedStem("taken", attempt)+".jpg")
		if err := os.WriteFile(taken, nil, 0o644); err != nil {
			t.Fatalf("occupying %q: %v", taken, err)
		}
	}

	rel, err := library.claim(month, "taken", "jpg", []byte("data"), "")

	if err == nil {
		t.Fatalf("claim() with every name taken = %q, nil, want a failure", rel)
	}
	want := fmt.Sprintf("no free name for %q after %d attempts", "taken", maxNameAttempts)
	if err.Error() != want {
		t.Fatalf("claim() error = %q, want %q", err.Error(), want)
	}
	if rel != "" {
		t.Errorf("claim() name = %q, want no name once it gives up", rel)
	}
	if _, err := os.Stat(library.notePath(month + "/taken.jpg")); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("Stat() of the note = %v, want every name another file holds given back", err)
	}
}

func TestClaimKeepsTheNoteOfAWriteThatFailsForTheNextStart(t *testing.T) {
	library := New(Config{Dir: t.TempDir()})
	capFileSize(t, 64)

	_, err := library.claim("2006/01", "harbor", "jpg", bytes.Repeat([]byte("x"), 1024), "")

	if !errors.Is(err, syscall.EFBIG) {
		t.Fatalf("claim() past the file size limit error = %v, want the write refused as too large", err)
	}
	if _, statErr := os.Stat(library.notePath("2006/01/harbor.jpg")); statErr != nil {
		t.Errorf("Stat() of the note = %v, want it kept for the next start to settle", statErr)
	}
}

func TestClaimGivesTheNameBackToADirectoryHoldingIt(t *testing.T) {
	t.Parallel()

	library := New(Config{Dir: t.TempDir()})
	if err := os.MkdirAll(library.abs("2006/01/harbor.jpg"), 0o755); err != nil {
		t.Fatalf("planting the directory: %v", err)
	}

	if _, err := library.claim("2006/01", "harbor", "jpg", []byte("data"), ""); err == nil {
		t.Fatal("claim() = nil, want the directory holding the name reported")
	}
	if _, err := os.Stat(library.notePath("2006/01/harbor.jpg")); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("Stat() of the note = %v, want no note left on a path a directory holds", err)
	}
}

func TestClaimReportsALibraryThatCannotHoldNotes(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, notesDir), nil, 0o644); err != nil {
		t.Fatalf("planting a file where the notes folder belongs: %v", err)
	}
	library := New(Config{Dir: dir})

	rel, err := library.claim("2006/01", "harbor", "jpg", []byte("data"), "")

	if err == nil {
		t.Fatalf("claim() = %q, nil, want the missing notes folder reported", rel)
	}
	if _, statErr := os.Stat(library.abs("2006/01/harbor.jpg")); !errors.Is(statErr, os.ErrNotExist) {
		t.Errorf("Stat() of the upload = %v, want nothing written without a note", statErr)
	}
}

func TestRemoveFilesKeepsTheNoteOfAFileItCannotDelete(t *testing.T) {
	t.Parallel()

	if os.Geteuid() == 0 {
		t.Skip("running as root, which deletes from a folder whatever its mode says")
	}
	library := New(Config{Dir: t.TempDir()})
	rel, err := library.claim("2006/01", "harbor", "jpg", []byte("data"), "")
	if err != nil {
		t.Fatalf("claim() error = %v, want nil", err)
	}
	folder := library.abs("2006/01")
	if err := os.Chmod(folder, 0o555); err != nil {
		t.Fatalf("locking the folder: %v", err)
	}
	t.Cleanup(func() { _ = os.Chmod(folder, 0o755) })

	library.removeFiles(rel)

	if _, err := os.Stat(library.notePath(rel)); err != nil {
		t.Errorf("Stat() of the note = %v, want the note kept while its file stays", err)
	}
}

func TestWriteExclusiveNamesTheDirectoryHoldingTheTarget(t *testing.T) {
	t.Parallel()

	blocked := filepath.Join(t.TempDir(), "held-by-a-directory.jpg")
	if err := os.Mkdir(blocked, 0o755); err != nil {
		t.Fatalf("planting the directory: %v", err)
	}

	err := writeExclusive(blocked, []byte("data"))

	want := fmt.Sprintf("%s is held by a directory", blocked)
	if err == nil || err.Error() != want {
		t.Fatalf("writeExclusive() onto a directory error = %v, want %q", err, want)
	}
	if errors.Is(err, os.ErrExist) {
		t.Errorf("writeExclusive() onto a directory error = %v, want the bare exists error replaced", err)
	}
}

func TestWriteExclusiveClearsTheFileItCannotFill(t *testing.T) {
	target := filepath.Join(t.TempDir(), "over-the-limit.jpg")
	capFileSize(t, 0)

	err := writeExclusive(target, []byte("data"))

	if !errors.Is(err, syscall.EFBIG) {
		t.Fatalf("writeExclusive() past the file size limit error = %v, want the write refused as too large", err)
	}
	if _, statErr := os.Stat(target); !errors.Is(statErr, os.ErrNotExist) {
		t.Errorf("Stat() of the target = %v, want the file it could not fill removed", statErr)
	}
}
