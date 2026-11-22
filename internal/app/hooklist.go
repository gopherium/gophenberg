// SPDX-License-Identifier: Apache-2.0

package app

import (
	"encoding/json"
	"fmt"

	"github.com/gopherium/framework/pluginkit/wire"
)

// listedHooks is what the manifest index says of hooks: the ones each plugin lists and every one a plugin defines.
type listedHooks struct {
	byPlugin map[string][]string
	defined  []string
}

// manifestIndex is what the site reads of the manifest index: every manifest, and each plugin's hooks and settings.
type manifestIndex struct {
	manifests []wire.Manifest
	hooks     listedHooks
	settings  map[string][]wire.Setting
}

// indexOf reads every manifest of the manifest index with each one's hooks and settings, none from no index.
func indexOf(index []byte) (manifestIndex, error) {
	held := manifestIndex{hooks: listedHooks{byPlugin: map[string][]string{}}, settings: map[string][]wire.Setting{}}
	if len(index) == 0 {
		return held, nil
	}
	var manifests []wire.Manifest
	if err := json.Unmarshal(index, &manifests); err != nil {
		return manifestIndex{}, fmt.Errorf("reading the plugin manifest index: %w", err)
	}
	held.manifests = manifests
	for _, manifest := range manifests {
		held.hooks.byPlugin[manifest.ID] = manifest.Hooks
		held.settings[manifest.ID] = manifest.Settings
		for _, defined := range manifest.Defines {
			held.hooks.defined = append(held.hooks.defined, defined.Hook)
		}
	}
	return held, nil
}
