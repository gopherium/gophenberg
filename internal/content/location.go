// SPDX-License-Identifier: Apache-2.0

package content

import (
	"context"
	"errors"
	"fmt"
)

// ErrRuleSourceUnknown reports a location rule naming an unregistered source.
var ErrRuleSourceUnknown = errors.New("content: rule source unknown")

// ErrRuleOperator reports a location rule using an operator its source does not offer.
var ErrRuleOperator = errors.New("content: rule operator not offered")

// ErrRuleValue reports a location rule holding no value.
var ErrRuleValue = errors.New("content: rule value missing")

// ErrRuleAnyNegated reports the any value beside the is not operator.
var ErrRuleAnyNegated = errors.New("content: the any value takes only the is operator")

// ErrParamTaken reports a second param registering under a taken name.
var ErrParamTaken = errors.New("content: param name taken")

// OperatorIs is the operator matching a rule value exactly.
const OperatorIs = "=="

// OperatorIsNot is the operator excluding a rule value.
const OperatorIsNot = "!="

// AnyContentType is the rule value matching every content type.
const AnyContentType = "*"

// ScreenContentType is the screen entry naming the content type fields would appear on.
const ScreenContentType = "content_type"

// Rule is one location condition a field group states.
type Rule struct {
	Source   string `json:"source"`
	Operator string `json:"operator"`
	Value    string `json:"value"`
}

// Rules is the location of a field group, OR groups of AND rules.
type Rules [][]Rule

// Screen carries what is known about the place fields would appear.
type Screen map[string]any

// Choice is one value a rule builder offers for a source.
type Choice struct {
	Value string
	Label string
}

// Param is one rule source the location engine validates and matches against.
type Param interface {
	// Name returns the source string rules reference the param by.
	Name() string
	// Operators returns the operators the param accepts.
	Operators() []string
	// Values returns the choices a rule builder offers for the param.
	Values(ctx context.Context) ([]Choice, error)
	// Matches reports whether the screen holds the value, before any operator inversion.
	Matches(scr Screen, value string) bool
}

// ParamRegistry holds the rule sources locations validate and match against.
type ParamRegistry struct {
	held  map[string]Param
	order []Param
}

// NewParamRegistry returns a registry holding no params.
func NewParamRegistry() *ParamRegistry {
	return &ParamRegistry{held: map[string]Param{}}
}

// DefaultParamRegistry returns a registry holding the built in content type param.
func DefaultParamRegistry(choices TypeChoices) *ParamRegistry {
	param := NewContentTypeParam(choices)
	return &ParamRegistry{held: map[string]Param{param.Name(): param}, order: []Param{param}}
}

// Register adds a param, refusing a name already taken.
func (r *ParamRegistry) Register(p Param) error {
	if _, taken := r.held[p.Name()]; taken {
		return Refuse(ErrParamTaken, "param_taken",
			fmt.Sprintf("%s: %s", ErrParamTaken, p.Name()), Details{"param": p.Name()})
	}
	r.held[p.Name()] = p
	r.order = append(r.order, p)
	return nil
}

// Param returns the registered param of a name, and whether one holds it.
func (r *ParamRegistry) Param(name string) (Param, bool) {
	p, found := r.held[name]
	return p, found
}

// All returns every registered param in registration order.
func (r *ParamRegistry) All() []Param {
	return append([]Param(nil), r.order...)
}

// Match reports whether any rule group fully matches the screen.
func (r Rules) Match(scr Screen, params *ParamRegistry) bool {
	for _, group := range r {
		if len(group) > 0 && groupMatches(group, scr, params) {
			return true
		}
	}
	return false
}

// groupMatches reports whether every rule of one group holds on the screen.
func groupMatches(group []Rule, scr Screen, params *ParamRegistry) bool {
	for _, rule := range group {
		p, found := params.Param(rule.Source)
		if !found {
			return false
		}
		matched := p.Matches(scr, rule.Value)
		if rule.Operator == OperatorIsNot {
			matched = !matched
		}
		if !matched {
			return false
		}
	}
	return true
}

// Validate reports the first rule that is not well formed against the registered params.
func (r Rules) Validate(params *ParamRegistry) error {
	for _, group := range r {
		for _, rule := range group {
			if err := validateRule(rule, params); err != nil {
				return err
			}
		}
	}
	return nil
}

// validateRule reports why one rule is not well formed, if it is not.
func validateRule(rule Rule, params *ParamRegistry) error {
	p, found := params.Param(rule.Source)
	if !found {
		return Refuse(ErrRuleSourceUnknown, "rule_source_unknown",
			fmt.Sprintf("%s: %s", ErrRuleSourceUnknown, rule.Source), Details{"source": rule.Source})
	}
	if !offered(p.Operators(), rule.Operator) {
		return Refuse(ErrRuleOperator, "rule_operator",
			fmt.Sprintf("%s: %s", ErrRuleOperator, rule.Operator),
			Details{"source": rule.Source, "operator": rule.Operator})
	}
	if rule.Value == "" {
		return Refuse(ErrRuleValue, "rule_value_missing",
			fmt.Sprintf("%s on %s", ErrRuleValue, rule.Source), Details{"source": rule.Source})
	}
	if rule.Value == AnyContentType && rule.Operator == OperatorIsNot {
		return Refuse(ErrRuleAnyNegated, "rule_any_negated",
			ErrRuleAnyNegated.Error(), Details{"source": rule.Source})
	}
	return nil
}

// offered reports whether an operator is among the offered ones.
func offered(operators []string, operator string) bool {
	for _, held := range operators {
		if held == operator {
			return true
		}
	}
	return false
}

// Equal reports whether the rules match once both are normalized.
func (r Rules) Equal(o Rules) bool {
	mine, theirs := r.Normalize(), o.Normalize()
	if len(mine) != len(theirs) {
		return false
	}
	for i := range mine {
		if len(mine[i]) != len(theirs[i]) {
			return false
		}
		for j := range mine[i] {
			if mine[i][j] != theirs[i][j] {
				return false
			}
		}
	}
	return true
}

// Normalize returns the rules without zero rules and empty groups, never nil.
func (r Rules) Normalize() Rules {
	normalized := Rules{}
	for _, group := range r {
		kept := make([]Rule, 0, len(group))
		for _, rule := range group {
			if rule != (Rule{}) {
				kept = append(kept, rule)
			}
		}
		if len(kept) > 0 {
			normalized = append(normalized, kept)
		}
	}
	return normalized
}

// TypeChoices lists the type choices the content type param offers.
type TypeChoices func(ctx context.Context) ([]Choice, error)

// contentTypeParam is the built in param matching the content type of a screen.
type contentTypeParam struct {
	choices TypeChoices
}

// NewContentTypeParam returns the built in param matching the content type of a screen.
func NewContentTypeParam(choices TypeChoices) Param {
	return contentTypeParam{choices: choices}
}

// Name returns the source string content type rules carry.
func (contentTypeParam) Name() string { return ScreenContentType }

// Operators returns the two comparison operators.
func (contentTypeParam) Operators() []string { return []string{OperatorIs, OperatorIsNot} }

// Values returns the type choices the param's source offers.
func (p contentTypeParam) Values(ctx context.Context) ([]Choice, error) {
	if p.choices == nil {
		return nil, nil
	}
	return p.choices(ctx)
}

// Matches reports whether the screen's content type is the value, with any matching every type.
func (contentTypeParam) Matches(scr Screen, value string) bool {
	if value == AnyContentType {
		return true
	}
	return scr[ScreenContentType] == value
}
