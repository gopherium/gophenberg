// SPDX-License-Identifier: Apache-2.0

package app

import "testing"

// pluginContentChangesFeature is the feature a site answers to while its plugins hear content changes.
const pluginContentChangesFeature = "../../test/features/features/plugin-content-changes.feature"

func TestPluginContentChanges(t *testing.T) {
	t.Parallel()

	runOperatorFeature(t, pluginContentChangesFeature, postgresOperators())
}
