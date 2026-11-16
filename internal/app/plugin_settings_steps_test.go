// SPDX-License-Identifier: Apache-2.0

package app

import "testing"

// pluginSettingsFeature is the feature a site answers to while its plugins read the settings their manifests declare.
const pluginSettingsFeature = "../../test/features/features/plugin-settings.feature"

func TestPluginSettings(t *testing.T) {
	t.Parallel()

	runOperatorFeature(t, pluginSettingsFeature, postgresOperators())
}
