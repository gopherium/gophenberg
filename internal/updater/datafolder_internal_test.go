// SPDX-License-Identifier: Apache-2.0

package updater

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gopherium/framework/gonsole"
)

func TestWritableMakesAPrivateDataFolderBeforeItProbes(t *testing.T) {
	t.Parallel()

	layout := Layout{Root: t.TempDir(), Data: filepath.Join(t.TempDir(), "gophenberg-data")}

	if err := (&run{layout: layout}).writable(); err != nil {
		t.Fatalf("writable() = %v, want nil", err)
	}

	info, err := os.Stat(layout.Data)
	if err != nil || info.Mode().Perm() != 0o700 {
		t.Errorf("the data folder stands as %v (%v), want it made with the mode 700", info, err)
	}
}

func TestWritableRefusesADataFolderItCannotMake(t *testing.T) {
	t.Parallel()

	taken := filepath.Join(t.TempDir(), "taken")
	if err := os.WriteFile(taken, nil, 0o600); err != nil {
		t.Fatalf("writing the file: %v", err)
	}
	layout := Layout{Root: t.TempDir(), Data: filepath.Join(taken, "gophenberg-data")}

	err := (&run{layout: layout}).writable()

	if err == nil || !strings.Contains(err.Error(), "updater: make the data folder") {
		t.Errorf("writable() = %v, want the data folder it could not make named", err)
	}
}

func TestRoomyRefusesADataFolderOnADiskWithNoRoom(t *testing.T) {
	t.Parallel()

	names := gonsole.Env{Prefix: "GOPHENBERG_", Getenv: func(string) string { return "" }}
	settings := Settings{DownloadCap: 1, names: names}
	layout := Layout{Root: t.TempDir(), Data: filepath.Join(t.TempDir(), "unread")}

	err := (&run{job: Job{Settings: settings}, layout: layout}).roomy()

	if err == nil || !strings.Contains(err.Error(), layout.Data+" has 0 MB free") {
		t.Errorf("roomy() = %v, want the data folder named", err)
	}
}
