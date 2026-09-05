// SPDX-License-Identifier: Apache-2.0

package content_test

import (
	"errors"
	"testing"

	"github.com/gopherium/gophenberg/internal/content"
)

// shownWhen returns the stored form of one rule showing a field when the source holds the value.
func shownWhen(source, operator, value string) []any {
	return []any{[]any{map[string]any{"source": source, "operator": operator, "value": value}}}
}

// sourceField returns a field of the kind carrying the given settings.
func sourceField(key string, kind content.FieldKind, settings map[string]any) content.Field {
	return content.Field{Key: key, Kind: kind, Settings: settings}
}

// conditioned returns a field shown when the source holds the value.
func conditioned(key string, kind content.FieldKind, source, operator, value string) content.Field {
	return sourceField(key, kind, map[string]any{content.SettingConditions: shownWhen(source, operator, value)})
}

// standing returns the error the scope's params answer for one rule on the source.
func standing(fields []content.Field, source, operator, value string) error {
	rules := content.Rules{{{Source: source, Operator: operator, Value: value}}}
	return rules.Validate(content.ScopeParams(fields))
}

func TestConditionsOfReadsTheStoredForm(t *testing.T) {
	t.Parallel()

	field := conditioned("sale_price", content.FieldKindNumber, "on_sale", content.OperatorIs, "true")

	rules := content.ConditionsOf(field)

	want := content.Rule{Source: "on_sale", Operator: content.OperatorIs, Value: "true"}
	if len(rules) != 1 || len(rules[0]) != 1 || rules[0][0] != want {
		t.Errorf("ConditionsOf() = %v, want %v", rules, want)
	}
}

func TestConditionsOfReadsNothingFromAFieldWithoutConditions(t *testing.T) {
	t.Parallel()

	for name, settings := range map[string]map[string]any{
		"no settings":     nil,
		"other settings":  {content.SettingInstructions: "Fill me in"},
		"malformed shape": {content.SettingConditions: "not a list"},
		"malformed rule":  {content.SettingConditions: []any{[]any{"not a rule"}}},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			rules := content.ConditionsOf(sourceField("note", content.FieldKindText, settings))

			if len(rules.Normalize()) != 0 {
				t.Errorf("ConditionsOf() = %v, want no rules", rules)
			}
		})
	}
}

func TestConditionsSettingRoundTrips(t *testing.T) {
	t.Parallel()

	rules := content.Rules{
		{{Source: "on_sale", Operator: content.OperatorIs, Value: "true"}},
		{{Source: "summary", Operator: content.OperatorNotEmpty, Value: ""}},
	}

	settings := map[string]any{content.SettingConditions: content.ConditionsSetting(rules)}
	back := content.ConditionsOf(sourceField("x", content.FieldKindText, settings))

	if !back.Equal(rules) {
		t.Errorf("ConditionsOf(ConditionsSetting()) = %v, want %v", back, rules)
	}
	if err := content.ValidateSettings(content.FieldKindText, settings); err != nil {
		t.Errorf("ValidateSettings() = %v, want the stored form accepted", err)
	}
}

func TestListedReadsTheFlag(t *testing.T) {
	t.Parallel()

	if !content.Listed(sourceField("x", content.FieldKindText, map[string]any{content.SettingListed: true})) {
		t.Error("Listed() = false, want the flag read")
	}
	if content.Listed(sourceField("x", content.FieldKindText, nil)) {
		t.Error("Listed() = true, want a field without the flag left out")
	}
}

func TestConditionsSettingIsAcceptedOnEveryKind(t *testing.T) {
	t.Parallel()

	stored := shownWhen("on_sale", content.OperatorIs, "true")
	for _, kind := range []content.FieldKind{
		content.FieldKindText, content.FieldKindNumber, content.FieldKindBoolean, content.FieldKindDate,
		content.FieldKindMedia, content.FieldKindRelation, content.FieldKindChoice,
		content.FieldKindSection, content.FieldKindRepeater,
	} {
		if err := content.ValidateSettings(kind, map[string]any{content.SettingConditions: stored}); err != nil {
			t.Errorf("ValidateSettings(%s) = %v, want conditions accepted", kind, err)
		}
	}
}

func TestConditionsSettingRefusesAMalformedShape(t *testing.T) {
	t.Parallel()

	for name, value := range map[string]any{
		"not a list":       "on_sale",
		"empty list":       []any{},
		"group not a list": []any{"on_sale"},
		"empty group":      []any{[]any{}},
		"rule not a map":   []any{[]any{"on_sale"}},
		"rule missing key": []any{[]any{map[string]any{"source": "on_sale", "operator": "=="}}},
		"rule extra key":   []any{[]any{map[string]any{"source": "a", "operator": "==", "value": "b", "more": 1}}},
		"rule not strings": []any{[]any{map[string]any{"source": "a", "operator": "==", "value": true}}},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			err := content.ValidateSettings(content.FieldKindText, map[string]any{content.SettingConditions: value})

			if !errors.Is(err, content.ErrSettingShape) {
				t.Errorf("ValidateSettings() = %v, want %v", err, content.ErrSettingShape)
			}
		})
	}
}

func TestListedSettingIsAcceptedOnListSafeKindsOnly(t *testing.T) {
	t.Parallel()

	for _, kind := range []content.FieldKind{
		content.FieldKindText, content.FieldKindNumber, content.FieldKindBoolean,
		content.FieldKindDate, content.FieldKindChoice,
	} {
		if err := content.ValidateSettings(kind, map[string]any{content.SettingListed: true}); err != nil {
			t.Errorf("ValidateSettings(%s) = %v, want listed accepted", kind, err)
		}
	}
	for _, kind := range []content.FieldKind{
		content.FieldKindMedia, content.FieldKindRelation, content.FieldKindSection, content.FieldKindRepeater,
	} {
		err := content.ValidateSettings(kind, map[string]any{content.SettingListed: true})
		if !errors.Is(err, content.ErrSettingUnknown) {
			t.Errorf("ValidateSettings(%s) = %v, want %v", kind, err, content.ErrSettingUnknown)
		}
	}
	worded := map[string]any{content.SettingListed: "yes"}
	if err := content.ValidateSettings(content.FieldKindText, worded); !errors.Is(err, content.ErrSettingShape) {
		t.Errorf("ValidateSettings(listed as a word) = %v, want %v", err, content.ErrSettingShape)
	}
}

func TestScopeParamsOffersSourceKindsOnly(t *testing.T) {
	t.Parallel()

	params := content.ScopeParams([]content.Field{
		sourceField("title", content.FieldKindText, nil),
		sourceField("author", content.FieldKindRelation, nil),
		sourceField("photos", content.FieldKindMedia, nil),
		sourceField("crew", content.FieldKindRepeater, nil),
	})

	listed := params.All()

	if len(listed) != 2 || listed[0].Name() != "title" || listed[1].Name() != "photos" {
		t.Errorf("All() = %v, want the text and media siblings alone in declared order", listed)
	}
	if choices, err := listed[0].Values(t.Context()); err != nil || choices != nil {
		t.Errorf("Values() = %v, %v, want no server side choices", choices, err)
	}
}

func TestFieldParamStandsBehindWellFormedValues(t *testing.T) {
	t.Parallel()

	choices := []any{map[string]any{"value": "percent", "label": "Percent"}}
	fields := []content.Field{
		sourceField("price", content.FieldKindNumber, nil),
		sourceField("on_sale", content.FieldKindBoolean, nil),
		sourceField("starts", content.FieldKindDate, nil),
		sourceField("kind", content.FieldKindChoice, map[string]any{content.SettingChoices: choices}),
		sourceField("open", content.FieldKindChoice, map[string]any{
			content.SettingChoices: choices, content.SettingAllowCustom: true,
		}),
		sourceField("title", content.FieldKindText, nil),
		sourceField("summary", content.FieldKindText, nil),
	}
	for name, rule := range map[string][3]string{
		"whole number":         {"price", content.OperatorIs, "10"},
		"negative decimal":     {"price", content.OperatorLess, "-3.5"},
		"decimal with zero":    {"price", content.OperatorGreater, "10.0"},
		"boolean true":         {"on_sale", content.OperatorIs, "true"},
		"boolean false":        {"on_sale", content.OperatorIsNot, "false"},
		"date":                 {"starts", content.OperatorLess, "2026-09-04"},
		"offered choice":       {"kind", content.OperatorIs, "percent"},
		"custom choice":        {"open", content.OperatorIs, "anything"},
		"any text":             {"title", content.OperatorContains, "  spaced  "},
		"valueless empty":      {"summary", content.OperatorEmpty, ""},
		"valueless not empty":  {"summary", content.OperatorNotEmpty, ""},
		"valueless with noise": {"summary", content.OperatorEmpty, "ignored"},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			if err := standing(fields, rule[0], rule[1], rule[2]); err != nil {
				t.Errorf("Validate() = %v, want the value accepted", err)
			}
		})
	}
}

func TestFieldParamRefusesAValueItCannotRead(t *testing.T) {
	t.Parallel()

	choices := []any{map[string]any{"value": "percent", "label": "Percent"}}
	fields := []content.Field{
		sourceField("price", content.FieldKindNumber, nil),
		sourceField("on_sale", content.FieldKindBoolean, nil),
		sourceField("starts", content.FieldKindDate, nil),
		sourceField("kind", content.FieldKindChoice, map[string]any{content.SettingChoices: choices}),
	}
	for name, rule := range map[string][3]string{
		"exponent":       {"price", content.OperatorIs, "1e3"},
		"infinity":       {"price", content.OperatorIs, "inf"},
		"padded":         {"price", content.OperatorIs, " 10"},
		"leading zero":   {"price", content.OperatorIs, "010"},
		"hex":            {"price", content.OperatorIs, "0x10"},
		"word":           {"price", content.OperatorIs, "ten"},
		"boolean word":   {"on_sale", content.OperatorIs, "yes"},
		"unpadded date":  {"starts", content.OperatorIs, "2026-9-4"},
		"unknown choice": {"kind", content.OperatorIs, "fixed"},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			err := standing(fields, rule[0], rule[1], rule[2])

			if !errors.Is(err, content.ErrRuleValue) {
				t.Fatalf("Validate() = %v, want %v", err, content.ErrRuleValue)
			}
			if code, _ := content.CodeOf(err); code != "rule_value_shape" {
				t.Errorf("code = %q, want rule_value_shape", code)
			}
		})
	}
}

func TestFieldParamRefusesAMissingValueUnderAValuedOperator(t *testing.T) {
	t.Parallel()

	fields := []content.Field{sourceField("title", content.FieldKindText, nil)}

	err := standing(fields, "title", content.OperatorIs, "")

	if !errors.Is(err, content.ErrRuleValue) {
		t.Fatalf("Validate() = %v, want %v", err, content.ErrRuleValue)
	}
	if code, _ := content.CodeOf(err); code != "rule_value_missing" {
		t.Errorf("code = %q, want rule_value_missing", code)
	}
}

func TestFieldParamRefusesAnOperatorTheKindDoesNotOffer(t *testing.T) {
	t.Parallel()

	fields := []content.Field{sourceField("on_sale", content.FieldKindBoolean, nil)}

	err := standing(fields, "on_sale", content.OperatorContains, "true")

	if !errors.Is(err, content.ErrRuleOperator) {
		t.Errorf("Validate() = %v, want %v", err, content.ErrRuleOperator)
	}
}

func TestConcealedRefusesAValueUnderAHiddenField(t *testing.T) {
	t.Parallel()

	fields := []content.Field{
		sourceField("on_sale", content.FieldKindBoolean, nil),
		conditioned("sale_price", content.FieldKindNumber, "on_sale", content.OperatorIs, "true"),
	}
	scope := content.Values{"on_sale": false, "sale_price": 20.0}

	err := content.Concealed(fields, scope, content.Values{"sale_price": 20.0})

	if !errors.Is(err, content.ErrFieldHidden) {
		t.Fatalf("Concealed() = %v, want %v", err, content.ErrFieldHidden)
	}
	if code, _ := content.CodeOf(err); code != "field_hidden" {
		t.Errorf("code = %q, want field_hidden", code)
	}
}

func TestConcealedLetsVisibleValuesAndClearsThrough(t *testing.T) {
	t.Parallel()

	fields := []content.Field{
		sourceField("on_sale", content.FieldKindBoolean, nil),
		conditioned("sale_price", content.FieldKindNumber, "on_sale", content.OperatorIs, "true"),
	}
	for name, tc := range map[string]struct {
		scope, submitted content.Values
	}{
		"shown field":         {content.Values{"on_sale": true}, content.Values{"sale_price": 20.0}},
		"shown field cleared": {content.Values{"on_sale": true}, content.Values{"sale_price": nil}},
		"hidden field absent": {content.Values{"on_sale": false, "sale_price": 20.0}, content.Values{"on_sale": false}},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			if err := content.Concealed(fields, tc.scope, tc.submitted); err != nil {
				t.Errorf("Concealed() = %v, want nothing refused", err)
			}
		})
	}
}

func TestConcealedRefusesAHiddenFieldARequestNamesAtAll(t *testing.T) {
	t.Parallel()

	fields := []content.Field{
		sourceField("on-sale", content.FieldKindBoolean, nil),
		conditioned("sale-price", content.FieldKindNumber, "on-sale", content.OperatorIs, "true"),
	}
	scope := content.Values{"on-sale": false}

	err := content.Concealed(fields, scope, content.Values{"sale-price": nil})

	if !errors.Is(err, content.ErrFieldHidden) {
		t.Errorf("Concealed() = %v, want a hidden field refused even when the request clears it", err)
	}
}

func TestConcealedLeavesWhatAContainerRowCarriesToTheRequest(t *testing.T) {
	t.Parallel()

	crew := content.Field{Key: "crew", Kind: content.FieldKindRepeater, Fields: []content.Field{
		sourceField("paid", content.FieldKindBoolean, nil),
		conditioned("fee", content.FieldKindNumber, "paid", content.OperatorIs, "true"),
	}}
	details := content.Field{Key: "details", Kind: content.FieldKindSection, Fields: crew.Fields}
	fields := []content.Field{crew, details}
	rows := []any{
		map[string]any{"paid": true, "fee": 100.0},
		map[string]any{"paid": false, "fee": 100.0},
	}

	if err := content.Concealed(fields, content.Values{"crew": rows}, content.Values{"crew": rows}); err != nil {
		t.Errorf("Concealed(hidden row value) = %v, want a row carried whole", err)
	}
	section := content.Values{"details": rows[1]}
	if err := content.Concealed(fields, section, section); err != nil {
		t.Errorf("Concealed(hidden section value) = %v, want a section carried whole", err)
	}
}

func TestConcealedRefusesAWholeHiddenContainer(t *testing.T) {
	t.Parallel()

	fields := []content.Field{
		sourceField("has_crew", content.FieldKindBoolean, nil),
		{Key: "crew", Kind: content.FieldKindRepeater, Fields: []content.Field{
			sourceField("name", content.FieldKindText, nil),
		}, Settings: map[string]any{content.SettingConditions: shownWhen("has_crew", content.OperatorIs, "true")}},
	}
	rows := []any{map[string]any{"name": "Maria Perez"}}

	err := content.Concealed(fields, content.Values{"has_crew": false, "crew": rows}, content.Values{"crew": rows})

	if !errors.Is(err, content.ErrFieldHidden) {
		t.Errorf("Concealed() = %v, want the container itself refused", err)
	}
}

func TestReferencedNamesTheSiblingReadingAKey(t *testing.T) {
	t.Parallel()

	fields := []content.Field{
		sourceField("on_sale", content.FieldKindBoolean, nil),
		conditioned("sale_price", content.FieldKindNumber, "on_sale", content.OperatorIs, "true"),
		conditioned("loop", content.FieldKindText, "loop", content.OperatorNotEmpty, ""),
	}

	if by, found := content.Referenced(fields, "on_sale"); !found || by != "sale_price" {
		t.Errorf("Referenced(on_sale) = %q, %v, want sale_price found", by, found)
	}
	if by, found := content.Referenced(fields, "sale_price"); found {
		t.Errorf("Referenced(sale_price) = %q, %v, want nothing found", by, found)
	}
	if by, found := content.Referenced(fields, "loop"); found {
		t.Errorf("Referenced(loop) = %q, %v, want a field's own reference left out", by, found)
	}
}

func TestAcyclicAcceptsAChainAndRefusesALoop(t *testing.T) {
	t.Parallel()

	chain := []content.Field{
		sourceField("has_discount", content.FieldKindBoolean, nil),
		conditioned("discount_type", content.FieldKindText, "has_discount", content.OperatorIs, "true"),
		conditioned("discount_amount", content.FieldKindNumber, "discount_type", content.OperatorIs, "percent"),
	}
	if err := content.Acyclic(chain); err != nil {
		t.Errorf("Acyclic(chain) = %v, want a chain accepted", err)
	}

	for name, fields := range map[string][]content.Field{
		"two fields": {
			conditioned("first", content.FieldKindText, "second", content.OperatorIs, "x"),
			conditioned("second", content.FieldKindText, "first", content.OperatorIs, "y"),
		},
		"self reference": {
			conditioned("first", content.FieldKindText, "first", content.OperatorNotEmpty, ""),
		},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			err := content.Acyclic(fields)

			if !errors.Is(err, content.ErrRuleCycle) {
				t.Fatalf("Acyclic() = %v, want %v", err, content.ErrRuleCycle)
			}
			code, _ := content.CodeOf(err)
			details, _ := content.DetailsOf(err)
			if code != "rule_cycle" || details["field"] != "first" {
				t.Errorf("code = %q, details = %v, want rule_cycle naming first", code, details)
			}
		})
	}
}
