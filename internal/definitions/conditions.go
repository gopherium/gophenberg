// SPDX-License-Identifier: Apache-2.0

package definitions

import (
	"github.com/gopherium/gophenberg/internal/content"
	"github.com/gopherium/gophenberg/sdk"
)

// declaredSettings returns the settings a plugin's field stands on, with its conditions and list flag folded in.
func declaredSettings(d sdk.FieldDeclaration) map[string]any {
	settings := make(map[string]any, len(d.Settings)+2)
	for name, value := range d.Settings {
		settings[name] = value
	}
	if rules := rulesOf(d.Conditions).Normalize(); len(rules) > 0 {
		settings[content.SettingConditions] = content.ConditionsSetting(rules)
	}
	if d.Listed {
		settings[content.SettingListed] = true
	}
	if len(settings) == 0 {
		return nil
	}
	return settings
}

// withheldConditions returns the settings without the rules a field is shown under.
func withheldConditions(settings map[string]any) map[string]any {
	held := make(map[string]any, len(settings))
	for name, value := range settings {
		if name != content.SettingConditions {
			held[name] = value
		}
	}
	if len(held) == 0 {
		return nil
	}
	return held
}
