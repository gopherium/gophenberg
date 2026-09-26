// SPDX-License-Identifier: Apache-2.0

package features_test

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"time"

	"github.com/cucumber/godog"

	"github.com/gopherium/gophenberg/internal/mediahost"
)

// scratchFolder is the folder a scenario points the process's scratch files at, and the setting it replaced.
type scratchFolder struct {
	dir   string
	prior string
	held  bool
}

// uploadsMayReach raises the upload cap of the Gophenberg the scenario starts next.
func uploadsMayReach(ctx context.Context, megabytes int) error {
	w, err := worldOf(ctx)
	if err != nil {
		return err
	}
	if w.site != nil {
		return errors.New("the upload cap must be set before Gophenberg starts")
	}
	w.mediaFiles = mediahost.New(mediahost.Config{
		Dir: w.mediaDir, MaxSize: int64(megabytes) << 20, Settings: w.settings,
	})
	return nil
}

// theServerWritesScratchFilesToAWatchedFolder points the process's scratch files at a folder of the scenario's own.
func theServerWritesScratchFilesToAWatchedFolder(ctx context.Context) error {
	w, err := worldOf(ctx)
	if err != nil {
		return err
	}
	dir, err := os.MkdirTemp("", "gophenberg-scratch-")
	if err != nil {
		return err
	}
	prior, held := os.LookupEnv("TMPDIR")
	w.scratch = &scratchFolder{dir: dir, prior: prior, held: held}
	return os.Setenv("TMPDIR", dir)
}

// restoreScratchFolder puts the process's scratch setting back and removes the watched folder.
func restoreScratchFolder(ctx context.Context, _ *godog.Scenario, err error) (context.Context, error) {
	w, ok := ctx.Value(worldKey{}).(*world)
	if !ok || w.scratch == nil {
		return ctx, err
	}
	restored := os.Unsetenv("TMPDIR")
	if w.scratch.held {
		restored = os.Setenv("TMPDIR", w.scratch.prior)
	}
	return ctx, errors.Join(err, restored, os.RemoveAll(w.scratch.dir))
}

// paddedPDF returns a PDF document padded to the given size in megabytes.
func paddedPDF(megabytes int) []byte {
	document := pdfDocument()
	return append(document, bytes.Repeat([]byte("\n"), megabytes<<20-len(document))...)
}

// uploadsALargePDF uploads a PDF of the given size in megabytes.
func uploadsALargePDF(ctx context.Context, megabytes int, name string) error {
	return uploadsMedia(ctx, name, paddedPDF(megabytes))
}

// uploadsALargePDFUnderTheField uploads a PDF of the given size under a field the server does not read.
func uploadsALargePDFUnderTheField(ctx context.Context, megabytes int, name, field string) error {
	w, err := worldOf(ctx)
	if err != nil {
		return err
	}
	return w.upload(mediaPath, field, name, paddedPDF(megabytes))
}

// uploadsALargeArchiveThatIsNotAZip uploads a theme archive of the given size that holds no zip.
func uploadsALargeArchiveThatIsNotAZip(ctx context.Context, megabytes int, name string) error {
	w, err := worldOf(ctx)
	if err != nil {
		return err
	}
	return w.upload(themesPath, "theme", name+".zip", bytes.Repeat([]byte("x"), megabytes<<20))
}

// theWatchedFolderHoldsNoScratchFile asserts every scratch file the upload wrote is gone.
func theWatchedFolderHoldsNoScratchFile(ctx context.Context) error {
	w, err := worldOf(ctx)
	if err != nil {
		return err
	}
	if w.scratch == nil {
		return errors.New("the scenario watches no scratch folder")
	}
	deadline := time.Now().Add(readyWait)
	for {
		left, err := os.ReadDir(w.scratch.dir)
		if err != nil {
			return fmt.Errorf("reading the watched folder: %w", err)
		}
		if len(left) == 0 {
			return nil
		}
		if time.Now().After(deadline) {
			return fmt.Errorf("the watched folder still holds %d entries such as %q, want none", len(left), left[0].Name())
		}
		time.Sleep(readyPoll)
	}
}

// initializeUploadScratch binds the steps of the upload scratch file feature.
func initializeUploadScratch(sc *godog.ScenarioContext) {
	registerSharedSteps(sc)
	sc.After(restoreScratchFolder)
	sc.Given(`^uploads may reach (\d+) MB$`, uploadsMayReach)
	sc.Given(`^a running Gophenberg with an empty media directory$`, aRunningGophenberg)
	sc.Given(`^the server writes its scratch files to a watched folder$`, theServerWritesScratchFilesToAWatchedFolder)
	sc.When(`^the administrator uploads a (\d+) MB PDF named "([^"]*)"$`, uploadsALargePDF)
	sc.When(`^the administrator uploads a (\d+) MB PDF named "([^"]*)" under the field "([^"]*)"$`,
		uploadsALargePDFUnderTheField)
	sc.When(`^the administrator uploads a (\d+) MB theme archive named "([^"]*)" that is not a zip$`,
		uploadsALargeArchiveThatIsNotAZip)
	sc.Then(`^the upload is refused explaining (.+)$`, theUploadIsRefused)
	sc.Then(`^the library lists one file named "([^"]*)"$`, theLibraryListsOneFileNamed)
	sc.Then(`^the watched folder holds no scratch file$`, theWatchedFolderHoldsNoScratchFile)
}
