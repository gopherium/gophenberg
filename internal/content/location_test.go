// SPDX-License-Identifier: Apache-2.0

package content_test

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/gopherium/gophenberg/internal/content"
)

// errValueRefused is what a stub param answers for a value it will not stand behind.
var errValueRefused = errors.New("the value does not stand")

// stubParam is a param comparing a screen entry named after itself with the rule value.
type stubParam struct {
	name    string
	refuses error
}

// Name returns the param's source string.
func (p stubParam) Name() string { return p.name }

// Operators returns the two comparison operators.
func (p stubParam) Operators() []string {
	return []string{content.OperatorIs, content.OperatorIsNot}
}

// Values returns no choices.
func (p stubParam) Values(context.Context) ([]content.Choice, error) { return nil, nil }

// Stands answers the refusal the stub was built with, if any.
func (p stubParam) Stands(string, string) error { return p.refuses }

// Holds reports whether the screen entry named after the param equals the value, inverted under is not.
func (p stubParam) Holds(scr content.Screen, operator, value string) bool {
	matched := scr[p.name] == value
	if operator == content.OperatorIsNot {
		return !matched
	}
	return matched
}

// holdingParam is a param that holds on every screen whatever the operator says.
type holdingParam struct{}

// Name returns the param's source string.
func (holdingParam) Name() string { return "always" }

// Operators returns the two comparison operators.
func (holdingParam) Operators() []string {
	return []string{content.OperatorIs, content.OperatorIsNot}
}

// Values returns no choices.
func (holdingParam) Values(context.Context) ([]content.Choice, error) { return nil, nil }

// Stands accepts every value.
func (holdingParam) Stands(string, string) error { return nil }

// Holds answers true on every screen.
func (holdingParam) Holds(content.Screen, string, string) bool { return true }

// registryOf returns a registry holding the given params.
func registryOf(t *testing.T, params ...content.Param) *content.ParamRegistry {
	t.Helper()

	registry := content.NewParamRegistry()
	for _, p := range params {
		if err := registry.Register(p); err != nil {
			t.Fatalf("Register(%s) error = %v, want nil", p.Name(), err)
		}
	}
	return registry
}

// rule returns one rule row.
func rule(source, operator, value string) content.Rule {
	return content.Rule{Source: source, Operator: operator, Value: value}
}

func TestRulesMatchOneRule(t *testing.T) {
	t.Parallel()

	params := registryOf(t, stubParam{name: "section"})
	rules := content.Rules{{rule("section", content.OperatorIs, "news")}}

	if !rules.Match(content.Screen{"section": "news"}, params) {
		t.Error("Match() = false, want the screen holding the value matched")
	}
	if rules.Match(content.Screen{"section": "sports"}, params) {
		t.Error("Match() = true, want a screen holding another value unmatched")
	}
}

func TestRulesMatchInvertsIsNot(t *testing.T) {
	t.Parallel()

	params := registryOf(t, stubParam{name: "section"})
	rules := content.Rules{{rule("section", content.OperatorIsNot, "news")}}

	if rules.Match(content.Screen{"section": "news"}, params) {
		t.Error("Match() = true, want the named value excluded")
	}
	if !rules.Match(content.Screen{"section": "sports"}, params) {
		t.Error("Match() = false, want every other value matched")
	}
}

func TestRulesMatchLeavesTheOperatorToTheParam(t *testing.T) {
	t.Parallel()

	params := registryOf(t, holdingParam{})
	rules := content.Rules{{rule("always", content.OperatorIsNot, "anything")}}

	if !rules.Match(content.Screen{}, params) {
		t.Error("Match() = false, want the param's answer taken as it is under is not")
	}
}

func TestRulesMatchNeedsEveryRuleOfAGroup(t *testing.T) {
	t.Parallel()

	params := registryOf(t, stubParam{name: "section"}, stubParam{name: "shape"})
	rules := content.Rules{{
		rule("section", content.OperatorIs, "news"),
		rule("shape", content.OperatorIs, "wide"),
	}}

	if !rules.Match(content.Screen{"section": "news", "shape": "wide"}, params) {
		t.Error("Match() = false, want a screen satisfying both rules matched")
	}
	if rules.Match(content.Screen{"section": "news", "shape": "tall"}, params) {
		t.Error("Match() = true, want one failing rule to fail its whole group")
	}
}

func TestRulesMatchTakesAnyGroup(t *testing.T) {
	t.Parallel()

	params := registryOf(t, stubParam{name: "section"})
	rules := content.Rules{
		{rule("section", content.OperatorIs, "news")},
		{rule("section", content.OperatorIs, "sports")},
	}

	if !rules.Match(content.Screen{"section": "sports"}, params) {
		t.Error("Match() = false, want a screen satisfying the second group matched")
	}
}

func TestEmptyRulesMatchNowhere(t *testing.T) {
	t.Parallel()

	params := registryOf(t, stubParam{name: "section"})

	if (content.Rules{}).Match(content.Screen{"section": "news"}, params) {
		t.Error("Match() = true, want no rules to match nowhere")
	}
	if (content.Rules)(nil).Match(content.Screen{"section": "news"}, params) {
		t.Error("Match() = true, want nil rules to match nowhere")
	}
}

func TestAGroupWithNoRulesMatchesNothing(t *testing.T) {
	t.Parallel()

	params := registryOf(t, stubParam{name: "section"})
	rules := content.Rules{{}}

	if rules.Match(content.Screen{"section": "news"}, params) {
		t.Error("Match() = true, want an empty group to match nothing rather than everything")
	}
}

func TestAnUnknownSourceFailsItsGroup(t *testing.T) {
	t.Parallel()

	params := registryOf(t, stubParam{name: "section"})
	rules := content.Rules{{
		rule("vanished", content.OperatorIs, "anything"),
		rule("section", content.OperatorIs, "news"),
	}}

	if rules.Match(content.Screen{"section": "news"}, params) {
		t.Error("Match() = true, want a rule naming an unregistered source to fail closed")
	}
}

func TestNormalizeStripsEmptyRulesAndGroups(t *testing.T) {
	t.Parallel()

	rules := content.Rules{
		{content.Rule{}, rule("section", content.OperatorIs, "news")},
		{},
		nil,
	}

	normalized := rules.Normalize()

	want := content.Rules{{rule("section", content.OperatorIs, "news")}}
	if len(normalized) != 1 || len(normalized[0]) != 1 || normalized[0][0] != want[0][0] {
		t.Errorf("Normalize() = %v, want %v", normalized, want)
	}
	if empty := (content.Rules)(nil).Normalize(); empty == nil || len(empty) != 0 {
		t.Errorf("Normalize() of nil = %v, want an empty non nil rules value", empty)
	}
}

func TestNormalizeBlanksTheValueOfAValuelessOperator(t *testing.T) {
	t.Parallel()

	rules := content.Rules{
		{rule("summary", content.OperatorEmpty, "junk")},
		{rule("summary", content.OperatorNotEmpty, "junk")},
	}

	normalized := rules.Normalize()

	for i, group := range normalized {
		if group[0].Value != "" {
			t.Errorf("Normalize()[%d] = %v, want the value blanked", i, group[0])
		}
	}
	blanked := content.Rules{
		{rule("summary", content.OperatorEmpty, "")},
		{rule("summary", content.OperatorNotEmpty, "")},
	}
	if !rules.Equal(blanked) {
		t.Error("Equal() = false, want a valueless rule equal whatever value it carried")
	}
}

func TestValidateAcceptsWellFormedRules(t *testing.T) {
	t.Parallel()

	params := registryOf(t, stubParam{name: "section"})
	rules := content.Rules{
		{rule("section", content.OperatorIs, "news"), rule("section", content.OperatorIsNot, "sports")},
	}

	if err := rules.Validate(params); err != nil {
		t.Errorf("Validate() = %v, want well formed rules accepted", err)
	}
}

func TestValidateRefusesAMalformedRule(t *testing.T) {
	t.Parallel()

	params := registryOf(t, stubParam{name: "section"})
	for name, tc := range map[string]struct {
		rules content.Rules
		want  error
		code  string
	}{
		"unknown source": {
			rules: content.Rules{{rule("vanished", content.OperatorIs, "news")}},
			want:  content.ErrRuleSourceUnknown,
			code:  "rule_source_unknown",
		},
		"operator the param does not offer": {
			rules: content.Rules{{rule("section", "~", "news")}},
			want:  content.ErrRuleOperator,
			code:  "rule_operator",
		},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			err := tc.rules.Validate(params)

			if !errors.Is(err, tc.want) {
				t.Fatalf("Validate() = %v, want %v", err, tc.want)
			}
			if code, _ := content.CodeOf(err); code != tc.code {
				t.Errorf("code = %q, want %q", code, tc.code)
			}
		})
	}
}

func TestValidateAsksTheParamWhetherTheValueStands(t *testing.T) {
	t.Parallel()

	params := registryOf(t, stubParam{name: "section", refuses: errValueRefused})
	rules := content.Rules{{rule("section", content.OperatorIs, "news")}}

	if err := rules.Validate(params); !errors.Is(err, errValueRefused) {
		t.Errorf("Validate() = %v, want the param's refusal of the value passed on", err)
	}
}

func TestParamRegistryRefusesADuplicateName(t *testing.T) {
	t.Parallel()

	registry := registryOf(t, stubParam{name: "section"})

	err := registry.Register(stubParam{name: "section"})

	if !errors.Is(err, content.ErrParamTaken) {
		t.Fatalf("Register() = %v, want %v", err, content.ErrParamTaken)
	}
	if code, _ := content.CodeOf(err); code != "param_taken" {
		t.Errorf("code = %q, want param_taken", code)
	}
}

func TestParamRegistryListsInRegistrationOrder(t *testing.T) {
	t.Parallel()

	registry := registryOf(t, stubParam{name: "section"}, stubParam{name: "shape"})

	listed := registry.All()

	if len(listed) != 2 || listed[0].Name() != "section" || listed[1].Name() != "shape" {
		t.Errorf("All() = %v, want the params in registration order", listed)
	}
}

func TestContentTypeParamMatchesTheScreenType(t *testing.T) {
	t.Parallel()

	param := content.NewContentTypeParam(nil)
	params := registryOf(t, param)
	rules := content.Rules{{rule("content_type", content.OperatorIs, "post")}}

	if !rules.Match(content.Screen{"content_type": "post"}, params) {
		t.Error("Match() = false, want the screen's content type matched")
	}
	if rules.Match(content.Screen{"content_type": "page"}, params) {
		t.Error("Match() = true, want another content type unmatched")
	}
}

func TestContentTypeParamExcludesTheNamedTypeUnderIsNot(t *testing.T) {
	t.Parallel()

	params := registryOf(t, content.NewContentTypeParam(nil))
	rules := content.Rules{{rule("content_type", content.OperatorIsNot, "post")}}

	if rules.Match(content.Screen{"content_type": "post"}, params) {
		t.Error("Match() = true, want the named content type excluded")
	}
	if !rules.Match(content.Screen{"content_type": "page"}, params) {
		t.Error("Match() = false, want every other content type matched")
	}
}

func TestContentTypeParamRefusesAnEmptyValue(t *testing.T) {
	t.Parallel()

	params := registryOf(t, content.NewContentTypeParam(nil))
	rules := content.Rules{{rule("content_type", content.OperatorIs, "")}}

	err := rules.Validate(params)

	if !errors.Is(err, content.ErrRuleValue) {
		t.Fatalf("Validate() = %v, want %v", err, content.ErrRuleValue)
	}
	if code, _ := content.CodeOf(err); code != "rule_value_missing" {
		t.Errorf("code = %q, want rule_value_missing", code)
	}
}

func TestTheAnyValueMatchesEveryContentType(t *testing.T) {
	t.Parallel()

	params := registryOf(t, content.NewContentTypeParam(nil))
	rules := content.Rules{{rule("content_type", content.OperatorIs, content.AnyContentType)}}

	for _, typeKey := range []string{"post", "page", "recipe"} {
		if !rules.Match(content.Screen{"content_type": typeKey}, params) {
			t.Errorf("Match() = false for %s, want the any value to match every type", typeKey)
		}
	}
}

func TestValidateRefusesTheAnyValueNegated(t *testing.T) {
	t.Parallel()

	params := registryOf(t, content.NewContentTypeParam(nil))
	rules := content.Rules{{rule("content_type", content.OperatorIsNot, content.AnyContentType)}}

	err := rules.Validate(params)

	if !errors.Is(err, content.ErrRuleAnyNegated) {
		t.Fatalf("Validate() = %v, want %v", err, content.ErrRuleAnyNegated)
	}
	if code, _ := content.CodeOf(err); code != "rule_any_negated" {
		t.Errorf("code = %q, want rule_any_negated", code)
	}
}

func TestTheDefaultRegistryHoldsTheContentTypeParam(t *testing.T) {
	t.Parallel()

	params := content.DefaultParamRegistry(nil)

	listed := params.All()
	if len(listed) != 1 || listed[0].Name() != content.ScreenContentType {
		t.Fatalf("All() = %v, want the content type param alone", listed)
	}
	rules := content.Rules{{rule(content.ScreenContentType, content.OperatorIs, "post")}}
	if err := rules.Validate(params); err != nil {
		t.Errorf("Validate() = %v, want the default registry validating a content type rule", err)
	}
}

func TestContentTypeParamServesItsChoices(t *testing.T) {
	t.Parallel()

	offered := []content.Choice{{Value: "post", Label: "Posts"}}
	param := content.NewContentTypeParam(func(context.Context) ([]content.Choice, error) {
		return offered, nil
	})

	choices, err := param.Values(t.Context())

	if err != nil {
		t.Fatalf("Values() error = %v, want nil", err)
	}
	if len(choices) != 1 || choices[0] != offered[0] {
		t.Errorf("Values() = %v, want the choices the source offers", choices)
	}
	if choiceless, err := content.NewContentTypeParam(nil).Values(t.Context()); err != nil || choiceless != nil {
		t.Errorf("Values() without a source = %v, %v, want none and no failure", choiceless, err)
	}
}

func TestRulesJSONCarriesTheWireShape(t *testing.T) {
	t.Parallel()

	rules := content.Rules{{rule("content_type", content.OperatorIs, "post")}}

	raw, err := json.Marshal(rules)

	if err != nil {
		t.Fatalf("Marshal() error = %v, want nil", err)
	}
	want := `[[{"source":"content_type","operator":"==","value":"post"}]]`
	if string(raw) != want {
		t.Errorf("Marshal() = %s, want %s", raw, want)
	}
	var back content.Rules
	if err := json.Unmarshal(raw, &back); err != nil {
		t.Fatalf("Unmarshal() error = %v, want nil", err)
	}
	if len(back) != 1 || len(back[0]) != 1 || back[0][0] != rules[0][0] {
		t.Errorf("Unmarshal() = %v, want the same rules back", back)
	}
}
