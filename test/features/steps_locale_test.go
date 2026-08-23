// SPDX-License-Identifier: Apache-2.0

package features_test

import (
	"context"
	"errors"
	"fmt"
	"net/http"

	"github.com/cucumber/godog"
)

// localePath is where a client asks which language it is answered in.
const localePath = "/api/locale"

// settingsPath is where an administrator writes what the site chose for itself.
const settingsPath = "/api/settings"

// theSupportedLocalesAre records the languages the site may answer in.
func theSupportedLocalesAre(ctx context.Context, first, second string) error {
	w, err := worldOf(ctx)
	if err != nil {
		return err
	}
	w.locales = []string{first, second}
	return nil
}

// noSiteDefaultLocale leaves the site without a chosen language.
func noSiteDefaultLocale(ctx context.Context) error {
	w, err := worldOf(ctx)
	if err != nil {
		return err
	}
	return w.settings.Save(ctx, map[string]string{"locale.default": ""})
}

// theSiteDefaultLocaleIs stores the language the site answers in.
func theSiteDefaultLocaleIs(ctx context.Context, locale string) error {
	w, err := worldOf(ctx)
	if err != nil {
		return err
	}
	return w.settings.Save(ctx, map[string]string{"locale.default": locale})
}

// aSignedInAdministratorWhoseLocaleIs signs in and stores the reader's own language.
func aSignedInAdministratorWhoseLocaleIs(ctx context.Context, locale string) error {
	if err := aSignedInAdministrator(ctx); err != nil {
		return err
	}
	return theAdministratorSetsTheirLocaleTo(ctx, locale)
}

// theAdministratorSetsTheirLocaleTo asks the server to remember the reader's language.
func theAdministratorSetsTheirLocaleTo(ctx context.Context, locale string) error {
	w, err := worldOf(ctx)
	if err != nil {
		return err
	}
	return w.patchJSON(localePath, fmt.Sprintf(`{"locale":%q}`, locale))
}

// theAdministratorSetsTheSiteLocaleTo asks the server to store the site's own language.
func theAdministratorSetsTheSiteLocaleTo(ctx context.Context, locale string) error {
	w, err := worldOf(ctx)
	if err != nil {
		return err
	}
	return w.patchJSON(settingsPath, fmt.Sprintf(`{"locale_default":%q}`, locale))
}

// theAdministratorSignsInAgain starts a fresh session for the same reader.
func theAdministratorSignsInAgain(ctx context.Context) error {
	w, err := worldOf(ctx)
	if err != nil {
		return err
	}
	body := fmt.Sprintf(`{"email":%q,"password":%q}`, adminEmail, adminPassword)
	if err := w.postJSON("/api/auth/login", body); err != nil {
		return err
	}
	return w.expect(http.StatusOK)
}

// aVisitorAsksForTheLocalePreferring reads the locale with a browser language named.
func aVisitorAsksForTheLocalePreferring(ctx context.Context, languages string) error {
	w, err := worldOf(ctx)
	if err != nil {
		return err
	}
	return w.getPreferring(localePath, languages)
}

// aVisitorAsksForTheLocaleWithoutSigningIn reads the locale with no session at all.
func aVisitorAsksForTheLocaleWithoutSigningIn(ctx context.Context) error {
	w, err := worldOf(ctx)
	if err != nil {
		return err
	}
	return w.get(localePath)
}

// theAdministratorAsksForTheLocale reads the locale as the signed in reader.
func theAdministratorAsksForTheLocale(ctx context.Context) error {
	return aVisitorAsksForTheLocaleWithoutSigningIn(ctx)
}

// theLocaleAnsweredIs asserts the server named the language.
func theLocaleAnsweredIs(ctx context.Context, want string) error {
	w, err := worldOf(ctx)
	if err != nil {
		return err
	}
	if err := w.expect(http.StatusOK); err != nil {
		return err
	}
	var answered struct {
		Locale string `json:"locale"`
	}
	if err := w.answer.decode(&answered); err != nil {
		return err
	}
	if answered.Locale != want {
		return fmt.Errorf("the locale answered is %q, want %q", answered.Locale, want)
	}
	return nil
}

// theLocaleIsAnsweredWithoutError asserts an unauthenticated reader is answered.
func theLocaleIsAnsweredWithoutError(ctx context.Context) error {
	w, err := worldOf(ctx)
	if err != nil {
		return err
	}
	return w.expect(http.StatusOK)
}

// theReaderSettingsStoreCannotBeRead makes every reader preference lookup fail.
func theReaderSettingsStoreCannotBeRead(ctx context.Context) error {
	w, err := worldOf(ctx)
	if err != nil {
		return err
	}
	w.readers.fails = errors.New("features: the reader settings store is unreachable")
	return nil
}

// initializeLocale registers the steps of the locale resolution feature.
func initializeLocale(sc *godog.ScenarioContext) {
	sc.Before(provisionWorld)
	sc.After(retireWorld)
	sc.Given(`^a running Gophenberg with the default content types$`, aRunningGophenbergWithTheDefaultContentTypes)
	sc.Given(`^the supported locales are "([^"]*)" and "([^"]*)"$`, theSupportedLocalesAre)
	sc.Given(`^no site default locale$`, noSiteDefaultLocale)
	sc.Given(`^the reader settings store cannot be read$`, theReaderSettingsStoreCannotBeRead)
	sc.Given(`^the site default locale is "([^"]*)"$`, theSiteDefaultLocaleIs)
	sc.Given(`^a signed in administrator$`, aSignedInAdministrator)
	sc.Given(`^a signed in administrator whose locale is "([^"]*)"$`, aSignedInAdministratorWhoseLocaleIs)
	sc.When(`^a visitor asks for the locale preferring "([^"]*)"$`, aVisitorAsksForTheLocalePreferring)
	sc.When(`^a visitor asks for the locale without signing in$`, aVisitorAsksForTheLocaleWithoutSigningIn)
	sc.When(`^the administrator asks for the locale$`, theAdministratorAsksForTheLocale)
	sc.When(`^the administrator signs in again$`, theAdministratorSignsInAgain)
	sc.When(`^the administrator sets the site locale to "([^"]*)"$`, theAdministratorSetsTheSiteLocaleTo)
	sc.When(`^the administrator sets their locale to "([^"]*)"$`, theAdministratorSetsTheirLocaleTo)
	sc.Then(`^the locale answered is "([^"]*)"$`, theLocaleAnsweredIs)
	sc.Then(`^the locale is answered without an error$`, theLocaleIsAnsweredWithoutError)
	sc.Then(`^the request is refused with the code "([^"]*)"$`, theRequestIsRefusedWithTheCode)
}
