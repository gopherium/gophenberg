// SPDX-License-Identifier: Apache-2.0

package content

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"time"
)

// SettingConditions is the setting holding the rules a field is shown under.
const SettingConditions = "conditions"

// SettingListed is the setting opting a field into the admin list as a column.
const SettingListed = "listed"

// ErrFieldHidden reports a value submitted under a field its conditions hide.
var ErrFieldHidden = errors.New("content: field is hidden")

// ErrRuleCycle reports conditions that lead back to the field they show.
var ErrRuleCycle = errors.New("content: conditions loop")

// ErrFieldReferenced reports a field a sibling's conditions read.
var ErrFieldReferenced = errors.New("content: a sibling's conditions read the field")

// decimal is the shape a number rule value takes.
var decimal = regexp.MustCompile(`^-?(0|[1-9][0-9]*)(\.[0-9]+)?$`)

// ConditionsOf returns the rules a field is shown under, none when it carries none.
func ConditionsOf(f Field) Rules {
	groups, _ := f.Settings[SettingConditions].([]any)
	rules := make(Rules, 0, len(groups))
	for _, group := range groups {
		rows, _ := group.([]any)
		held := make([]Rule, 0, len(rows))
		for _, row := range rows {
			parts, _ := row.(map[string]any)
			var rule Rule
			rule.Source, _ = parts["source"].(string)
			rule.Operator, _ = parts["operator"].(string)
			rule.Value, _ = parts["value"].(string)
			held = append(held, rule)
		}
		rules = append(rules, held)
	}
	return rules.Normalize()
}

// ConditionsSetting returns the stored form of the rules, the value to hold under SettingConditions.
func ConditionsSetting(rules Rules) []any {
	groups := []any{}
	for _, group := range rules.Normalize() {
		rows := make([]any, 0, len(group))
		for _, rule := range group {
			rows = append(rows, map[string]any{
				"source": rule.Source, "operator": rule.Operator, "value": rule.Value,
			})
		}
		groups = append(groups, rows)
	}
	return groups
}

// Listed reports whether a field is opted into the admin list.
func Listed(f Field) bool {
	listed, _ := f.Settings[SettingListed].(bool)
	return listed
}

// SourceOperators returns the operators a field of the kind offers as a rule source, none for a kind no rule reads.
func SourceOperators(kind FieldKind, multiple bool) []string {
	switch kind {
	case FieldKindText:
		return []string{OperatorIs, OperatorIsNot, OperatorContains, OperatorEmpty, OperatorNotEmpty}
	case FieldKindNumber, FieldKindDate:
		return []string{OperatorIs, OperatorIsNot, OperatorLess, OperatorGreater, OperatorEmpty, OperatorNotEmpty}
	case FieldKindBoolean:
		return []string{OperatorIs, OperatorIsNot}
	case FieldKindChoice:
		if multiple {
			return []string{OperatorContains, OperatorEmpty, OperatorNotEmpty}
		}
		return []string{OperatorIs, OperatorIsNot, OperatorEmpty, OperatorNotEmpty}
	case FieldKindMedia:
		return []string{OperatorEmpty, OperatorNotEmpty}
	default:
		return nil
	}
}

// fieldParam is the param reading one sibling field as a rule source.
type fieldParam struct {
	field Field
}

// Name returns the sibling's key.
func (p fieldParam) Name() string { return p.field.Key }

// Operators returns the operators the sibling's kind offers.
func (p fieldParam) Operators() []string {
	return SourceOperators(p.field.Kind, multipleOf(p.field))
}

// Values returns no choices, the rule builder reads the sibling itself.
func (fieldParam) Values(context.Context) ([]Choice, error) { return nil, nil }

// Stands reports why a rule value does not read as the sibling's kind, if it does not.
func (p fieldParam) Stands(operator, value string) error {
	if !needsValue(operator) {
		return nil
	}
	if value == "" {
		return valueMissing(p.field.Key)
	}
	if !p.reads(value) {
		return Refuse(ErrRuleValue, "rule_value_shape",
			fmt.Sprintf("content: rule value %q does not read as %s on %s", value, p.field.Kind, p.field.Key),
			Details{"source": p.field.Key, "value": value})
	}
	return nil
}

// reads reports whether the value is one the sibling's kind can hold.
func (p fieldParam) reads(value string) bool {
	switch p.field.Kind {
	case FieldKindNumber:
		return decimal.MatchString(value)
	case FieldKindBoolean:
		return value == "true" || value == "false"
	case FieldKindDate:
		_, err := time.Parse(dateLayout, value)
		return err == nil
	case FieldKindChoice:
		return allowsCustom(p.field) || choiceValues(p.field.Settings[SettingChoices])[value]
	default:
		return true
	}
}

// Holds reports whether the sibling's value on the screen satisfies the operator and the value.
func (p fieldParam) Holds(scr Screen, operator, value string) bool {
	return compare(operator, scr[p.field.Key], value)
}

// multipleOf reports whether a choice field takes several values.
func multipleOf(f Field) bool {
	multiple, _ := f.Settings[SettingMultiple].(bool)
	return multiple
}

// allowsCustom reports whether a choice field takes values beyond its choices.
func allowsCustom(f Field) bool {
	custom, _ := f.Settings[SettingAllowCustom].(bool)
	return custom
}

// ScopeParams returns the registry of the sibling fields a condition may read.
func ScopeParams(fields []Field) *ParamRegistry {
	params := NewParamRegistry()
	for _, f := range fields {
		if SourceOperators(f.Kind, multipleOf(f)) == nil {
			continue
		}
		p := fieldParam{field: f}
		params.held[f.Key] = p
		params.order = append(params.order, p)
	}
	return params
}

// Hidden returns the keys of the fields their conditions hide on the scope, a hidden source reading as absent.
func Hidden(fields []Field, scope Values) map[string]bool {
	params := ScopeParams(fields)
	screen := screenOf(fields, scope)
	hidden := map[string]bool{}
	ordered, looped := dependencyOrder(fields)
	for _, f := range ordered {
		rules := ConditionsOf(f)
		if len(rules) == 0 || rules.Match(screen, params) {
			continue
		}
		hidden[f.Key] = true
		delete(screen, f.Key)
	}
	for _, key := range looped {
		hidden[key] = true
	}
	return hidden
}

// screenOf returns the scope as a screen where an absent boolean reads as false.
func screenOf(fields []Field, scope Values) Screen {
	screen := make(Screen, len(scope))
	for key, value := range scope {
		screen[key] = value
	}
	for _, f := range fields {
		if f.Kind == FieldKindBoolean && screen[f.Key] == nil {
			screen[f.Key] = false
		}
	}
	return screen
}

// dependencyOrder returns the fields with every source before its dependents, and the keys a loop leaves unordered.
func dependencyOrder(fields []Field) ([]Field, []string) {
	declared := make(map[string]bool, len(fields))
	for _, f := range fields {
		declared[f.Key] = true
	}
	placed := make(map[string]bool, len(fields))
	ordered := make([]Field, 0, len(fields))
	for progress := true; progress; {
		progress = false
		for _, f := range fields {
			if placed[f.Key] || !sourcesPlaced(f, declared, placed) {
				continue
			}
			placed[f.Key] = true
			ordered = append(ordered, f)
			progress = true
		}
	}
	var looped []string
	for _, f := range fields {
		if !placed[f.Key] {
			looped = append(looped, f.Key)
		}
	}
	return ordered, looped
}

// sourcesPlaced reports whether every declared source the field's conditions read is already ordered.
func sourcesPlaced(f Field, declared, placed map[string]bool) bool {
	for _, group := range ConditionsOf(f) {
		for _, rule := range group {
			if declared[rule.Source] && !placed[rule.Source] {
				return false
			}
		}
	}
	return true
}

// Shown returns the values with every key the conditions hide taken away, at any depth.
func Shown(fields []Field, values Values) Values {
	hidden := Hidden(fields, values)
	shown := make(Values, len(values))
	for key, value := range values {
		if !hidden[key] {
			shown[key] = value
		}
	}
	for _, f := range fields {
		if f.Kind.Holds() && shown[f.Key] != nil {
			shown[f.Key] = shownInside(f, shown[f.Key])
		}
	}
	return shown
}

// shownInside returns a container's value with every key its own rules hide taken away from each row.
func shownInside(f Field, value any) any {
	if rows, listed := value.([]any); listed {
		held := make([]any, len(rows))
		for i, row := range rows {
			held[i] = shownInside(f, row)
		}
		return held
	}
	inside, held := value.(map[string]any)
	if !held {
		return value
	}
	return map[string]any(Shown(f.Fields, inside))
}

// Concealed reports the first submitted value standing under a field the scope hides, at any depth.
func Concealed(fields []Field, scope, submitted Values) error {
	hidden := Hidden(fields, scope)
	for _, f := range fields {
		value := submitted[f.Key]
		if value == nil {
			continue
		}
		if hidden[f.Key] {
			return Refuse(ErrFieldHidden, "field_hidden",
				fmt.Sprintf("%s: %s", ErrFieldHidden, f.Key), Details{"field": f.Key})
		}
		if !f.Kind.Holds() {
			continue
		}
		if err := concealedInside(f, value); err != nil {
			return err
		}
	}
	return nil
}

// concealedInside reports the first value a container hides, each row read as its own scope.
func concealedInside(f Field, value any) error {
	if rows, listed := value.([]any); listed {
		for _, row := range rows {
			if err := concealedInside(f, row); err != nil {
				return err
			}
		}
		return nil
	}
	inside, _ := value.(map[string]any)
	return Concealed(f.Fields, inside, inside)
}

// Referenced returns the sibling whose conditions read the key, and whether one does.
func Referenced(fields []Field, key string) (string, bool) {
	for _, f := range fields {
		if f.Key == key {
			continue
		}
		for _, group := range ConditionsOf(f) {
			for _, rule := range group {
				if rule.Source == key {
					return f.Key, true
				}
			}
		}
	}
	return "", false
}

// Unreferenced reports whether a sibling's conditions read the key, refusing the removal when one does.
func Unreferenced(fields []Field, key string) error {
	by, found := Referenced(fields, key)
	if !found {
		return nil
	}
	return Refuse(ErrFieldReferenced, "field_referenced",
		fmt.Sprintf("%s: %s reads %s", ErrFieldReferenced, by, key), Details{"field": key, "by": by})
}

// Stands reports whether a field's conditions close no loop and read siblings of its scope.
func Stands(siblings []Field, f Field) error {
	scope := besides(siblings, f)
	if err := Acyclic(append(scope, f)); err != nil {
		return err
	}
	return ConditionsOf(f).Validate(ScopeParams(scope))
}

// besides returns the fields other than the one named, so a field never stands as its own sibling.
func besides(fields []Field, f Field) []Field {
	kept := make([]Field, 0, len(fields))
	for _, held := range fields {
		if held.Key != f.Key {
			kept = append(kept, held)
		}
	}
	return kept
}

// Acyclic reports the first field whose conditions lead back to itself, if one does.
func Acyclic(fields []Field) error {
	_, looped := dependencyOrder(fields)
	if len(looped) == 0 {
		return nil
	}
	return Refuse(ErrRuleCycle, "rule_cycle",
		fmt.Sprintf("%s: %s", ErrRuleCycle, looped[0]), Details{"field": looped[0]})
}
