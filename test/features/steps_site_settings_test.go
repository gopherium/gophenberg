// SPDX-License-Identifier: Apache-2.0

package features_test

import (
	"context"
	"fmt"
	"net/http"
	"strings"

	"github.com/cucumber/godog"

	"github.com/gopherium/gophenberg/internal/media"
)

// publishedPath is where a visitor reads the content the site has published.
const publishedPath = "/api/content/v1/items"

// summaryMarker is the class the built in site stamps on every item it lists.
const summaryMarker = `class="gophenberg-site__summary"`

// listedPage is the page of published content the content API answers with.
type listedPage struct {
	Items []struct {
		Title string `json:"title"`
	} `json:"items"`
	Total   int `json:"total"`
	PerPage int `json:"per_page"`
}

// theAdministratorSetsThePageSizeTo asks the server to store how many items a listing carries.
func theAdministratorSetsThePageSizeTo(ctx context.Context, size int) error {
	w, err := worldOf(ctx)
	if err != nil {
		return err
	}
	return w.patchJSON(settingsPath, fmt.Sprintf(`{"content_per_page":%d}`, size))
}

// theAdministratorSetsThePictureQualityTo asks the server to store the quality copies are saved at.
func theAdministratorSetsThePictureQualityTo(ctx context.Context, quality int) error {
	w, err := worldOf(ctx)
	if err != nil {
		return err
	}
	return w.patchJSON(settingsPath, fmt.Sprintf(`{"jpeg_quality":%d}`, quality))
}

// aVisitorListsThePublishedContent reads the published content without naming a page size.
func aVisitorListsThePublishedContent(ctx context.Context) error {
	w, err := visit(ctx, publishedPath)
	if err != nil {
		return err
	}
	return w.expect(http.StatusOK)
}

// aVisitorListsThePublishedContentOneAtATime reads the published content naming its own page size.
func aVisitorListsThePublishedContentOneAtATime(ctx context.Context) error {
	w, err := visit(ctx, publishedPath+"?per_page=1")
	if err != nil {
		return err
	}
	return w.expect(http.StatusOK)
}

// theListingCarriesItemsOutOf asserts the page holds the items it should out of the whole.
func theListingCarriesItemsOutOf(ctx context.Context, carried, total int) error {
	w, err := worldOf(ctx)
	if err != nil {
		return err
	}
	var listed listedPage
	if err := w.answer.decode(&listed); err != nil {
		return err
	}
	if len(listed.Items) != carried || listed.Total != total {
		return fmt.Errorf("the listing carries %d items out of %d, want %d out of %d",
			len(listed.Items), listed.Total, carried, total)
	}
	return nil
}

// theListingOffersPagesOf asserts the page size the listing reports back.
func theListingOffersPagesOf(ctx context.Context, size int) error {
	w, err := worldOf(ctx)
	if err != nil {
		return err
	}
	var listed listedPage
	if err := w.answer.decode(&listed); err != nil {
		return err
	}
	if listed.PerPage != size {
		return fmt.Errorf("the listing offers pages of %d, want %d", listed.PerPage, size)
	}
	return nil
}

// theAddressCarriesSummaries asserts the built in site listed that many items at the address.
func theAddressCarriesSummaries(ctx context.Context, address string, carried int) error {
	w, err := visit(ctx, address)
	if err != nil {
		return err
	}
	if err := w.expect(http.StatusOK); err != nil {
		return err
	}
	if listed := strings.Count(string(w.answer.body), summaryMarker); listed != carried {
		return fmt.Errorf("the page at %q lists %d items, want %d", address, listed, carried)
	}
	return nil
}

// theCopiesOfTheSecondPictureWeighLess asserts every derived copy shrank at the lower quality.
func theCopiesOfTheSecondPictureWeighLess(ctx context.Context) error {
	first, second, err := twoStoredPictures(ctx)
	if err != nil {
		return err
	}
	for _, name := range []string{"thumbnail", "medium", "large"} {
		lighter, heavier := second.Sizes[name].Filesize, first.Sizes[name].Filesize
		if lighter == 0 || heavier == 0 {
			return fmt.Errorf("the pictures carry no %q copy to weigh", name)
		}
		if lighter >= heavier {
			return fmt.Errorf("the %q copy weighs %d at the lower quality, want less than %d",
				name, lighter, heavier)
		}
	}
	return nil
}

// bothPicturesKeepTheSameOriginal asserts the file each upload sent was stored untouched.
func bothPicturesKeepTheSameOriginal(ctx context.Context) error {
	first, second, err := twoStoredPictures(ctx)
	if err != nil {
		return err
	}
	kept, sent := second.Sizes["full"].Filesize, first.Sizes["full"].Filesize
	if kept == 0 || sent == 0 {
		return fmt.Errorf("the pictures carry no original to weigh")
	}
	if kept != sent {
		return fmt.Errorf("the originals weigh %d and %d, want the upload stored as sent", sent, kept)
	}
	return nil
}

// twoStoredPictures returns the first and second stored pictures in the order they were uploaded.
func twoStoredPictures(ctx context.Context) (media.Media, media.Media, error) {
	w, err := worldOf(ctx)
	if err != nil {
		return media.Media{}, media.Media{}, err
	}
	items, err := storedMedia(w)
	if err != nil {
		return media.Media{}, media.Media{}, err
	}
	if len(items) != 2 {
		return media.Media{}, media.Media{}, fmt.Errorf("the library holds %d pictures, want two", len(items))
	}
	return items[1], items[0], nil
}

// initializeSiteSettings registers the steps of the site settings feature.
func initializeSiteSettings(sc *godog.ScenarioContext) {
	sc.Before(provisionWorld)
	sc.After(retireWorld)
	sc.Given(`^a running Gophenberg with the default content types$`, aRunningGophenbergWithTheDefaultContentTypes)
	sc.Given(`^a signed in administrator$`, aSignedInAdministrator)
	sc.Given(`^the published post "([^"]*)"$`, thePublishedPost)
	sc.When(`^the administrator sets the page size to (\d+)$`, theAdministratorSetsThePageSizeTo)
	sc.When(`^the administrator sets the picture quality to (\d+)$`, theAdministratorSetsThePictureQualityTo)
	sc.When(`^a visitor lists the published content$`, aVisitorListsThePublishedContent)
	sc.When(`^a visitor lists the published content one item at a time$`, aVisitorListsThePublishedContentOneAtATime)
	sc.When(`^the administrator uploads a (\d+) by (\d+) pixel JPEG named "([^"]*)"$`, uploadsAPixelJPEG)
	sc.Then(`^the listing carries (\d+) items? out of (\d+)$`, theListingCarriesItemsOutOf)
	sc.Then(`^the listing offers pages of (\d+)$`, theListingOffersPagesOf)
	sc.Then(`^"([^"]*)" carries (\d+) summaries$`, theAddressCarriesSummaries)
	sc.Then(`^"([^"]*)" carries (\d+) summary$`, theAddressCarriesSummaries)
	sc.Then(`^the copies of the second picture weigh less$`, theCopiesOfTheSecondPictureWeighLess)
	sc.Then(`^both pictures keep the same original$`, bothPicturesKeepTheSameOriginal)
	sc.Then(`^the request is refused with the code "([^"]*)"$`, theRequestIsRefusedWithTheCode)
}
