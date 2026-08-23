// SPDX-License-Identifier: Apache-2.0

package mediahost

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"syscall"
	"testing"
)

// capFileSizeToNothing forbids the process every byte of file growth for the rest of the test.
func capFileSizeToNothing(t *testing.T) {
	t.Helper()
	var original syscall.Rlimit
	if err := syscall.Getrlimit(syscall.RLIMIT_FSIZE, &original); err != nil {
		t.Skipf("reading the file size limit: %v", err)
	}
	forbidden := original
	forbidden.Cur = 0
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

	rel, err := library.claim(month, "taken", "jpg", []byte("data"))

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
	capFileSizeToNothing(t)

	err := writeExclusive(target, []byte("data"))

	if !errors.Is(err, syscall.EFBIG) {
		t.Fatalf("writeExclusive() past the file size limit error = %v, want the write refused as too large", err)
	}
	if _, statErr := os.Stat(target); !errors.Is(statErr, os.ErrNotExist) {
		t.Errorf("Stat() of the target = %v, want the file it could not fill removed", statErr)
	}
}
