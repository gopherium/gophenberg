// SPDX-License-Identifier: Apache-2.0

package features_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"

	"github.com/cucumber/godog"
)

// narrowedRow is one row of a listing as its readers decode it.
type narrowedRow struct {
	Title  string                     `json:"title"`
	Fields map[string]json.RawMessage `json:"fields"`
}

// narrowedPage is a listing as its readers decode it.
type narrowedPage struct {
	Items []narrowedRow `json:"items"`
	Total int           `json:"total"`
}

// fieldQuery returns the query string naming the term, empty when the key is unnamed.
func fieldQuery(key string, values ...string) string {
	if key == "" {
		return ""
	}
	query := url.Values{}
	for _, value := range values {
		query.Add("field["+key+"]", value)
	}
	return "&" + query.Encode()
}

// theAdministratorListsContentWhere reads the admin listing narrowed by the term.
func theAdministratorListsContentWhere(ctx context.Context, key, value string) error {
	w, err := worldOf(ctx)
	if err != nil {
		return err
	}
	return w.get(contentPath + "?type=post" + fieldQuery(key, value))
}

// theAdministratorListsContentWhereTwice reads the admin listing naming one term twice.
func theAdministratorListsContentWhereTwice(ctx context.Context, key, first, second string) error {
	w, err := worldOf(ctx)
	if err != nil {
		return err
	}
	return w.get(contentPath + "?type=post" + fieldQuery(key, first, second))
}

// aVisitorListsPublishedWhere reads the published listing of the type narrowed by the term.
func aVisitorListsPublishedWhere(ctx context.Context, typeKey, key, value string) error {
	w, err := worldOf(ctx)
	if err != nil {
		return err
	}
	return w.get("/api/content/v1/items?type=" + typeKey + fieldQuery(key, value))
}

// listedAnswer returns the listing the last answer carries.
func listedAnswer(w *world) (narrowedPage, error) {
	if w.answer == nil {
		return narrowedPage{}, fmt.Errorf("nothing was listed yet")
	}
	var held narrowedPage
	if err := json.Unmarshal(w.answer.body, &held); err != nil {
		return narrowedPage{}, fmt.Errorf("reading the listing: %w", err)
	}
	return held, nil
}

// theListingHoldsOnly asserts the listing carries the one titled item.
func theListingHoldsOnly(ctx context.Context, title string) error {
	w, err := worldOf(ctx)
	if err != nil {
		return err
	}
	held, err := listedAnswer(w)
	if err != nil {
		return err
	}
	if len(held.Items) != 1 || held.Items[0].Title != title {
		return fmt.Errorf("the listing holds %v, want only %q", titlesListed(held), title)
	}
	return nil
}

// theListingHoldsItems asserts how many items the listing carries.
func theListingHoldsItems(ctx context.Context, count int) error {
	w, err := worldOf(ctx)
	if err != nil {
		return err
	}
	held, err := listedAnswer(w)
	if err != nil {
		return err
	}
	if len(held.Items) != count {
		return fmt.Errorf("the listing holds %v, want %d items", titlesListed(held), count)
	}
	return nil
}

// titlesListed returns the titles the listing carries.
func titlesListed(held narrowedPage) []string {
	titles := make([]string, len(held.Items))
	for i, item := range held.Items {
		titles[i] = item.Title
	}
	return titles
}

// theListedItemCarries asserts the listed item carries the value under the key.
func theListedItemCarries(ctx context.Context, title, value, key string) error {
	row, err := rowListed(ctx, title)
	if err != nil {
		return err
	}
	raw, found := row.Fields[key]
	if !found {
		return fmt.Errorf("the listed %q holds no %q, want %q", title, key, value)
	}
	if string(raw) != value {
		return fmt.Errorf("the listed %q holds %s in %q, want %q", title, raw, key, value)
	}
	return nil
}

// theListedItemCarriesNothingUnder asserts the listed item carries nothing under the key.
func theListedItemCarriesNothingUnder(ctx context.Context, title, key string) error {
	row, err := rowListed(ctx, title)
	if err != nil {
		return err
	}
	if raw, found := row.Fields[key]; found {
		return fmt.Errorf("the listed %q holds %s in %q, want nothing", title, raw, key)
	}
	return nil
}

// rowListed returns the listed row carrying the title.
func rowListed(ctx context.Context, title string) (narrowedRow, error) {
	w, err := worldOf(ctx)
	if err != nil {
		return narrowedRow{}, err
	}
	held, err := listedAnswer(w)
	if err != nil {
		return narrowedRow{}, err
	}
	for _, item := range held.Items {
		if item.Title == title {
			return item, nil
		}
	}
	return narrowedRow{}, fmt.Errorf("the listing holds %v, want %q among them", titlesListed(held), title)
}

// initializeFieldQuery registers the steps of the field query feature.
func initializeFieldQuery(sc *godog.ScenarioContext) {
	sc.Before(provisionWorld)
	sc.After(retireWorld)
	sc.Given(`^a running Gophenberg with the default content types$`, aRunningGophenbergWithTheDefaultContentTypes)
	sc.Given(`^a signed in administrator$`, aSignedInAdministrator)
	sc.Given(`^the group "([^"]*)" placed on "([^"]*)"$`, theGroupExists)
	sc.Given(`^the "([^"]*)" field "([^"]*)" in "([^"]*)" with settings:$`, theFieldWithSettingsExists)
	sc.Given(`^the post "([^"]*)"$`, thePostExists)
	sc.Given(`^the post "([^"]*)" holding:$`, thePostHoldsTheValues)
	sc.Given(`^the administrator publishes "([^"]*)"$`, theAdministratorPublishes)
	sc.When(`^the administrator lists content where "([^"]*)" is "([^"]*)"$`, theAdministratorListsContentWhere)
	sc.When(
		`^the administrator lists content where "([^"]*)" is "([^"]*)" and "([^"]*)" again$`,
		theAdministratorListsContentWhereTwice,
	)
	sc.When(
		`^a visitor lists published "([^"]*)" where "([^"]*)" is "([^"]*)"$`,
		aVisitorListsPublishedWhere,
	)
	sc.Then(`^the listing holds only "([^"]*)"$`, theListingHoldsOnly)
	sc.Then(`^the listing holds (\d+) items$`, theListingHoldsItems)
	sc.Then(`^the listed item "([^"]*)" carries "([^"]*)" under "([^"]*)"$`, theListedItemCarries)
	sc.Then(`^the listed item "([^"]*)" carries nothing under "([^"]*)"$`, theListedItemCarriesNothingUnder)
	sc.Then(`^the request is refused with the code "([^"]*)"$`, theRequestIsRefusedWithTheCode)
}
