// SPDX-License-Identifier: Apache-2.0

package app

import (
	"context"
	"errors"
	"io"
	"reflect"
	"testing"

	"github.com/gopherium/framework/pluginkit"
	"github.com/gopherium/framework/pluginkit/pluginstoretest"
	"github.com/gopherium/framework/pluginkit/wire"

	"github.com/gopherium/gophenberg/internal/definitions"
)

// unreadableRecords is a plugin store in memory whose every read of the records fails.
type unreadableRecords struct {
	*pluginstoretest.Memory
}

// Records answers errDropped.
func (unreadableRecords) Records(context.Context) (map[string]pluginkit.PluginRecord, error) {
	return nil, errDropped
}

func TestInstalledPluginsListsEachManifestInTheStateItsRecordHolds(t *testing.T) {
	t.Parallel()

	installed := installedPlugins{
		manifests: []wire.Manifest{
			{ID: "notes", Name: "Notes"},
			{ID: "feed", Name: "Feed", Description: "Serves your published posts as an RSS feed.", Version: "0.0.1"},
		},
		records: pluginstoretest.NewMemory(pluginkit.PluginRecord{
			ID: "feed", State: pluginkit.PluginPaused, Step: pluginkit.StepRegister, LastError: "the title is too long",
		}),
	}

	listed, err := installed.Installed(t.Context())

	want := []pluginkit.InstalledPlugin{
		{
			ID: "feed", Name: "Feed", Description: "Serves your published posts as an RSS feed.", Version: "0.0.1",
			State: pluginkit.PluginPaused, PausedStep: pluginkit.StepRegister, PausedReason: "the title is too long",
		},
		{ID: "notes", Name: "Notes", State: pluginkit.PluginActive},
	}
	if err != nil || !reflect.DeepEqual(listed, want) {
		t.Errorf("Installed() = %+v (%v), want %+v", listed, err, want)
	}
}

func TestInstalledPluginsAnswersAFailedReadOfTheRecords(t *testing.T) {
	t.Parallel()

	installed := installedPlugins{
		manifests: []wire.Manifest{{ID: "feed", Name: "Feed"}},
		records:   unreadableRecords{Memory: pluginstoretest.NewMemory()},
	}

	listed, err := installed.Installed(t.Context())

	if !errors.Is(err, errDropped) || listed != nil {
		t.Errorf("Installed() = %+v (%v), want no list and the failed read", listed, err)
	}
}

func TestServerConfigListsThePluginsTheManifestsNameInTheStateTheRecordsHold(t *testing.T) {
	t.Parallel()

	records := pluginstoretest.NewMemory(pluginkit.PluginRecord{ID: "notes", State: pluginkit.PluginInactive})
	start := boot{
		settings: timedConfig(), built: site{pluginRecords: records}, host: pluginkit.NewHost(),
		logger: testLogger(io.Discard), manifests: []wire.Manifest{{ID: "notes", Name: "Notes"}},
	}

	cfg := start.serverConfig(pluginStart{walked: definitions.Walked{}})

	if cfg.InstalledPlugins == nil {
		t.Fatal("InstalledPlugins = nil, want the plugins the manifests name")
	}
	listed, err := cfg.InstalledPlugins.Installed(t.Context())
	want := []pluginkit.InstalledPlugin{{ID: "notes", Name: "Notes", State: pluginkit.PluginInactive}}
	if err != nil || !reflect.DeepEqual(listed, want) {
		t.Errorf("InstalledPlugins.Installed() = %+v (%v), want %+v", listed, err, want)
	}
}
