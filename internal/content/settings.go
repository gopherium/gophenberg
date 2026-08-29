// SPDX-License-Identifier: Apache-2.0

package content

import (
	"errors"
	"fmt"
	"math"
	"sort"
)

// ErrSettingUnknown reports a setting the field's kind does not take.
var ErrSettingUnknown = errors.New("content: setting unknown")

// ErrSettingShape reports a setting holding a value of the wrong shape.
var ErrSettingShape = errors.New("content: setting shape")

// ErrSettingBounds reports settings that disagree with each other.
var ErrSettingBounds = errors.New("content: settings disagree")

// ErrFieldBounds reports a value falling outside the bounds its field carries.
var ErrFieldBounds = errors.New("content: value out of bounds")

// SettingDefault names the value the editor pre-fills a fresh item with.
const SettingDefault = "default"

// SettingInstructions names the help text shown under the control.
const SettingInstructions = "instructions"

// SettingPlaceholder names the ghost text an empty control shows.
const SettingPlaceholder = "placeholder"

// SettingMin names the smallest value a number field accepts.
const SettingMin = "min"

// SettingMax names the largest value a number field accepts.
const SettingMax = "max"

// SettingMaxLength names the longest text a text field accepts.
const SettingMaxLength = "maxlength"

// SettingStep names the increment the editor control moves by.
const SettingStep = "step"

// settingChecks returns the settings the kind takes, each with its shape check.
func settingChecks(kind FieldKind) map[string]func(value any) bool {
	held := map[string]func(value any) bool{SettingInstructions: settingString}
	switch kind {
	case FieldKindText:
		held[SettingDefault] = settingString
		held[SettingPlaceholder] = settingString
		held[SettingMaxLength] = settingWhole
	case FieldKindNumber:
		held[SettingDefault] = settingNumeric
		held[SettingPlaceholder] = settingString
		held[SettingMin] = settingNumeric
		held[SettingMax] = settingNumeric
		held[SettingStep] = settingPositive
	case FieldKindBoolean:
		held[SettingDefault] = settingBool
	}
	return held
}

// settingString reports whether the value is a string.
func settingString(value any) bool {
	_, held := value.(string)
	return held
}

// settingBool reports whether the value is a boolean.
func settingBool(value any) bool {
	_, held := value.(bool)
	return held
}

// settingNumber returns the value as a number, and whether it is one.
func settingNumber(value any) (float64, bool) {
	switch held := value.(type) {
	case float64:
		return held, true
	case float32:
		return float64(held), true
	case int:
		return float64(held), true
	case int32:
		return float64(held), true
	case int64:
		return float64(held), true
	default:
		return 0, false
	}
}

// settingNumeric reports whether the value is a number.
func settingNumeric(value any) bool {
	_, held := settingNumber(value)
	return held
}

// settingPositive reports whether the value is a number above zero.
func settingPositive(value any) bool {
	held, ok := settingNumber(value)
	return ok && held > 0
}

// settingWhole reports whether the value is a whole number above zero.
func settingWhole(value any) bool {
	held, ok := settingNumber(value)
	return ok && held > 0 && held == math.Trunc(held)
}

// ValidateSettings reports whether the settings are ones the kind takes, well shaped and agreeing.
func ValidateSettings(kind FieldKind, settings map[string]any) error {
	checks := settingChecks(kind)
	names := make([]string, 0, len(settings))
	for name := range settings {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		check, taken := checks[name]
		if !taken {
			return Refuse(ErrSettingUnknown, "setting_unknown",
				fmt.Sprintf("%s: %s on %s", ErrSettingUnknown, name, kind),
				Details{"setting": name, "kind": string(kind)})
		}
		if !check(settings[name]) {
			return Refuse(ErrSettingShape, "setting_shape",
				fmt.Sprintf("%s: %s", ErrSettingShape, name), Details{"setting": name, "kind": string(kind)})
		}
	}
	return settingsAgree(kind, settings)
}

// settingsAgree reports whether the kind's settings sit inside each other's bounds.
func settingsAgree(kind FieldKind, settings map[string]any) error {
	switch kind {
	case FieldKindNumber:
		return numberSettingsAgree(settings)
	case FieldKindText:
		return textSettingsAgree(settings)
	default:
		return nil
	}
}

// disagree returns the refusal naming the setting that falls outside the others.
func disagree(setting string) error {
	return Refuse(ErrSettingBounds, "setting_bounds",
		fmt.Sprintf("%s: %s", ErrSettingBounds, setting), Details{"setting": setting})
}

// numberSettingsAgree reports whether min, max and the default sit together.
func numberSettingsAgree(settings map[string]any) error {
	low, hasLow := settingNumber(settings[SettingMin])
	high, hasHigh := settingNumber(settings[SettingMax])
	if hasLow && hasHigh && low > high {
		return disagree(SettingMin)
	}
	chosen, hasChosen := settingNumber(settings[SettingDefault])
	if hasChosen && ((hasLow && chosen < low) || (hasHigh && chosen > high)) {
		return disagree(SettingDefault)
	}
	return nil
}

// textSettingsAgree reports whether the default fits inside maxlength.
func textSettingsAgree(settings map[string]any) error {
	chosen, hasChosen := settings[SettingDefault].(string)
	longest, hasLongest := settingNumber(settings[SettingMaxLength])
	if hasChosen && hasLongest && float64(len([]rune(chosen))) > longest {
		return disagree(SettingDefault)
	}
	return nil
}
