// SPDX-License-Identifier: Apache-2.0

package app

import "testing"

// pluginRoutePanicsFeature is the feature a site answers to while a plugin's route panics.
const pluginRoutePanicsFeature = "../../test/features/features/plugin-route-panics.feature"

func TestPluginRoutePanics(t *testing.T) {
	t.Parallel()

	runOperatorFeature(t, pluginRoutePanicsFeature, postgresOperators())
}
