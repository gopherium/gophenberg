// SPDX-License-Identifier: Apache-2.0

package features_test

import (
	"bytes"
	"context"
	"fmt"
	"image"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"regexp"

	"github.com/cucumber/godog"

	"github.com/gopherium/gophenberg/internal/media"
)

// mediaPath is the admin route uploads arrive over.
const mediaPath = "/api/media"

// quotedName pulls the quoted rendition names out of a step's list.
var quotedName = regexp.MustCompile(`"([^"]+)"`)

// renditionBounds caps each rendition's dimensions by its slug.
var renditionBounds = map[string]int{"thumbnail": 150, "medium": 300, "large": 1024, "full": 2560}

// uploadsMedia posts data as a media file, remembering the raw upload.
func uploadsMedia(ctx context.Context, filename string, data []byte) error {
	w, err := worldOf(ctx)
	if err != nil {
		return err
	}
	w.lastUpload = data
	return w.upload(mediaPath, "file", filename, data)
}

// uploadsAPixelJPEG uploads a photo of the given size.
func uploadsAPixelJPEG(ctx context.Context, width, height int, name string) error {
	photo, err := jpegImage(width, height)
	if err != nil {
		return err
	}
	return uploadsMedia(ctx, name, photo)
}

// uploadsAnOrientedJPEG uploads a wide photo carrying the given orientation tag.
func uploadsAnOrientedJPEG(ctx context.Context, tag int) error {
	photo, err := orientedJPEG(uint16(tag))
	if err != nil {
		return err
	}
	return uploadsMedia(ctx, "sideways.jpg", photo)
}

// uploadsAnAnimatedGIF uploads a two frame animation.
func uploadsAnAnimatedGIF(ctx context.Context, name string) error {
	animation, err := animatedGIF()
	if err != nil {
		return err
	}
	return uploadsMedia(ctx, name, animation)
}

// uploadsAPDF uploads a minimal document.
func uploadsAPDF(ctx context.Context, name string) error {
	return uploadsMedia(ctx, name, pdfDocument())
}

// uploadsAValidJPEGTwice uploads the same photo under the same name twice.
func uploadsAValidJPEGTwice(ctx context.Context, name string) error {
	photo, err := jpegImage(400, 300)
	if err != nil {
		return err
	}
	if err := uploadsMedia(ctx, name, photo); err != nil {
		return err
	}
	return uploadsMedia(ctx, name, photo)
}

// uploadsAFlawedFile uploads a file carrying the named flaw.
func uploadsAFlawedFile(ctx context.Context, flaw string) error {
	name, data, err := flawedUpload(flaw)
	if err != nil {
		return err
	}
	return uploadsMedia(ctx, name, data)
}

// noAdministratorIsSignedIn gives the scenario a client without a session.
func noAdministratorIsSignedIn(ctx context.Context) error {
	w, err := worldOf(ctx)
	if err != nil {
		return err
	}
	if err := w.running(); err != nil {
		return err
	}
	w.visitor = &http.Client{Transport: w.site.Client().Transport}
	return nil
}

// aVisitorPostsAFile posts a small upload without a session.
func aVisitorPostsAFile(ctx context.Context) error {
	w, err := worldOf(ctx)
	if err != nil {
		return err
	}
	if w.visitor == nil {
		return fmt.Errorf("the scenario gave the visitor no client")
	}
	photo, err := jpegImage(4, 4)
	if err != nil {
		return err
	}
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	part, err := writer.CreateFormFile("file", "harbor.jpg")
	if err != nil {
		return fmt.Errorf("building the upload: %w", err)
	}
	if _, err := part.Write(photo); err != nil {
		return fmt.Errorf("writing the upload: %w", err)
	}
	if err := writer.Close(); err != nil {
		return fmt.Errorf("closing the upload: %w", err)
	}
	request, err := http.NewRequest(http.MethodPost, w.site.URL+mediaPath, &body)
	if err != nil {
		return fmt.Errorf("building the request: %w", err)
	}
	request.Header.Set("Content-Type", writer.FormDataContentType())
	response, err := w.visitor.Do(request)
	if err != nil {
		return fmt.Errorf("posting as a visitor: %w", err)
	}
	defer func() { _ = response.Body.Close() }()
	w.answer = &answer{status: response.StatusCode}
	return nil
}

// storedMedia returns every item the scenario's library holds, newest first.
func storedMedia(w *world) ([]media.Media, error) {
	items, _, err := w.mediaStore.List(context.Background(), media.Filter{Page: 1, PerPage: 1000})
	if err != nil {
		return nil, fmt.Errorf("listing the library: %w", err)
	}
	return items, nil
}

// mediaNamed returns the stored items of the given kind carrying the title.
func mediaNamed(w *world, kind media.Type, name string) ([]media.Media, error) {
	items, err := storedMedia(w)
	if err != nil {
		return nil, err
	}
	named := make([]media.Media, 0, 1)
	for _, item := range items {
		if item.Type == kind && item.Title == name {
			named = append(named, item)
		}
	}
	return named, nil
}

// oneMediaNamed returns the single stored item of the given kind carrying the title.
func oneMediaNamed(ctx context.Context, kind media.Type, name string) (*world, media.Media, error) {
	w, err := worldOf(ctx)
	if err != nil {
		return nil, media.Media{}, err
	}
	named, err := mediaNamed(w, kind, name)
	if err != nil {
		return nil, media.Media{}, err
	}
	if len(named) != 1 {
		return nil, media.Media{}, fmt.Errorf("the library lists %q %d times, want once", name, len(named))
	}
	return w, named[0], nil
}

// storedBytes reads a library-relative file from the scenario's media directory.
func (w *world) storedBytes(file string) ([]byte, error) {
	data, err := os.ReadFile(filepath.Join(w.mediaDir, filepath.FromSlash(file)))
	if err != nil {
		return nil, fmt.Errorf("reading the stored file: %w", err)
	}
	return data, nil
}

// theLibraryListsOneImageNamed asserts one stored image carries the name.
func theLibraryListsOneImageNamed(ctx context.Context, name string) error {
	_, _, err := oneMediaNamed(ctx, media.TypeImage, name)
	return err
}

// theLibraryListsOneFileNamed asserts one stored plain file carries the name.
func theLibraryListsOneFileNamed(ctx context.Context, name string) error {
	_, _, err := oneMediaNamed(ctx, media.TypeFile, name)
	return err
}

// theImageOffersRenditions asserts the single image carries exactly the named renditions.
func theImageOffersRenditions(ctx context.Context, names string) error {
	w, err := worldOf(ctx)
	if err != nil {
		return err
	}
	items, err := storedMedia(w)
	if err != nil {
		return err
	}
	if len(items) != 1 {
		return fmt.Errorf("the library holds %d items, want the uploaded image alone", len(items))
	}
	wanted := quotedName.FindAllStringSubmatch(names, -1)
	if len(wanted) != len(items[0].Sizes) {
		return fmt.Errorf("the image offers %v, want %d renditions", items[0].Sizes, len(wanted))
	}
	for _, name := range wanted {
		if _, offered := items[0].Sizes[name[1]]; !offered {
			return fmt.Errorf("the image offers %v, want %q among them", items[0].Sizes, name[1])
		}
	}
	return nil
}

// everyRenditionFitsItsBoundingBox asserts each rendition obeys its slug's bound and exists on disk.
func everyRenditionFitsItsBoundingBox(ctx context.Context) error {
	w, err := worldOf(ctx)
	if err != nil {
		return err
	}
	items, err := storedMedia(w)
	if err != nil {
		return err
	}
	if len(items) != 1 {
		return fmt.Errorf("the library holds %d items, want the uploaded image alone", len(items))
	}
	for slug, rendition := range items[0].Sizes {
		if err := renditionFits(w, slug, rendition); err != nil {
			return err
		}
	}
	return nil
}

// renditionFits asserts one rendition obeys its slug's bound and exists on disk.
func renditionFits(w *world, slug string, rendition media.Rendition) error {
	bound, known := renditionBounds[slug]
	if !known {
		return fmt.Errorf("the image offers a rendition %q no bound is set for", slug)
	}
	if rendition.Width > bound || rendition.Height > bound {
		return fmt.Errorf("%q measures %dx%d, want it within %d", slug, rendition.Width, rendition.Height, bound)
	}
	if slug == "thumbnail" && (rendition.Width != 150 || rendition.Height != 150) {
		return fmt.Errorf("the thumbnail measures %dx%d, want an exact 150x150 crop", rendition.Width, rendition.Height)
	}
	if _, err := w.storedBytes(rendition.File); err != nil {
		return err
	}
	return nil
}

// theStoredImageIsTallerThanWide asserts the stored pixels stand upright.
func theStoredImageIsTallerThanWide(ctx context.Context) error {
	w, item, err := oneMediaNamed(ctx, media.TypeImage, "sideways")
	if err != nil {
		return err
	}
	if item.Height <= item.Width {
		return fmt.Errorf("the library holds %dx%d, want it taller than wide", item.Width, item.Height)
	}
	data, err := w.storedBytes(item.File)
	if err != nil {
		return err
	}
	cfg, _, err := image.DecodeConfig(bytes.NewReader(data))
	if err != nil {
		return fmt.Errorf("reading the stored image: %w", err)
	}
	if cfg.Height <= cfg.Width {
		return fmt.Errorf("the stored file measures %dx%d, want it taller than wide", cfg.Width, cfg.Height)
	}
	return nil
}

// theStoredImageCarriesNoOrientationTag asserts the stored file dropped its EXIF orientation.
func theStoredImageCarriesNoOrientationTag(ctx context.Context) error {
	w, item, err := oneMediaNamed(ctx, media.TypeImage, "sideways")
	if err != nil {
		return err
	}
	data, err := w.storedBytes(item.File)
	if err != nil {
		return err
	}
	if bytes.Contains(data, []byte("Exif\x00\x00")) {
		return fmt.Errorf("the stored file still carries an EXIF segment, want the tag dropped")
	}
	return nil
}

// theFullRenditionIsAtMost asserts the display copy is bounded to the given width.
func theFullRenditionIsAtMost(ctx context.Context, width int) error {
	w, err := worldOf(ctx)
	if err != nil {
		return err
	}
	items, err := storedMedia(w)
	if err != nil {
		return err
	}
	if len(items) != 1 {
		return fmt.Errorf("the library holds %d items, want the uploaded image alone", len(items))
	}
	full, offered := items[0].Sizes["full"]
	if !offered {
		return fmt.Errorf("the image offers %v, want a full rendition", items[0].Sizes)
	}
	if full.Width > width {
		return fmt.Errorf("the full rendition is %d pixels wide, want at most %d", full.Width, width)
	}
	return nil
}

// theOriginalUploadIsKeptOnDisk asserts the stored file is the upload untouched.
func theOriginalUploadIsKeptOnDisk(ctx context.Context) error {
	return theStoredFileIsByteForByte(ctx)
}

// theImageOffersNoRenditions asserts the single image carries no derived copies.
func theImageOffersNoRenditions(ctx context.Context) error {
	w, err := worldOf(ctx)
	if err != nil {
		return err
	}
	items, err := storedMedia(w)
	if err != nil {
		return err
	}
	if len(items) != 1 {
		return fmt.Errorf("the library holds %d items, want the uploaded image alone", len(items))
	}
	if len(items[0].Sizes) != 0 {
		return fmt.Errorf("the image offers %v, want no renditions", items[0].Sizes)
	}
	return nil
}

// theStoredFileIsByteForByte asserts the stored file equals the last upload exactly.
func theStoredFileIsByteForByte(ctx context.Context) error {
	w, err := worldOf(ctx)
	if err != nil {
		return err
	}
	items, err := storedMedia(w)
	if err != nil {
		return err
	}
	if len(items) != 1 {
		return fmt.Errorf("the library holds %d items, want the upload alone", len(items))
	}
	data, err := w.storedBytes(items[0].File)
	if err != nil {
		return err
	}
	if !bytes.Equal(data, w.lastUpload) {
		return fmt.Errorf("the stored file differs from the upload, want it byte for byte")
	}
	return nil
}

// theLibraryListsTwoImagesNamed asserts two stored images carry the name.
func theLibraryListsTwoImagesNamed(ctx context.Context, name string) error {
	w, err := worldOf(ctx)
	if err != nil {
		return err
	}
	named, err := mediaNamed(w, media.TypeImage, name)
	if err != nil {
		return err
	}
	if len(named) != 2 {
		return fmt.Errorf("the library lists %q %d times, want both uploads", name, len(named))
	}
	return nil
}

// theirStoredFilesDiffer asserts the two stored images live in different files.
func theirStoredFilesDiffer(ctx context.Context) error {
	w, err := worldOf(ctx)
	if err != nil {
		return err
	}
	items, err := storedMedia(w)
	if err != nil {
		return err
	}
	if len(items) != 2 {
		return fmt.Errorf("the library holds %d items, want the two uploads", len(items))
	}
	if items[0].File == items[1].File {
		return fmt.Errorf("both uploads share the file %q, want distinct files", items[0].File)
	}
	for _, item := range items {
		if _, err := w.storedBytes(item.File); err != nil {
			return err
		}
	}
	return nil
}

// theLibraryHoldsNothing asserts no media item was stored.
func theLibraryHoldsNothing(ctx context.Context) error {
	w, err := worldOf(ctx)
	if err != nil {
		return err
	}
	items, err := storedMedia(w)
	if err != nil {
		return err
	}
	if len(items) != 0 {
		return fmt.Errorf("the library holds %d items, want none", len(items))
	}
	return nil
}

// theMediaDirectoryHoldsNoTrace asserts a refused upload left nothing on disk.
func theMediaDirectoryHoldsNoTrace(ctx context.Context) error {
	w, err := worldOf(ctx)
	if err != nil {
		return err
	}
	left := 0
	err = filepath.WalkDir(w.mediaDir, func(path string, _ os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if path != w.mediaDir {
			left++
		}
		return nil
	})
	if err != nil {
		return fmt.Errorf("walking the media directory: %w", err)
	}
	if left != 0 {
		return fmt.Errorf("the media directory holds %d entries, want the refused upload to leave nothing", left)
	}
	return nil
}

// theRequestIsRefusedAsUnauthenticated asserts the server demanded a session.
func theRequestIsRefusedAsUnauthenticated(ctx context.Context) error {
	w, err := worldOf(ctx)
	if err != nil {
		return err
	}
	if w.answer == nil {
		return fmt.Errorf("no request was made, want it refused")
	}
	if w.answer.status != http.StatusUnauthorized {
		return fmt.Errorf("status = %d, want %d", w.answer.status, http.StatusUnauthorized)
	}
	return nil
}

// initializeMediaUpload binds the steps of the media upload feature.
func initializeMediaUpload(sc *godog.ScenarioContext) {
	registerSharedSteps(sc)
	sc.Given(`^a running Gophenberg with an empty media directory$`, aRunningGophenberg)
	sc.Given(`^no administrator is signed in$`, noAdministratorIsSignedIn)
	sc.When(`^the administrator uploads a (\d+) by (\d+) pixel JPEG named "([^"]*)"$`, uploadsAPixelJPEG)
	sc.When(`^the administrator uploads a JPEG carrying orientation tag (\d+)$`, uploadsAnOrientedJPEG)
	sc.When(`^the administrator uploads an animated GIF named "([^"]*)"$`, uploadsAnAnimatedGIF)
	sc.When(`^the administrator uploads a PDF named "([^"]*)"$`, uploadsAPDF)
	sc.When(`^the administrator uploads a valid JPEG named "([^"]*)" twice$`, uploadsAValidJPEGTwice)
	sc.When(`^the administrator uploads a file that (.+)$`, uploadsAFlawedFile)
	sc.When(`^a visitor posts a file to the media endpoint$`, aVisitorPostsAFile)
	sc.Then(`^the upload is refused explaining (.+)$`, theUploadIsRefused)
	sc.Then(`^the library lists one image named "([^"]*)"$`, theLibraryListsOneImageNamed)
	sc.Then(`^the library lists one file named "([^"]*)"$`, theLibraryListsOneFileNamed)
	sc.Then(`^the image offers the renditions (.+)$`, theImageOffersRenditions)
	sc.Then(`^every rendition fits its bounding box$`, everyRenditionFitsItsBoundingBox)
	sc.Then(`^the stored image is taller than it is wide$`, theStoredImageIsTallerThanWide)
	sc.Then(`^the stored image carries no orientation tag$`, theStoredImageCarriesNoOrientationTag)
	sc.Then(`^the full rendition is at most (\d+) pixels wide$`, theFullRenditionIsAtMost)
	sc.Then(`^the original upload is kept on disk$`, theOriginalUploadIsKeptOnDisk)
	sc.Then(`^the image offers no renditions$`, theImageOffersNoRenditions)
	sc.Then(`^the stored file is byte for byte the upload$`, theStoredFileIsByteForByte)
	sc.Then(`^the library lists two images named "([^"]*)"$`, theLibraryListsTwoImagesNamed)
	sc.Then(`^their stored files differ$`, theirStoredFilesDiffer)
	sc.Then(`^the library holds nothing$`, theLibraryHoldsNothing)
	sc.Then(`^the media directory holds no trace of the upload$`, theMediaDirectoryHoldsNoTrace)
	sc.Then(`^the request is refused as unauthenticated$`, theRequestIsRefusedAsUnauthenticated)
}
