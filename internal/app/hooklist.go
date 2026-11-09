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

// listedHooksOf reads the hooks each compiled plugin lists and defines from the manifest index, none from no index.
func listedHooksOf(index []byte) (listedHooks, error) {
	held := listedHooks{byPlugin: map[string][]string{}}
	if len(index) == 0 {
		return held, nil
	}
	var manifests []wire.Manifest
	if err := json.Unmarshal(index, &manifests); err != nil {
		return listedHooks{}, fmt.Errorf("reading the plugin manifest index: %w", err)
	}
	for _, manifest := range manifests {
		held.byPlugin[manifest.ID] = manifest.Hooks
		for _, defined := range manifest.Defines {
			held.defined = append(held.defined, defined.Hook)
		}
	}
	return held, nil
}
