// SPDX-License-Identifier: Apache-2.0

package mediahost_test

import (
	"context"
	"errors"
	"os"
	"path"
	"path/filepath"
	"slices"
	"testing"
	"time"

	"github.com/gopherium/gophenberg/internal/media"
	"github.com/gopherium/gophenberg/internal/mediahost"
)

// filesOf returns the item's stored file and every rendition file it owns.
func filesOf(m media.Media) []string {
	files := []string{m.File}
	for _, r := range m.Sizes {
		if !slices.Contains(files, r.File) {
			files = append(files, r.File)
		}
	}
	return files
}

// stillStored reports whether a library relative file is still on disk.
func stillStored(t *testing.T, l *mediahost.Library, file string) bool {
	t.Helper()
	_, err := os.Stat(filepath.Join(l.Dir(), filepath.FromSlash(file)))
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("reading %q: %v", file, err)
	}
	return err == nil
}

// nothingSaved answers that no stored item names any of the files.
func nothingSaved(context.Context, []string) (map[string]bool, error) {
	return map[string]bool{}, nil
}

// savedAs answers that a stored item names the one file.
func savedAs(file string) mediahost.Saved {
	return func(context.Context, []string) (map[string]bool, error) {
		return map[string]bool{file: true}, nil
	}
}

// soon is a cutoff every note written so far stands before.
func soon() time.Time {
	return time.Now().Add(time.Minute)
}

func TestRecoverDeletesTheFilesOfAnUploadThatWasNeverSaved(t *testing.T) {
	t.Parallel()

	library := newLibrary(t)
	m := mustIngest(t, library, "harbor.jpg", jpegImage(t, 2400, 1600))

	deleted, err := library.Recover(t.Context(), soon(), nothingSaved)

	if err != nil {
		t.Fatalf("Recover() error = %v, want nil", err)
	}
	for _, file := range filesOf(m) {
		if stillStored(t, library, file) {
			t.Errorf("%s is still stored, want an unsaved upload's files deleted", file)
		}
		if !slices.Contains(deleted, file) {
			t.Errorf("Recover() = %v, want %s reported as deleted", deleted, file)
		}
	}
}

func TestRecoverLeavesTheFilesOfAFinishedItem(t *testing.T) {
	t.Parallel()

	library := newLibrary(t)
	m := mustIngest(t, library, "harbor.jpg", jpegImage(t, 2400, 1600))
	library.Finish(m)

	deleted, err := library.Recover(t.Context(), soon(), nothingSaved)

	if err != nil || len(deleted) != 0 {
		t.Fatalf("Recover() = %v, %v, want nothing deleted once the item finished", deleted, err)
	}
	for _, file := range filesOf(m) {
		if !stillStored(t, library, file) {
			t.Errorf("%s is gone, want a finished item's files kept", file)
		}
	}
}

func TestRecoverKeepsAnItemSavedBeforeItsUploadWasMarkedFinished(t *testing.T) {
	t.Parallel()

	library := newLibrary(t)
	m := mustIngest(t, library, "harbor.jpg", jpegImage(t, 2400, 1600))

	deleted, err := library.Recover(t.Context(), soon(), savedAs(m.File))

	if err != nil || len(deleted) != 0 {
		t.Fatalf("Recover() = %v, %v, want nothing deleted for a saved item", deleted, err)
	}
	if again, err := library.Recover(t.Context(), soon(), nothingSaved); err != nil || len(again) != 0 {
		t.Errorf("a second Recover() = %v, %v, want the saved item's notes cleared the first time", again, err)
	}
	for _, file := range filesOf(m) {
		if !stillStored(t, library, file) {
			t.Errorf("%s is gone, want a saved item's files kept", file)
		}
	}
}

func TestRecoverLeavesAnUploadNotedAfterTheCutoff(t *testing.T) {
	t.Parallel()

	library := newLibrary(t)
	m := mustIngest(t, library, "harbor.jpg", jpegImage(t, 2400, 1600))

	deleted, err := library.Recover(t.Context(), time.Now().Add(-time.Hour), nothingSaved)

	if err != nil || len(deleted) != 0 {
		t.Fatalf("Recover() = %v, %v, want an upload noted after the cutoff left alone", deleted, err)
	}
	if !stillStored(t, library, m.File) {
		t.Errorf("%s is gone, want an upload noted after the cutoff kept", m.File)
	}
}

func TestRecoverAsksAboutEachItemOnce(t *testing.T) {
	t.Parallel()

	library := newLibrary(t)
	m := mustIngest(t, library, "harbor.jpg", jpegImage(t, 2400, 1600))
	var asked []string

	_, err := library.Recover(t.Context(), soon(), func(_ context.Context, files []string) (map[string]bool, error) {
		asked = append(asked, files...)
		return map[string]bool{}, nil
	})

	if err != nil {
		t.Fatalf("Recover() error = %v, want nil", err)
	}
	if len(m.Sizes) == 0 {
		t.Fatalf("Sizes = %v, want renditions noted beside the item's file", m.Sizes)
	}
	if !slices.Equal(asked, []string{m.File}) {
		t.Errorf("Recover() asked about %v, want the item's own file once", asked)
	}
}

func TestRecoverTouchesNothingWhenItCannotAskWhatIsSaved(t *testing.T) {
	t.Parallel()

	library := newLibrary(t)
	m := mustIngest(t, library, "harbor.jpg", jpegImage(t, 2400, 1600))
	errStoreDown := errors.New("the store is down")

	deleted, err := library.Recover(t.Context(), soon(), func(context.Context, []string) (map[string]bool, error) {
		return nil, errStoreDown
	})

	if !errors.Is(err, errStoreDown) || len(deleted) != 0 {
		t.Fatalf("Recover() = %v, %v, want %v and nothing deleted", deleted, err, errStoreDown)
	}
	if !stillStored(t, library, m.File) {
		t.Errorf("%s is gone, want nothing touched without an answer", m.File)
	}
}

func TestRecoverFindsNothingInALibraryThatNeverStoredAnUpload(t *testing.T) {
	t.Parallel()

	deleted, err := newLibrary(t).Recover(t.Context(), soon(), nothingSaved)

	if err != nil || len(deleted) != 0 {
		t.Errorf("Recover() = %v, %v, want nothing and no failure", deleted, err)
	}
}

// lockFolder keeps the files inside the library relative folder from being deleted until the test ends.
func lockFolder(t *testing.T, l *mediahost.Library, folder string) func() {
	t.Helper()
	if os.Geteuid() == 0 {
		t.Skip("running as root, which deletes from a folder whatever its mode says")
	}
	locked := filepath.Join(l.Dir(), filepath.FromSlash(folder))
	if err := os.Chmod(locked, 0o555); err != nil {
		t.Fatalf("locking %q: %v", folder, err)
	}
	unlock := func() {
		if err := os.Chmod(locked, 0o755); err != nil {
			t.Errorf("unlocking %q: %v", folder, err)
		}
	}
	t.Cleanup(unlock)
	return unlock
}

func TestRecoverReportsAFileItCannotDeleteAndTriesAgainLater(t *testing.T) {
	t.Parallel()

	library := newLibrary(t)
	m := mustIngest(t, library, "notes.pdf", pdfDocument())
	unlock := lockFolder(t, library, filepath.ToSlash(filepath.Dir(m.File)))

	deleted, err := library.Recover(t.Context(), soon(), nothingSaved)

	if err == nil || len(deleted) != 0 {
		t.Fatalf("Recover() = %v, %v, want the file it could not delete reported", deleted, err)
	}
	unlock()
	again, err := library.Recover(t.Context(), soon(), nothingSaved)
	if err != nil || !slices.Equal(again, []string{m.File}) {
		t.Errorf("a later Recover() = %v, %v, want the file deleted once it can be", again, err)
	}
}

func TestRemoveKeepsTheNoteOfAFileItCannotDeleteForRecoverToRetry(t *testing.T) {
	t.Parallel()

	library := newLibrary(t)
	m := mustIngest(t, library, "notes.pdf", pdfDocument())
	unlock := lockFolder(t, library, filepath.ToSlash(filepath.Dir(m.File)))
	if err := library.Remove(m); err == nil {
		t.Fatal("Remove() = nil, want the file it could not delete reported")
	}
	unlock()

	deleted, err := library.Recover(t.Context(), soon(), nothingSaved)

	if err != nil || !slices.Equal(deleted, []string{m.File}) {
		t.Errorf("Recover() = %v, %v, want the file Remove could not delete tried again", deleted, err)
	}
}

func TestRecoverReportsTheNoteOfASavedItemItCannotClear(t *testing.T) {
	t.Parallel()

	library := newLibrary(t)
	m := mustIngest(t, library, "notes.pdf", pdfDocument())
	lockFolder(t, library, path.Join(".uploading", path.Dir(m.File)))

	deleted, err := library.Recover(t.Context(), soon(), savedAs(m.File))

	if err == nil || len(deleted) != 0 {
		t.Errorf("Recover() = %v, %v, want the note it could not clear reported and nothing deleted", deleted, err)
	}
	if !stillStored(t, library, m.File) {
		t.Errorf("%s is gone, want a saved item's file kept", m.File)
	}
}

func TestRecoverReportsTheNoteOfADeletedFileItCannotClear(t *testing.T) {
	t.Parallel()

	library := newLibrary(t)
	m := mustIngest(t, library, "notes.pdf", pdfDocument())
	lockFolder(t, library, path.Join(".uploading", path.Dir(m.File)))

	deleted, err := library.Recover(t.Context(), soon(), nothingSaved)

	if err == nil || !slices.Equal(deleted, []string{m.File}) {
		t.Errorf("Recover() = %v, %v, want the file deleted and the note it could not clear reported", deleted, err)
	}
}

func TestRemoveLeavesNoNoteForRecoverToFind(t *testing.T) {
	t.Parallel()

	library := newLibrary(t)
	m := mustIngest(t, library, "harbor.jpg", jpegImage(t, 2400, 1600))
	if err := library.Remove(m); err != nil {
		t.Fatalf("Remove() error = %v, want nil", err)
	}

	deleted, err := library.Recover(t.Context(), soon(), nothingSaved)

	if err != nil || len(deleted) != 0 {
		t.Errorf("Recover() = %v, %v, want nothing left to recover once the item was removed", deleted, err)
	}
}

func TestIngestSkipsANameAnUnfinishedUploadStillHolds(t *testing.T) {
	t.Parallel()

	library := newLibrary(t)
	first := mustIngest(t, library, "notes.pdf", pdfDocument())
	if err := os.Remove(filepath.Join(library.Dir(), filepath.FromSlash(first.File))); err != nil {
		t.Fatalf("removing the first upload's file, as a stop before its write would: %v", err)
	}

	second := mustIngest(t, library, "notes.pdf", pdfDocument())

	if second.File == first.File {
		t.Errorf("File = %q, want a name the unfinished upload does not still hold", second.File)
	}
	library.Finish(second)
	if _, err := library.Recover(t.Context(), soon(), nothingSaved); err != nil {
		t.Fatalf("Recover() error = %v, want nil", err)
	}
	if !stillStored(t, library, second.File) {
		t.Errorf("%s is gone, want the second upload untouched by the first one's recovery", second.File)
	}
}
