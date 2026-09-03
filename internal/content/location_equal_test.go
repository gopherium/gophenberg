// SPDX-License-Identifier: Apache-2.0

package content_test

import (
	"testing"

	"github.com/gopherium/gophenberg/internal/content"
)

func TestRulesEqualJudgesShapeAndContent(t *testing.T) {
	t.Parallel()

	onPost := content.Rules{{{Source: "content_type", Operator: content.OperatorIs, Value: "post"}}}
	onPage := content.Rules{{{Source: "content_type", Operator: content.OperatorIs, Value: "page"}}}
	onBoth := content.Rules{{
		{Source: "content_type", Operator: content.OperatorIs, Value: "post"},
		{Source: "content_type", Operator: content.OperatorIs, Value: "page"},
	}}
	either := content.Rules{onPost[0], onPage[0]}

	for name, test := range map[string]struct {
		left, right content.Rules
		want        bool
	}{
		"the same rules":          {onPost, onPost, true},
		"an empty row dropped":    {onPost, content.Rules{onPost[0], {}}, true},
		"nothing against nothing": {nil, content.Rules{}, true},
		"a different value":       {onPost, onPage, false},
		"a longer row":            {onPost, onBoth, false},
		"more rows":               {onPost, either, false},
	} {
		if got := test.left.Equal(test.right); got != test.want {
			t.Errorf("%s: Equal() = %v, want %v", name, got, test.want)
		}
	}
}
