// SPDX-License-Identifier: Apache-2.0

package updater_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/gopherium/gophenberg/internal/updater"
)

// wantLink fails t unless the link at path names target.
func wantLink(t *testing.T, path, target string) {
	t.Helper()
	held, err := os.Readlink(path)
	if err != nil || held != target {
		t.Errorf("%s names %q, %v, want %q", path, held, err, target)
	}
}

// denied makes the folder at path take mode until t ends, skipping t under root, which ignores the mode.
func denied(t *testing.T, path string, mode os.FileMode) {
	t.Helper()
	modeRestored(t, path)
	if err := os.Chmod(path, mode); err != nil {
		t.Fatalf("changing the mode of %s: %v", path, err)
	}
}

// modeRestored puts the folder at path back to 0o755 once t ends, skipping t under root, which ignores any mode.
func modeRestored(t *testing.T, path string) {
	t.Helper()
	if os.Geteuid() == 0 {
		t.Skip("running as root, which reads and writes a folder whatever its mode says")
	}
	t.Cleanup(func() { _ = os.Chmod(path, 0o755) })
}

func TestLayoutNamesEachPlaceOfTheInstallRoot(t *testing.T) {
	t.Parallel()

	layout := updater.Layout{Root: "/opt/gophenberg"}

	for got, want := range map[string]string{
		layout.Build("0.21.0"):    "/opt/gophenberg/versions/0.21.0",
		layout.Binary("0.21.0"):   "/opt/gophenberg/versions/0.21.0/gophenberg",
		layout.Download("0.21.0"): "/opt/gophenberg/downloads/0.21.0.partial",
		layout.Backups():          "/opt/gophenberg/backups",
		layout.StateFile():        "/opt/gophenberg/state/update.json",
	} {
		if got != want {
			t.Errorf("the layout names %s, want %s", got, want)
		}
	}
}

func TestLayoutKeepsTheBackupsAndTheStateInADataFolder(t *testing.T) {
	t.Parallel()

	layout := updater.Layout{Root: "/opt/gophenberg", Data: "/srv/gophenberg-data"}

	for got, want := range map[string]string{
		layout.Build("0.21.0"):    "/opt/gophenberg/versions/0.21.0",
		layout.Download("0.21.0"): "/opt/gophenberg/downloads/0.21.0.partial",
		layout.Backups():          "/srv/gophenberg-data/backups",
		layout.StateFile():        "/srv/gophenberg-data/state/update.json",
		layout.Kept():             "/srv/gophenberg-data",
	} {
		if got != want {
			t.Errorf("the layout names %s, want %s", got, want)
		}
	}
}

func TestLayoutKeepsTheBackupsAndTheStateInTheInstallRootWithNoDataFolder(t *testing.T) {
	t.Parallel()

	layout := updater.Layout{Root: "/opt/gophenberg"}

	if got := layout.Kept(); got != "/opt/gophenberg" {
		t.Errorf("Kept() = %q, want the install root", got)
	}
}

func TestMakeDataMakesAFolderOnlyItsOwnerOpens(t *testing.T) {
	t.Parallel()

	layout := updater.Layout{Data: filepath.Join(t.TempDir(), "parent", "gophenberg-data")}

	if err := layout.MakeData(); err != nil {
		t.Fatalf("MakeData() = %v, want nil", err)
	}

	info, err := os.Stat(layout.Data)
	if err != nil {
		t.Fatalf("reading the data folder: %v", err)
	}
	if info.Mode().Perm() != 0o700 {
		t.Errorf("the data folder has the mode %o, want 700", info.Mode().Perm())
	}
}

func TestMakeDataLeavesAFolderThatIsThereAsItStands(t *testing.T) {
	t.Parallel()

	layout := updater.Layout{Data: t.TempDir()}
	if err := os.Chmod(layout.Data, 0o750); err != nil {
		t.Fatalf("changing the mode: %v", err)
	}

	if err := layout.MakeData(); err != nil {
		t.Fatalf("MakeData() = %v, want nil", err)
	}

	info, err := os.Stat(layout.Data)
	if err != nil || info.Mode().Perm() != 0o750 {
		t.Errorf("the data folder has the mode %v (%v), want it left at 750", info.Mode(), err)
	}
}

func TestMakeDataMakesNothingWithNoDataFolder(t *testing.T) {
	t.Parallel()

	if err := (updater.Layout{}).MakeData(); err != nil {
		t.Errorf("MakeData() = %v, want nil", err)
	}
}

func TestPointMakesARelativeLinkToTheBuild(t *testing.T) {
	t.Parallel()

	layout := updater.Layout{Root: t.TempDir()}

	if err := layout.Point(updater.Current, "0.21.0"); err != nil {
		t.Fatalf("Point() = %v, want nil", err)
	}

	wantLink(t, filepath.Join(layout.Root, "current"), filepath.Join("versions", "0.21.0"))
	if name, err := layout.Named(updater.Current); err != nil || name != "0.21.0" {
		t.Errorf("Named() = %q, %v, want the build 0.21.0", name, err)
	}
}

func TestPointReplacesTheLinkThereAndLeavesNoNewLinkBehind(t *testing.T) {
	t.Parallel()

	layout := updater.Layout{Root: t.TempDir()}
	if err := os.Symlink(filepath.Join("versions", "0.20.0"), filepath.Join(layout.Root, "previous")); err != nil {
		t.Fatalf("making the previous link: %v", err)
	}
	stale := filepath.Join(layout.Root, ".previous.partial")
	if err := os.WriteFile(stale, []byte("left by a stop"), 0o644); err != nil {
		t.Fatalf("writing a stale new link: %v", err)
	}

	if err := layout.Point(updater.Previous, "0.21.0"); err != nil {
		t.Fatalf("Point() = %v, want nil", err)
	}

	wantLink(t, filepath.Join(layout.Root, "previous"), filepath.Join("versions", "0.21.0"))
	wantMissing(t, stale)
}

func TestPointRefusesALinkItCannotReplace(t *testing.T) {
	t.Parallel()

	layout := updater.Layout{Root: t.TempDir()}
	kept := filepath.Join(layout.Root, "current", "notes.txt")
	if err := os.MkdirAll(filepath.Dir(kept), 0o755); err != nil {
		t.Fatalf("making a folder called current: %v", err)
	}
	if err := os.WriteFile(kept, []byte("a person's notes"), 0o644); err != nil {
		t.Fatalf("writing into the folder: %v", err)
	}

	err := layout.Point(updater.Current, "0.21.0")

	wantRefused(t, err, "updater: point current at 0.21.0")
	wantFile(t, kept, "a person's notes")
	wantMissing(t, filepath.Join(layout.Root, ".current.partial"))
}

func TestPointRefusesARootItCannotOpen(t *testing.T) {
	t.Parallel()

	for name, root := range map[string]func(t *testing.T) string{
		"a root that is not there": func(t *testing.T) string { return filepath.Join(t.TempDir(), "gophenberg") },
		"a root it may not read": func(t *testing.T) string {
			root := t.TempDir()
			denied(t, root, 0o300)
			return root
		},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			layout := updater.Layout{Root: root(t)}

			err := layout.Point(updater.Current, "0.21.0")

			wantRefused(t, err, "updater: point current at 0.21.0")
			wantMissing(t, filepath.Join(layout.Root, "current"))
		})
	}
}

func TestPointInMakesARelativeLinkInsideTheTree(t *testing.T) {
	t.Parallel()

	layout := updater.Layout{Root: t.TempDir()}

	if err := updater.PointIn(openTree(t, layout.Root), updater.Current, "0.21.0"); err != nil {
		t.Fatalf("PointIn() = %v, want nil", err)
	}

	wantLink(t, filepath.Join(layout.Root, "current"), filepath.Join("versions", "0.21.0"))
	if name, err := layout.Named(updater.Current); err != nil || name != "0.21.0" {
		t.Errorf("Named() = %q, %v, want the build 0.21.0", name, err)
	}
}

func TestPointInReplacesLinksThatLedOutOfTheTreeWithoutFollowingThem(t *testing.T) {
	t.Parallel()

	root, outside := t.TempDir(), t.TempDir()
	kept := filepath.Join(outside, "notes.txt")
	if err := os.WriteFile(kept, []byte("a person's notes"), 0o644); err != nil {
		t.Fatalf("writing a file outside the tree: %v", err)
	}
	for link, target := range map[string]string{"current": outside, ".current.partial": kept} {
		if err := os.Symlink(target, filepath.Join(root, link)); err != nil {
			t.Fatalf("linking %s out of the tree: %v", link, err)
		}
	}

	if err := updater.PointIn(openTree(t, root), updater.Current, "0.21.0"); err != nil {
		t.Fatalf("PointIn() = %v, want nil", err)
	}

	wantLink(t, filepath.Join(root, "current"), filepath.Join("versions", "0.21.0"))
	wantMissing(t, filepath.Join(root, ".current.partial"))
	wantEntries(t, outside, "notes.txt")
	wantFile(t, kept, "a person's notes")
}

func TestPointInRefusesARootItCannotSaveToDisk(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	tree := openTree(t, root)
	denied(t, root, 0o300)

	err := updater.PointIn(tree, updater.Current, "0.21.0")

	wantRefused(t, err, "updater: point current at 0.21.0")
	wantMissing(t, filepath.Join(root, ".current.partial"))
}

func TestNamedRefusesALinkThatNamesNoBuild(t *testing.T) {
	t.Parallel()

	for _, target := range []string{"elsewhere/0.21.0", "versions/..", "versions/.", "versions"} {
		layout := updater.Layout{Root: t.TempDir()}
		if err := os.Symlink(target, filepath.Join(layout.Root, "current")); err != nil {
			t.Fatalf("making the current link: %v", err)
		}

		name, err := layout.Named(updater.Current)

		wantRefused(t, err, "updater: the current link names "+target+", not a build in versions")
		if name != "" {
			t.Errorf("Named() of a link to %s = %q, want no build named", target, name)
		}
	}
}

func TestNamedRefusesALinkThatIsNotThere(t *testing.T) {
	t.Parallel()

	_, err := updater.Layout{Root: t.TempDir()}.Named(updater.Previous)

	wantRefused(t, err, "updater: read the previous link")
}
