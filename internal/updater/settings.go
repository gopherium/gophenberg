// SPDX-License-Identifier: Apache-2.0

package updater

import (
	"cmp"
	"errors"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"github.com/gopherium/framework/gonsole"
)

// maxMegabytes is the largest size setting in megabytes, a pebibyte, so that sizes added up never overflow an int64.
const maxMegabytes = 1 << 30

// Settings is how a server install updates itself and checks each update at its first starts, as GOPHENBERG_ names it.
type Settings struct {
	Root            string
	Data            string
	DownloadCap     int64
	DownloadTimeout time.Duration
	DiskMargin      int64
	BackupsKept     int
	PgDump          string
	Attempts        int
	VerifyTimeout   time.Duration
	VersionsKept    int
	names           gonsole.Env
}

// SettingsFrom reads the update settings env holds, refusing a relative folder and any value out of range.
func SettingsFrom(env gonsole.Env) (Settings, error) {
	root, rootErr := gonsole.Parse(env, "INSTALL_ROOT", "", absolute)
	data, dataErr := gonsole.Parse(env, "DATA_DIR", "", absolute)
	timeout, timeoutErr := env.Duration("UPDATE_DOWNLOAD_TIMEOUT", 10*time.Minute)
	capMB, capErr := env.Count("UPDATE_DOWNLOAD_CAP_MB", 200, gonsole.AtMost(maxMegabytes))
	marginMB, marginErr := env.Count("UPDATE_DISK_MARGIN_MB", 64, gonsole.AllowZero(), gonsole.AtMost(maxMegabytes))
	kept, keptErr := env.Count("UPDATE_BACKUPS_KEEP", 3)
	check, checkErr := checkSettingsFrom(env)
	outsideErr := outsideRoot(env, root, data)
	if err := errors.Join(rootErr, dataErr, outsideErr, timeoutErr, capErr, marginErr, keptErr, checkErr); err != nil {
		return Settings{}, err
	}
	check.Root, check.Data, check.DownloadCap, check.DownloadTimeout = root, data, megabytes(capMB), timeout
	check.DiskMargin, check.BackupsKept = megabytes(marginMB), kept
	check.PgDump, check.names = cmp.Or(env.Value("PG_DUMP_BIN"), "pg_dump"), env
	return check, nil
}

// outsideRoot refuses a data folder that is the install root or stands inside it.
func outsideRoot(env gonsole.Env, root, data string) error {
	if root == "" || data == "" {
		return nil
	}
	relative, _ := filepath.Rel(root, data)
	if relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
		return nil
	}
	return fmt.Errorf("%s: must stand outside %s, got %q", env.Key("DATA_DIR"), env.Key("INSTALL_ROOT"), data)
}

// Layout returns the install root and the data folder the settings name.
func (s Settings) Layout() Layout {
	return Layout{Root: s.Root, Data: s.Data}
}

// checkSettingsFrom reads how the first starts of a new build check it: the attempts, the timeout and the builds kept.
func checkSettingsFrom(env gonsole.Env) (Settings, error) {
	attempts, attemptsErr := env.Count("UPDATE_ATTEMPTS", 3)
	verify, verifyErr := env.Duration("UPDATE_VERIFY_TIMEOUT", time.Minute)
	versions, versionsErr := env.Count("UPDATE_VERSIONS_KEEP", 2)
	err := errors.Join(attemptsErr, verifyErr, versionsErr)
	return Settings{Attempts: attempts, VerifyTimeout: verify, VersionsKept: versions}, err
}

// absolute reads a folder, refusing a path that does not start at the top of the file system.
func absolute(path string) (string, error) {
	if !filepath.IsAbs(path) {
		return "", fmt.Errorf("must be an absolute path, got %q", path)
	}
	return filepath.Clean(path), nil
}

// megabytes returns how many bytes n megabytes hold.
func megabytes(n int) int64 {
	return int64(n) << 20
}
