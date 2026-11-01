// SPDX-License-Identifier: Apache-2.0

package updater

import (
	"cmp"
	"fmt"
	"os"
	"path/filepath"
)

const (
	// Current names the link to the build that runs.
	Current = "current"
	// Previous names the link to the build an update goes back to.
	Previous = "previous"
	// versionsFolder holds one folder per build.
	versionsFolder = "versions"
	// downloadsFolder holds the archive a download writes, never the temporary folder of the system.
	downloadsFolder = "downloads"
	// backupsFolder holds the backups of the database taken before each switch.
	backupsFolder = "backups"
	// stateFolder holds the state file.
	stateFolder = "state"
	// partialSuffix ends the name of a file a step is still writing.
	partialSuffix = ".partial"
)

// Layout is an install root: one folder per build side by side, and the links naming the current and previous builds.
type Layout struct {
	Root string
	// Data names the folder that keeps the backups and the state in place of the install root.
	Data string
}

// Kept returns the folder that keeps the backups and the state, the install root with no data folder.
func (l Layout) Kept() string {
	return cmp.Or(l.Data, l.Root)
}

// MakeData makes a data folder only its owner opens, leaving one that is already there as it stands.
func (l Layout) MakeData() error {
	if l.Data == "" {
		return nil
	}
	if err := os.MkdirAll(l.Data, 0o700); err != nil {
		return fmt.Errorf("updater: make the data folder: %w", err)
	}
	return nil
}

// Build returns the folder of the build called name.
func (l Layout) Build(name string) string {
	return filepath.Join(l.Root, versionsFolder, name)
}

// Binary returns the gophenberg binary of the build called name.
func (l Layout) Binary(name string) string {
	return filepath.Join(l.Build(name), binaryName)
}

// Download returns the partial file the archive of release downloads into.
func (l Layout) Download(release string) string {
	return filepath.Join(l.Root, downloadsFolder, release+partialSuffix)
}

// Backups returns the folder that keeps the backups of the database.
func (l Layout) Backups() string {
	return filepath.Join(l.Kept(), backupsFolder)
}

// folders returns the install root, the folder that keeps the backups and the state, and each folder a job writes into.
func (l Layout) folders() []string {
	return []string{
		l.Root, filepath.Join(l.Root, versionsFolder), filepath.Join(l.Root, downloadsFolder), l.Kept(), l.Backups(),
		filepath.Dir(l.StateFile()),
	}
}

// Named returns the build the link called link names, like 0.20.0 for versions/0.20.0, refusing any other target.
func (l Layout) Named(link string) (string, error) {
	target, err := os.Readlink(filepath.Join(l.Root, link))
	if err != nil {
		return "", fmt.Errorf("updater: read the %s link: %w", link, err)
	}
	name := filepath.Base(target)
	if filepath.Join(versionsFolder, name) != target {
		return "", fmt.Errorf("updater: the %s link names %s, not a build in %s", link, target, versionsFolder)
	}
	return name, nil
}

// PointIn points the link called link of tree at the build called name, renaming a new link over the old one.
func PointIn(tree *os.Root, link, name string) error {
	staged := "." + link + partialSuffix
	_ = tree.Remove(staged)
	err := tree.Symlink(filepath.Join(versionsFolder, name), staged)
	if err == nil {
		err = tree.Rename(staged, link)
	}
	if err == nil {
		err = syncIn(tree, ".")
	}
	if err != nil {
		_ = tree.Remove(staged)
		return fmt.Errorf("updater: point %s at %s: %w", link, name, err)
	}
	return nil
}

// Point points the link called link at the build called name, renaming a new link over the old one.
func (l Layout) Point(link, name string) error {
	tree, err := os.OpenRoot(l.Root)
	if err != nil {
		return fmt.Errorf("updater: point %s at %s: %w", link, name, err)
	}
	defer func() { _ = tree.Close() }()
	return PointIn(tree, link, name)
}
