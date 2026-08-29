// SPDX-License-Identifier: Apache-2.0

package features_test

import (
	"github.com/cucumber/godog"
)

// initializeMediaValues registers the steps of the media values feature.
func initializeMediaValues(sc *godog.ScenarioContext) {
	sc.Before(provisionWorld)
	sc.After(retireWorld)
	sc.Given(`^a running Gophenberg with the default content types$`, aRunningGophenbergWithTheDefaultContentTypes)
	sc.Given(`^a signed in administrator$`, aSignedInAdministrator)
	sc.Given(`^the group "([^"]*)" placed on "([^"]*)"$`, theGroupExists)
	sc.Given(`^the published post "([^"]*)"$`, thePublishedPost)
	sc.Given(`^the published category "([^"]*)"$`, thePublishedCategory)
	sc.Given(
		`^the type "([^"]*)" labeled "([^"]*)" and "([^"]*)" under "([^"]*)" serving term pages$`,
		theTermTypeExists,
	)
	sc.Given(`^the many media field "([^"]*)" in "([^"]*)"$`, theManyMediaFieldExists)
	sc.Given(`^the administrator uploads a (\d+) by (\d+) pixel JPEG named "([^"]*)"$`, uploadsAPixelJPEG)
	sc.Given(`^the administrator uploads an animated GIF named "([^"]*)"$`, uploadsAnAnimatedGIF)
	sc.Given(`^the administrator describes the image "([^"]*)"$`, describesTheImage)
	sc.When(`^the administrator describes the image "([^"]*)"$`, describesTheImage)
	sc.When(`^a visitor resolves "([^"]*)"$`, aVisitorResolves)
}
