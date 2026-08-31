// SPDX-License-Identifier: Apache-2.0

package content

import (
	"errors"
	"fmt"
	"math"
	"slices"
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

// SettingChoices names the value and label pairs a choice field offers.
const SettingChoices = "choices"

// SettingMultiple names whether a choice field holds several values.
const SettingMultiple = "multiple"

// SettingPresentation names the control a field is edited as.
const SettingPresentation = "presentation"

// SettingAllowNull names whether a choice control offers an explicit empty entry.
const SettingAllowNull = "allow_null"

// SettingAllowCustom names whether a choice field takes values outside its choices.
const SettingAllowCustom = "allow_custom"

// SettingVariant names the flavor a text field is edited and checked as.
const SettingVariant = "variant"

// settingChecks returns the settings the kind takes, each with its shape check.
func settingChecks(kind FieldKind) map[string]func(value any) bool {
	held := map[string]func(value any) bool{SettingInstructions: settingString}
	switch kind {
	case FieldKindText:
		held[SettingDefault] = settingString
		held[SettingPlaceholder] = settingString
		held[SettingMaxLength] = settingWhole
		held[SettingVariant] = settingOneOf("email", "url", "textarea")
	case FieldKindNumber:
		held[SettingDefault] = settingNumeric
		held[SettingPlaceholder] = settingString
		held[SettingMin] = settingNumeric
		held[SettingMax] = settingNumeric
		held[SettingStep] = settingPositive
		held[SettingPresentation] = settingOneOf("range")
	case FieldKindBoolean:
		held[SettingDefault] = settingBool
	case FieldKindRepeater:
		held[SettingMin] = settingWhole
		held[SettingMax] = settingWhole
	case FieldKindChoice:
		held[SettingDefault] = settingChoiceDefault
		held[SettingChoices] = settingChoicePairs
		held[SettingMultiple] = settingBool
		held[SettingPresentation] = settingOneOf("select", "checkbox", "radio", "buttons")
		held[SettingAllowNull] = settingBool
		held[SettingAllowCustom] = settingBool
	}
	return held
}

// settingOneOf returns a check accepting one of the named words.
func settingOneOf(words ...string) func(value any) bool {
	return func(value any) bool {
		held, ok := value.(string)
		return ok && slices.Contains(words, held)
	}
}

// settingChoicePairs reports whether the value is a list of value and label pairs.
func settingChoicePairs(value any) bool {
	pairs, ok := value.([]any)
	if !ok {
		return false
	}
	for _, pair := range pairs {
		if !choicePair(pair) {
			return false
		}
	}
	return true
}

// choicePair reports whether the value is one pair of a value and a label, both words.
func choicePair(value any) bool {
	pair, ok := value.(map[string]any)
	if !ok || len(pair) != 2 {
		return false
	}
	chosen, hasValue := pair["value"].(string)
	named, hasLabel := pair["label"].(string)
	return hasValue && hasLabel && chosen != "" && named != ""
}

// settingChoiceDefault reports whether the value is a word or a list of words.
func settingChoiceDefault(value any) bool {
	if _, ok := value.(string); ok {
		return true
	}
	members, ok := value.([]any)
	if !ok {
		return false
	}
	for _, member := range members {
		if _, ok := member.(string); !ok {
			return false
		}
	}
	return true
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
	case FieldKindChoice:
		return choiceSettingsAgree(settings)
	default:
		return nil
	}
}

// choiceSettingsAgree reports whether the default matches multiple and sits among the choices.
func choiceSettingsAgree(settings map[string]any) error {
	chosen, hasChosen := settings[SettingDefault]
	if !hasChosen {
		return nil
	}
	members, isMany := chosen.([]any)
	if !isMany {
		members = []any{chosen}
	}
	if multiple, _ := settings[SettingMultiple].(bool); multiple != isMany {
		return disagree(SettingDefault)
	}
	if custom, _ := settings[SettingAllowCustom].(bool); custom {
		return nil
	}
	held := choiceValues(settings[SettingChoices])
	if len(held) == 0 {
		return nil
	}
	for _, member := range members {
		if !held[member.(string)] {
			return disagree(SettingDefault)
		}
	}
	return nil
}

// choiceValues returns the values a choices setting lists.
func choiceValues(listed any) map[string]bool {
	held := map[string]bool{}
	pairs, _ := listed.([]any)
	for _, pair := range pairs {
		if named, ok := pair.(map[string]any); ok {
			if chosen, ok := named["value"].(string); ok {
				held[chosen] = true
			}
		}
	}
	return held
}

// disagree returns the error naming the setting that falls outside the others.
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

// textSettingsAgree reports whether the default fits inside maxlength and reads as the variant.
func textSettingsAgree(settings map[string]any) error {
	chosen, hasChosen := settings[SettingDefault].(string)
	if !hasChosen {
		return nil
	}
	longest, hasLongest := settingNumber(settings[SettingMaxLength])
	if hasLongest && float64(len([]rune(chosen))) > longest {
		return disagree(SettingDefault)
	}
	variant, _ := settings[SettingVariant].(string)
	if !readsAsVariant(variant, chosen) {
		return disagree(SettingDefault)
	}
	return nil
}
