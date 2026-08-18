// SPDX-License-Identifier: Apache-2.0

package features_test

import (
	"context"
	"fmt"
	"strings"

	"github.com/cucumber/godog"
)

// missingPath is an address no content lives at.
const missingPath = "/nothing-lives-here"

// aReaderOpensAPageThatDoesNotExist asks for an address holding no content.
func aReaderOpensAPageThatDoesNotExist(ctx context.Context) error {
	w, err := worldOf(ctx)
	if err != nil {
		return err
	}
	return w.get(missingPath)
}

// aReaderOpensAPageThatDoesNotExistPreferring asks for a missing address in a named language.
func aReaderOpensAPageThatDoesNotExistPreferring(ctx context.Context, languages string) error {
	w, err := worldOf(ctx)
	if err != nil {
		return err
	}
	return w.getPreferring(missingPath, languages)
}

// aReaderOpensTheLatestPosts asks for the listing of published content.
func aReaderOpensTheLatestPosts(ctx context.Context) error {
	w, err := worldOf(ctx)
	if err != nil {
		return err
	}
	return w.get("/")
}

// aPostPublishedOn takes a post public and stamps when it went out.
func aPostPublishedOn(ctx context.Context, day string) error {
	if err := aSignedInAdministrator(ctx); err != nil {
		return err
	}
	if err := publish(ctx, "post", "A dated post", ""); err != nil {
		return err
	}
	w, err := worldOf(ctx)
	if err != nil {
		return err
	}
	return w.publishedOn("A dated post", day)
}

// thePublicPageReads checks the served page carries the given words.
func thePublicPageReads(ctx context.Context, words string) error {
	w, err := worldOf(ctx)
	if err != nil {
		return err
	}
	if !strings.Contains(string(w.answer.body), words) {
		return fmt.Errorf("the page does not read %q", words)
	}
	return nil
}

// thePublicPageDeclaresTheLanguage checks the served page names its language.
func thePublicPageDeclaresTheLanguage(ctx context.Context, locale string) error {
	return thePublicPageReads(ctx, fmt.Sprintf(`<html lang="%s">`, locale))
}

// initializePublicSiteLanguage registers the steps the public language feature speaks.
func initializePublicSiteLanguage(sc *godog.ScenarioContext) {
	sc.Before(provisionWorld)
	sc.After(retireWorld)
	sc.Given(`^a running Gophenberg with the default content types$`, aRunningGophenbergWithTheDefaultContentTypes)
	sc.Given(`^the supported locales are "([^"]*)" and "([^"]*)"$`, theSupportedLocalesAre)
	sc.Given(`^no site default locale$`, noSiteDefaultLocale)
	sc.Given(`^the site default locale is "([^"]*)"$`, theSiteDefaultLocaleIs)
	sc.Given(`^a post published on "([^"]*)"$`, aPostPublishedOn)
	sc.When(`^a reader opens a page that does not exist$`, aReaderOpensAPageThatDoesNotExist)
	sc.When(`^a reader opens a page that does not exist preferring "([^"]*)"$`,
		aReaderOpensAPageThatDoesNotExistPreferring)
	sc.When(`^a reader opens the latest posts$`, aReaderOpensTheLatestPosts)
	sc.Then(`^the public page reads "([^"]*)"$`, thePublicPageReads)
	sc.Then(`^the public page declares the language "([^"]*)"$`, thePublicPageDeclaresTheLanguage)
}
