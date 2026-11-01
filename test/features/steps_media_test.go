// SPDX-License-Identifier: Apache-2.0

package features_test

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"image"
	"mime/multipart"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"regexp"

	"github.com/cucumber/godog"

	"github.com/gopherium/gophenberg/internal/media"
	"github.com/gopherium/gophenberg/internal/mediahost"
)

// mediaPath is the admin route uploads arrive over.
const mediaPath = "/api/media"

// quotedName pulls the quoted size names out of a step's list.
var quotedName = regexp.MustCompile(`"([^"]+)"`)

// sizeBounds caps each image size's dimensions by its slug.
var sizeBounds = map[string]int{"thumbnail": 150, "medium": 300, "large": 1024, "full": 2560}

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

// thePixelBudgetIs sets the pixel budget of the Gophenberg the scenario starts next.
func thePixelBudgetIs(ctx context.Context, megapixels int) error {
	w, err := worldOf(ctx)
	if err != nil {
		return err
	}
	if w.site != nil {
		return errors.New("the pixel budget must be set before Gophenberg starts")
	}
	w.mediaConfig.MaxPixels = megapixels * 1_000_000
	w.mediaFiles = mediahost.New(w.mediaConfig)
	return nil
}

// uploadsAFramedGIF uploads a GIF whose one frame declares its own size on a canvas of another.
func uploadsAFramedGIF(ctx context.Context, name string, frameW, frameH, canvasW, canvasH int) error {
	return uploadsMedia(ctx, name, framedGIF(frameW, frameH, canvasW, canvasH))
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
	rows, _, err := w.mediaStore.List(context.Background(), media.Filter{Page: 1, PerPage: 1000})
	if err != nil {
		return nil, fmt.Errorf("listing the library: %w", err)
	}
	items := make([]media.Media, len(rows))
	for i, row := range rows {
		items[i] = row.Media
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

// theImageOffersSubSizes asserts the single image carries exactly the named sub-sizes.
func theImageOffersSubSizes(ctx context.Context, names string) error {
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
		return fmt.Errorf("the image offers %v, want %d sub-sizes", items[0].Sizes, len(wanted))
	}
	for _, name := range wanted {
		if _, offered := items[0].Sizes[name[1]]; !offered {
			return fmt.Errorf("the image offers %v, want %q among them", items[0].Sizes, name[1])
		}
	}
	return nil
}

// everySubSizeFitsItsBoundingBox asserts each sub-size obeys its slug's bound and exists on disk.
func everySubSizeFitsItsBoundingBox(ctx context.Context) error {
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
	for slug, subSize := range items[0].Sizes {
		if err := subSizeFits(w, slug, subSize); err != nil {
			return err
		}
	}
	return nil
}

// subSizeFits asserts one sub-size obeys its slug's bound and exists on disk.
func subSizeFits(w *world, slug string, subSize media.SubSize) error {
	bound, known := sizeBounds[slug]
	if !known {
		return fmt.Errorf("the image offers a sub-size %q no bound is set for", slug)
	}
	if subSize.Width > bound || subSize.Height > bound {
		return fmt.Errorf("%q measures %dx%d, want it within %d", slug, subSize.Width, subSize.Height, bound)
	}
	if slug == "thumbnail" && (subSize.Width != 150 || subSize.Height != 150) {
		return fmt.Errorf("the thumbnail measures %dx%d, want an exact 150x150 crop", subSize.Width, subSize.Height)
	}
	if _, err := w.storedBytes(subSize.File); err != nil {
		return err
	}
	return nil
}

// theOnlyItem returns the scenario's world and the single item its library holds.
func theOnlyItem(ctx context.Context) (*world, media.Media, error) {
	w, err := worldOf(ctx)
	if err != nil {
		return nil, media.Media{}, err
	}
	items, err := storedMedia(w)
	if err != nil {
		return nil, media.Media{}, err
	}
	if len(items) != 1 {
		return nil, media.Media{}, fmt.Errorf("the library holds %d items, want the uploaded image alone", len(items))
	}
	return w, items[0], nil
}

// theImageIsTallerThanWide asserts the single image measures upright.
func theImageIsTallerThanWide(ctx context.Context) error {
	_, item, err := theOnlyItem(ctx)
	if err != nil {
		return err
	}
	if item.Height <= item.Width {
		return fmt.Errorf("the library holds %dx%d, want it taller than wide", item.Width, item.Height)
	}
	return nil
}

// theFullSizeIsStoredAs asserts the single image's full size lives in a file of the given name.
func theFullSizeIsStoredAs(ctx context.Context, name string) error {
	_, item, err := theOnlyItem(ctx)
	if err != nil {
		return err
	}
	if got := path.Base(item.Sizes["full"].File); got != name {
		return fmt.Errorf("the full size is stored as %q, want %q", got, name)
	}
	return nil
}

// theSizeMeasures asserts one size of the single image measures width by height in its data and its file.
func theSizeMeasures(ctx context.Context, slug string, width, height int) error {
	w, item, err := theOnlyItem(ctx)
	if err != nil {
		return err
	}
	size, offered := item.Sizes[slug]
	if !offered {
		return fmt.Errorf("the image offers %v, want a %s size", item.Sizes, slug)
	}
	data, err := w.storedBytes(size.File)
	if err != nil {
		return err
	}
	cfg, _, err := image.DecodeConfig(bytes.NewReader(data))
	if err != nil {
		return fmt.Errorf("reading the %s size: %w", slug, err)
	}
	if size.Width != width || size.Height != height || cfg.Width != width || cfg.Height != height {
		return fmt.Errorf("the %s size measures %dx%d with a file of %dx%d, want %dx%d",
			slug, size.Width, size.Height, cfg.Width, cfg.Height, width, height)
	}
	return nil
}

// theFullSizeCarriesNoOrientationTag asserts the single image's full size holds no EXIF segment.
func theFullSizeCarriesNoOrientationTag(ctx context.Context) error {
	w, item, err := theOnlyItem(ctx)
	if err != nil {
		return err
	}
	data, err := w.storedBytes(item.Sizes["full"].File)
	if err != nil {
		return err
	}
	if bytes.Contains(data, []byte("Exif\x00\x00")) {
		return fmt.Errorf("the full size carries an EXIF segment, want none")
	}
	return nil
}

// uploadsASizedOrientedJPEG uploads a photo stored at width by height carrying the given orientation tag.
func uploadsASizedOrientedJPEG(ctx context.Context, width, height, tag int, name string) error {
	photo, err := orientedSizedJPEG(uint16(tag), width, height)
	if err != nil {
		return err
	}
	return uploadsMedia(ctx, name, photo)
}

// theFullSubSizeIsAtMost asserts the display copy is bounded to the given width.
func theFullSubSizeIsAtMost(ctx context.Context, width int) error {
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
		return fmt.Errorf("the image offers %v, want a full sub-size", items[0].Sizes)
	}
	if full.Width > width {
		return fmt.Errorf("the full sub-size is %d pixels wide, want at most %d", full.Width, width)
	}
	return nil
}

// theOriginalUploadIsKeptOnDisk asserts the stored file is the upload untouched.
func theOriginalUploadIsKeptOnDisk(ctx context.Context) error {
	return theStoredFileIsByteForByte(ctx)
}

// theImageOffersNoSubSizes asserts the single image carries no sub-sizes.
func theImageOffersNoSubSizes(ctx context.Context) error {
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
		return fmt.Errorf("the image offers %v, want no sub-sizes", items[0].Sizes)
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
	sc.Given(`^the pixel budget is (\d+) megapixels$`, thePixelBudgetIs)
	sc.When(`^the administrator uploads a GIF named "([^"]*)" holding a (\d+) by (\d+) pixel frame `+
		`on a (\d+) by (\d+) pixel canvas$`, uploadsAFramedGIF)
	sc.When(`^the administrator uploads a (\d+) by (\d+) pixel JPEG named "([^"]*)"$`, uploadsAPixelJPEG)
	sc.When(`^the administrator uploads a JPEG carrying orientation tag (\d+)$`, uploadsAnOrientedJPEG)
	sc.When(`^the administrator uploads an animated GIF named "([^"]*)"$`, uploadsAnAnimatedGIF)
	sc.When(`^the administrator uploads a PDF named "([^"]*)"$`, uploadsAPDF)
	sc.When(`^the administrator uploads a valid JPEG named "([^"]*)" twice$`, uploadsAValidJPEGTwice)
	sc.When(`^the administrator uploads a file that (.+)$`, uploadsAFlawedFile)
	sc.When(`^a visitor posts a file to the media endpoint$`, aVisitorPostsAFile)
	sc.Then(`^the request is refused with the code "([^"]*)"$`, theRequestIsRefusedWithTheCode)
	sc.Then(`^the error names "([^"]*)" under "([^"]*)"$`, theErrorNamesUnder)
	sc.Then(`^the error carries no data$`, theErrorCarriesNoData)
	sc.Then(`^the upload is refused explaining (.+)$`, theUploadIsRefused)
	sc.Then(`^the library lists one image named "([^"]*)"$`, theLibraryListsOneImageNamed)
	sc.Then(`^the library lists one file named "([^"]*)"$`, theLibraryListsOneFileNamed)
	sc.Then(`^the image offers the sizes (.+)$`, theImageOffersSubSizes)
	sc.Then(`^every size fits its bounding box$`, everySubSizeFitsItsBoundingBox)
	sc.When(`^the administrator uploads a (\d+) by (\d+) pixel JPEG carrying orientation tag (\d+) named "([^"]*)"$`,
		uploadsASizedOrientedJPEG)
	sc.Then(`^the image is taller than it is wide$`, theImageIsTallerThanWide)
	sc.Then(`^the full size is stored as "([^"]*)"$`, theFullSizeIsStoredAs)
	sc.Then(`^the (full|large|medium|thumbnail) size measures (\d+) by (\d+) pixels$`, theSizeMeasures)
	sc.Then(`^the full size carries no orientation tag$`, theFullSizeCarriesNoOrientationTag)
	sc.Then(`^the full size is at most (\d+) pixels wide$`, theFullSubSizeIsAtMost)
	sc.Then(`^the original upload is kept on disk$`, theOriginalUploadIsKeptOnDisk)
	sc.Then(`^the image offers no sizes$`, theImageOffersNoSubSizes)
	sc.Then(`^the stored file is byte for byte the upload$`, theStoredFileIsByteForByte)
	sc.Then(`^the library lists two images named "([^"]*)"$`, theLibraryListsTwoImagesNamed)
	sc.Then(`^their stored files differ$`, theirStoredFilesDiffer)
	sc.Then(`^the library holds nothing$`, theLibraryHoldsNothing)
	sc.Then(`^the media directory holds no trace of the upload$`, theMediaDirectoryHoldsNoTrace)
	sc.Then(`^the request is refused as unauthenticated$`, theRequestIsRefusedAsUnauthenticated)
}
