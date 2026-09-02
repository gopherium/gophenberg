// SPDX-License-Identifier: Apache-2.0

package features_test

import (
	"os"
	"testing"

	"github.com/cucumber/godog"
)

// wipTag selects the scenarios an item has not implemented yet.
const wipTag = "@wip"

// runFeature runs one feature file as its own suite, refusing undefined steps.
func runFeature(t *testing.T, path string, initialize func(*godog.ScenarioContext)) {
	t.Helper()
	tags := "~" + wipTag
	if os.Getenv("GOPHENBERG_BDD_WIP") != "" {
		tags = ""
	}
	suite := godog.TestSuite{
		ScenarioInitializer: initialize,
		Options: &godog.Options{
			Format:   "pretty",
			Paths:    []string{path},
			Tags:     tags,
			Strict:   true,
			TestingT: t,
		},
	}
	if suite.Run() != 0 {
		t.Error("the feature did not pass")
	}
}

func TestContentTypes(t *testing.T) {
	runFeature(t, "features/content-types.feature", initializeContentTypes)
}

func TestContentFields(t *testing.T) {
	runFeature(t, "features/content-fields.feature", initializeContentFields)
}

func TestFieldGroups(t *testing.T) {
	runFeature(t, "features/field-groups.feature", initializeFieldGroups)
}

func TestFieldSettings(t *testing.T) {
	runFeature(t, "features/field-settings.feature", initializeFieldSettings)
}

func TestFieldKinds(t *testing.T) {
	runFeature(t, "features/field-kinds.feature", initializeFieldKinds)
}

func TestContainers(t *testing.T) {
	runFeature(t, "features/containers.feature", initializeContainers)
}

func TestMediaValues(t *testing.T) {
	runFeature(t, "features/media-values.feature", initializeMediaValues)
}

func TestRouteGate(t *testing.T) {
	runFeature(t, "features/roles-route-gate.feature", initializeRouteGate)
}

func TestOwnership(t *testing.T) {
	runFeature(t, "features/roles-ownership.feature", initializeOwnership)
}

func TestErrorCodes(t *testing.T) {
	runFeature(t, "features/error-codes.feature", initializeErrorCodes)
}

func TestLocaleResolution(t *testing.T) {
	runFeature(t, "features/locale-resolution.feature", initializeLocale)
}

func TestPublicSiteLanguage(t *testing.T) {
	runFeature(t, "features/public-site-language.feature", initializePublicSiteLanguage)
}

func TestContentRelations(t *testing.T) {
	runFeature(t, "features/content-relations.feature", initializeContentRelations)
}

func TestContentTerms(t *testing.T) {
	runFeature(t, "features/content-terms.feature", initializeContentTerms)
}

func TestContentHierarchy(t *testing.T) {
	runFeature(t, "features/content-hierarchy.feature", initializeContentHierarchy)
}

func TestContentServing(t *testing.T) {
	runFeature(t, "features/content-serving.feature", initializeContentServing)
}

func TestSiteSettings(t *testing.T) {
	runFeature(t, "features/site-settings.feature", initializeSiteSettings)
}

func TestThemeUpload(t *testing.T) {
	runFeature(t, "features/theme-upload.feature", initializeUpload)
}

func TestMediaUpload(t *testing.T) {
	runFeature(t, "features/media-upload.feature", initializeMediaUpload)
}

func TestMediaLibrary(t *testing.T) {
	runFeature(t, "features/media-library.feature", initializeMediaLibrary)
}

func TestMediaServing(t *testing.T) {
	runFeature(t, "features/media-serving.feature", initializeMediaServing)
}

func TestThemeActivation(t *testing.T) {
	runFeature(t, "features/theme-activation.feature", initializeActivation)
}

func TestThemeRollback(t *testing.T) {
	runFeature(t, "features/theme-rollback.feature", initializeRollback)
}
