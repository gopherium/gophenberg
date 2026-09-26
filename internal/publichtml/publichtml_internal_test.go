// SPDX-License-Identifier: Apache-2.0

package publichtml

import (
	"strings"
	"testing"

	"golang.org/x/net/html"
)

// seenByABrowser returns markup with every delimiter a browser reads as a comment taken out.
func seenByABrowser(markup string) string {
	var kept strings.Builder
	tokens := html.NewTokenizer(strings.NewReader(markup))
	for kind := tokens.Next(); kind != html.ErrorToken; kind = tokens.Next() {
		raw := string(tokens.Raw())
		if kind == html.CommentToken && blockDelimiter.FindString(raw) == raw {
			continue
		}
		kept.WriteString(raw)
	}
	return kept.String()
}

func FuzzSanitizeServesNothingThePolicyWouldRemove(f *testing.F) {
	for _, seed := range []string{
		"<!-- wp:paragraph --><p>a</p><!-- /wp:paragraph -->",
		`<!-- wp:group {"layout":{"type":"constrained"}} --><div class="wp-block-group"><p>x</p></div><!-- /wp:group -->`,
		`<!-- wp:spacer {"height":"40px"} /-->`,
		`<p title="<!-- wp:z {" onmouseover=alert(1) x="} -->">hi</p>`,
		`<!-- wp:paragraph --><p>a</p><script><!-- wp:x --></script><!-- /wp:paragraph -->`,
		`<!-- wp:image --><img src="x.png" alt="<!-- wp:z " -->"><!-- /wp:image -->`,
		"<<!-- wp:x -->",
		"<textarea><!-- wp:x --></textarea><!-- wp:y -->",
		"<svg><!-- wp:x --></svg>",
	} {
		f.Add(seed)
	}
	f.Fuzz(func(t *testing.T, content string) {
		served := seenByABrowser(Sanitize(content))
		if again := policy.Sanitize(served); again != served {
			t.Errorf("Sanitize(%q) serves markup the policy removes:\n got %q\nthen %q", content, served, again)
		}
	})
}
