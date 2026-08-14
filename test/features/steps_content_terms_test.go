// SPDX-License-Identifier: Apache-2.0

package features_test

import (
	"context"
	"fmt"
	"net/http"

	"github.com/cucumber/godog"
)

// relatedAnswer is a term page as the content API answers it.
type relatedAnswer struct {
	Kind string `json:"kind"`
	Type struct {
		Key string `json:"key"`
	} `json:"type"`
	Item *struct {
		Title string `json:"title"`
		Path  string `json:"path"`
	} `json:"item"`
	Page *struct {
		Items []struct {
			Title string `json:"title"`
		} `json:"items"`
		Total   int `json:"total"`
		Page    int `json:"page"`
		PerPage int `json:"per_page"`
	} `json:"page"`
}

// theTermTypeExists registers a content type whose items list what points at them.
func theTermTypeExists(ctx context.Context, key, singular, plural, routeWord string) error {
	if err := theTypeExists(ctx, key, singular, plural, routeWord); err != nil {
		return err
	}
	w, err := worldOf(ctx)
	if err != nil {
		return err
	}
	if err := w.patchJSON(typesPath+"/"+key, `{"page_kind":"archive"}`); err != nil {
		return err
	}
	return w.expect(http.StatusOK)
}

// thePublishedCategory takes a category public.
func thePublishedCategory(ctx context.Context, title string) error {
	return publish(ctx, "category", title, "")
}

// thePublishedPostFiledUnder takes a post public filed under a category.
func thePublishedPostFiledUnder(ctx context.Context, title, target string) error {
	if err := thePublishedPost(ctx, title); err != nil {
		return err
	}
	if err := theAdministratorFilesUnderOne(ctx, title, target); err != nil {
		return err
	}
	w, err := worldOf(ctx)
	if err != nil {
		return err
	}
	return w.expect(http.StatusOK)
}

// thePostFiledUnderDraft stores an unpublished post filed under a category.
func thePostFiledUnderDraft(ctx context.Context, title, target string) error {
	if err := thePostExists(ctx, title); err != nil {
		return err
	}
	if err := theAdministratorFilesUnderOne(ctx, title, target); err != nil {
		return err
	}
	w, err := worldOf(ctx)
	if err != nil {
		return err
	}
	return w.expect(http.StatusOK)
}

// thePublishedPostFiledThroughBoth points both relation fields of a post at one category.
func thePublishedPostFiledThroughBoth(ctx context.Context, title, target string) error {
	if err := thePublishedPostFiledUnder(ctx, title, target); err != nil {
		return err
	}
	if err := theAdministratorFilesInFieldUnderOne(ctx, title, "series", target); err != nil {
		return err
	}
	w, err := worldOf(ctx)
	if err != nil {
		return err
	}
	return w.expect(http.StatusOK)
}

// threePublishedPostsFiledUnder takes three posts public under one category.
func threePublishedPostsFiledUnder(ctx context.Context, target string) error {
	for _, title := range []string{"First filed", "Second filed", "Third filed"} {
		if err := thePublishedPostFiledUnder(ctx, title, target); err != nil {
			return err
		}
	}
	return nil
}

// theAdministratorTrashes sends published content to the trash.
func theAdministratorTrashes(ctx context.Context, title string) error {
	return theAdministratorDeletes(ctx, title)
}

// thePublishedGuideFiledUnder takes a guide public filed under a category.
func thePublishedGuideFiledUnder(ctx context.Context, title, target string) error {
	if err := publish(ctx, "guide", title, ""); err != nil {
		return err
	}
	if err := theAdministratorFilesUnderOne(ctx, title, target); err != nil {
		return err
	}
	w, err := worldOf(ctx)
	if err != nil {
		return err
	}
	return w.expect(http.StatusOK)
}

// aVisitorResolvesOneAtATime asks the content API for an address a single item at a time.
func aVisitorResolvesOneAtATime(ctx context.Context, address string) error {
	w, err := worldOf(ctx)
	if err != nil {
		return err
	}
	return w.get("/api/content/v1/resolve?per_page=1&path=" + address)
}

// termAnswer returns the term page the last resolve answered with.
func termAnswer(ctx context.Context) (relatedAnswer, error) {
	w, err := worldOf(ctx)
	if err != nil {
		return relatedAnswer{}, err
	}
	if err := w.expect(http.StatusOK); err != nil {
		return relatedAnswer{}, fmt.Errorf("resolving the term page: %w", err)
	}
	var held relatedAnswer
	if err := w.answer.decode(&held); err != nil {
		return relatedAnswer{}, err
	}
	return held, nil
}

// theAnswerIsATermOf asserts the resolver named a term page of the type.
func theAnswerIsATermOf(ctx context.Context, key string) error {
	held, err := termAnswer(ctx)
	if err != nil {
		return err
	}
	if held.Kind != "term" || held.Type.Key != key {
		return fmt.Errorf("the answer is a %q of %q, want a term of %q", held.Kind, held.Type.Key, key)
	}
	return nil
}

// theAnswerCarriesTheItem asserts the term page carries the addressed item itself.
func theAnswerCarriesTheItem(ctx context.Context, title string) error {
	held, err := termAnswer(ctx)
	if err != nil {
		return err
	}
	if held.Item == nil || held.Item.Title != title {
		return fmt.Errorf("the answer carries %v, want the item %q", held.Item, title)
	}
	return nil
}

// isAmongTheRelatedContent asserts the term page lists the titled content.
func isAmongTheRelatedContent(ctx context.Context, title string) error {
	held, err := termAnswer(ctx)
	if err != nil {
		return err
	}
	if held.Page == nil {
		return fmt.Errorf("the answer carries no page, want %q among the related content", title)
	}
	for _, listed := range held.Page.Items {
		if listed.Title == title {
			return nil
		}
	}
	return fmt.Errorf("the term page lists %d items, none of them %q", len(held.Page.Items), title)
}

// isNotAmongTheRelatedContent asserts the term page leaves the titled content out.
func isNotAmongTheRelatedContent(ctx context.Context, title string) error {
	held, err := termAnswer(ctx)
	if err != nil {
		return err
	}
	if held.Page == nil {
		return fmt.Errorf("the answer carries no page, want a listing without %q", title)
	}
	for _, listed := range held.Page.Items {
		if listed.Title == title {
			return fmt.Errorf("the term page still lists %q", title)
		}
	}
	return nil
}

// noRelatedContentIsListed asserts the term page lists nothing at all.
func noRelatedContentIsListed(ctx context.Context) error {
	held, err := termAnswer(ctx)
	if err != nil {
		return err
	}
	if held.Page == nil {
		return fmt.Errorf("the answer carries no page, want an empty listing")
	}
	if len(held.Page.Items) != 0 {
		return fmt.Errorf("the term page lists %d items, want none", len(held.Page.Items))
	}
	return nil
}

// oneItemIsRelated asserts the term page lists exactly one item.
func oneItemIsRelated(ctx context.Context) error {
	held, err := termAnswer(ctx)
	if err != nil {
		return err
	}
	if held.Page == nil || len(held.Page.Items) != 1 {
		return fmt.Errorf("the term page lists %v, want exactly one item", held.Page)
	}
	return nil
}

// theTermPageHoldsAloneOf asserts which item a page carries and how many the term holds.
func theTermPageHoldsAloneOf(ctx context.Context, title string, total int) error {
	held, err := termAnswer(ctx)
	if err != nil {
		return err
	}
	if held.Page == nil || len(held.Page.Items) != 1 {
		return fmt.Errorf("the term page lists %v, want %q alone", held.Page, title)
	}
	if held.Page.Items[0].Title != title {
		return fmt.Errorf("the term page lists %q, want %q", held.Page.Items[0].Title, title)
	}
	if held.Page.Total != total {
		return fmt.Errorf("the term page counts %d, want %d behind it", held.Page.Total, total)
	}
	return nil
}

// theAnswerCarriesNoEntityTag asserts the term page offers no validator to cache against.
func theAnswerCarriesNoEntityTag(ctx context.Context) error {
	w, err := worldOf(ctx)
	if err != nil {
		return err
	}
	if held := w.answer.header.Get("ETag"); held != "" {
		return fmt.Errorf("the term page carries the validator %q, want none", held)
	}
	return nil
}

// initializeContentTerms registers the steps of the term page feature.
func initializeContentTerms(sc *godog.ScenarioContext) {
	sc.Before(provisionWorld)
	sc.After(retireWorld)
	sc.Given(`^a running Gophenberg with the default content types$`, aRunningGophenbergWithTheDefaultContentTypes)
	sc.Given(`^a signed in administrator$`, aSignedInAdministrator)
	sc.Given(
		`^the type "([^"]*)" labeled "([^"]*)" and "([^"]*)" under "([^"]*)" serving term pages$`,
		theTermTypeExists,
	)
	sc.Given(
		`^the "relation" field "([^"]*)" on "([^"]*)" targeting "([^"]*)" holding (one|many)$`,
		theRelationFieldHolding,
	)
	sc.Given(`^the published category "([^"]*)"$`, thePublishedCategory)
	sc.Given(`^the published post "([^"]*)"$`, thePublishedPost)
	sc.Given(`^the published post "([^"]*)" filed under "([^"]*)"$`, thePublishedPostFiledUnder)
	sc.Given(`^the post "([^"]*)" filed under "([^"]*)"$`, thePostFiledUnderDraft)
	sc.Given(
		`^the published post "([^"]*)" filed under "([^"]*)" through both fields$`,
		thePublishedPostFiledThroughBoth,
	)
	sc.Given(`^three published posts filed under "([^"]*)"$`, threePublishedPostsFiledUnder)
	sc.Given(`^the published guide "([^"]*)" filed under "([^"]*)"$`, thePublishedGuideFiledUnder)
	sc.Given(`^the type "([^"]*)" labeled "([^"]*)" and "([^"]*)" under "([^"]*)"$`, theTypeExists)
	sc.When(`^a visitor resolves "([^"]*)"$`, aVisitorResolves)
	sc.When(`^a visitor resolves "([^"]*)" one item at a time$`, aVisitorResolvesOneAtATime)
	sc.When(`^the administrator trashes "([^"]*)"$`, theAdministratorTrashes)
	sc.When(`^the administrator deactivates the type "([^"]*)"$`, theTypeIsDeactivated)
	sc.Then(`^the answer is a term of "([^"]*)"$`, theAnswerIsATermOf)
	sc.Then(`^the answer is an archive of "([^"]*)"$`, theAnswerIsAnArchiveOf)
	sc.Then(`^the answer carries the item "([^"]*)"$`, theAnswerCarriesTheItem)
	sc.Then(`^"([^"]*)" is among the related content$`, isAmongTheRelatedContent)
	sc.Then(`^"([^"]*)" is not among the related content$`, isNotAmongTheRelatedContent)
	sc.Then(`^no related content is listed$`, noRelatedContentIsListed)
	sc.Then(`^one item is related$`, oneItemIsRelated)
	sc.Then(`^it holds "([^"]*)" alone of (\d+) related items$`, theTermPageHoldsAloneOf)
	sc.Then(`^the answer carries no entity tag$`, theAnswerCarriesNoEntityTag)
	sc.Then(`^"([^"]*)" answers not found$`, theAddressAnswersNotFound)
}
