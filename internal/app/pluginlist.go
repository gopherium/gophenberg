// SPDX-License-Identifier: Apache-2.0

package app

import (
	"context"

	"github.com/gopherium/framework/pluginkit"
	"github.com/gopherium/framework/pluginkit/wire"
)

// installedPlugins lists the plugins the manifests name, in the state the records hold when it is asked.
type installedPlugins struct {
	manifests []wire.Manifest
	records   pluginkit.PluginStore
}

// Installed reads the records, then lists each plugin the manifests name with the state its record holds.
func (p installedPlugins) Installed(ctx context.Context) ([]pluginkit.InstalledPlugin, error) {
	records, err := p.records.Records(ctx)
	if err != nil {
		return nil, err
	}
	return pluginkit.Installed(p.manifests, records), nil
}
