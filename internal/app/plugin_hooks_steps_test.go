// SPDX-License-Identifier: Apache-2.0

package app

import "testing"

// pluginHooksFeature is the feature a site answers to while its plugins add tags to the head of its pages.
const pluginHooksFeature = "../../test/features/features/plugin-hooks.feature"

func TestPluginHooks(t *testing.T) {
	t.Parallel()

	runOperatorFeature(t, pluginHooksFeature, postgresOperators())
}
