// SPDX-License-Identifier: Apache-2.0

package content

import (
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"testing"
)

// fixtureField is one field as the shared conditions fixture declares it.
type fixtureField struct {
	Key        string `json:"key"`
	Kind       string `json:"kind"`
	Multiple   bool   `json:"multiple"`
	Conditions []any  `json:"conditions"`
}

// conditionsFixture is the shared table both evaluators must pass.
type conditionsFixture struct {
	Operators []struct {
		Kind     string   `json:"kind"`
		Multiple bool     `json:"multiple"`
		Offers   []string `json:"offers"`
	} `json:"operators"`
	Compare []struct {
		Operator string `json:"operator"`
		Held     any    `json:"held"`
		Value    string `json:"value"`
		Holds    bool   `json:"holds"`
	} `json:"compare"`
	Hidden []struct {
		Name   string         `json:"name"`
		Fields []fixtureField `json:"fields"`
		Scope  map[string]any `json:"scope"`
		Hidden []string       `json:"hidden"`
	} `json:"hidden"`
}

// loadConditionsFixture reads the shared fixture from testdata.
func loadConditionsFixture(t *testing.T) conditionsFixture {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join("testdata", "conditions.json"))
	if err != nil {
		t.Fatalf("reading the fixture: %v", err)
	}
	var fixture conditionsFixture
	if err := json.Unmarshal(raw, &fixture); err != nil {
		t.Fatalf("decoding the fixture: %v", err)
	}
	return fixture
}

// fieldOf returns the field a fixture entry declares.
func declaredField(declared fixtureField) Field {
	settings := map[string]any{}
	if declared.Multiple {
		settings[SettingMultiple] = true
	}
	if declared.Conditions != nil {
		settings[SettingConditions] = declared.Conditions
	}
	return Field{Key: declared.Key, Kind: FieldKind(declared.Kind), Settings: settings}
}

func TestFixtureOperatorsPerKind(t *testing.T) {
	t.Parallel()

	for _, row := range loadConditionsFixture(t).Operators {
		offered := SourceOperators(FieldKind(row.Kind), row.Multiple)
		if !slices.Equal(offered, row.Offers) && (len(offered) > 0 || len(row.Offers) > 0) {
			t.Errorf("SourceOperators(%s, %v) = %v, want %v", row.Kind, row.Multiple, offered, row.Offers)
		}
	}
}

func TestFixtureCompareTable(t *testing.T) {
	t.Parallel()

	for _, row := range loadConditionsFixture(t).Compare {
		if got := compare(row.Operator, row.Held, row.Value); got != row.Holds {
			t.Errorf("compare(%q, %v, %q) = %v, want %v", row.Operator, row.Held, row.Value, got, row.Holds)
		}
	}
}

func TestFixtureHiddenScopes(t *testing.T) {
	t.Parallel()

	for _, scenario := range loadConditionsFixture(t).Hidden {
		fields := make([]Field, len(scenario.Fields))
		for i, declared := range scenario.Fields {
			fields[i] = declaredField(declared)
		}

		hidden := Hidden(fields, scenario.Scope)

		got := make([]string, 0, len(hidden))
		for key := range hidden {
			got = append(got, key)
		}
		slices.Sort(got)
		want := slices.Clone(scenario.Hidden)
		slices.Sort(want)
		if !slices.Equal(got, want) {
			t.Errorf("%s: Hidden() = %v, want %v", scenario.Name, got, want)
		}
	}
}
