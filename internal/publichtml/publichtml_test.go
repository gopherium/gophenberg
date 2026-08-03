// SPDX-License-Identifier: Apache-2.0

package publichtml_test

import (
	"strings"
	"testing"

	"github.com/gopherium/gophenberg/internal/publichtml"
)

func TestSanitizeKeepsStaticBlockMarkupByteStable(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name    string
		content string
	}{
		{
			name:    "paragraph",
			content: "<!-- wp:paragraph --><p>Maria Perez wrote this.</p><!-- /wp:paragraph -->",
		},
		{
			name:    "heading",
			content: `<!-- wp:heading --><h2 class="wp-block-heading">A Heading</h2><!-- /wp:heading -->`,
		},
		{
			name: "quote",
			content: `<!-- wp:quote --><blockquote class="wp-block-quote">` +
				`<p>Quoted words.</p><cite>A Source</cite></blockquote><!-- /wp:quote -->`,
		},
		{
			name: "list",
			content: `<!-- wp:list --><ul class="wp-block-list"><!-- wp:list-item --><li>One</li>` +
				`<!-- /wp:list-item --></ul><!-- /wp:list -->`,
		},
		{
			name: "columns",
			content: `<!-- wp:columns --><div class="wp-block-columns"><!-- wp:column -->` +
				`<div class="wp-block-column"><p>Left</p></div><!-- /wp:column --></div><!-- /wp:columns -->`,
		},
		{
			name: "group with attributes",
			content: `<!-- wp:group {"layout":{"type":"constrained"}} -->` +
				`<div class="wp-block-group"><p>Grouped</p></div><!-- /wp:group -->`,
		},
		{
			name: "image",
			content: `<!-- wp:image {"sizeSlug":"large"} --><figure class="wp-block-image size-large">` +
				`<img src="https://example.com/photo.jpg" alt="A photo"/>` +
				`<figcaption class="wp-element-caption">A caption</figcaption></figure><!-- /wp:image -->`,
		},
		{
			name: "styled paragraph",
			content: `<!-- wp:paragraph {"style":{"color":{"text":"#cf2e2e"}}} -->` +
				`<p class="has-text-color" style="color:#cf2e2e">Colored</p><!-- /wp:paragraph -->`,
		},
		{
			name:    "link",
			content: `<!-- wp:paragraph --><p><a href="https://example.com/page">A link</a></p><!-- /wp:paragraph -->`,
		},
		{
			name: "attributes carrying the escapes the editor writes",
			content: `<!-- wp:heading {"citation":"a \u003e b \u0026 c \u002d\u002d d"} -->` +
				`<h2 class="wp-block-heading">Title</h2><!-- /wp:heading -->`,
		},
		{
			name: "details block",
			content: `<!-- wp:details --><details class="wp-block-details" open="">` +
				`<summary>More</summary><p>Hidden</p></details><!-- /wp:details -->`,
		},
		{
			name:    "void block",
			content: `<!-- wp:spacer {"height":"40px"} --><div style="height:40px" aria-hidden="true"></div><!-- /wp:spacer -->`,
		},
		{
			name:    "namespaced block",
			content: `<!-- wp:my-plugin/my-block --><p>Third party</p><!-- /wp:my-plugin/my-block -->`,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			got := publichtml.Sanitize(tc.content)

			if got != tc.content {
				t.Errorf("Sanitize() = %q, want it byte stable at %q", got, tc.content)
			}
		})
	}
}

func TestSanitizeStripsScriptableMarkup(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name    string
		content string
		gone    string
	}{
		{
			name:    "script element",
			content: `<!-- wp:paragraph --><p>Hi</p><script>alert(1)</script><!-- /wp:paragraph -->`,
			gone:    "alert(1)",
		},
		{
			name:    "event handler",
			content: `<!-- wp:paragraph --><p onclick="steal()">Hi</p><!-- /wp:paragraph -->`,
			gone:    "onclick",
		},
		{
			name:    "javascript href",
			content: `<!-- wp:paragraph --><p><a href="javascript:steal()">Hi</a></p><!-- /wp:paragraph -->`,
			gone:    "javascript:",
		},
		{
			name:    "iframe",
			content: `<!-- wp:paragraph --><iframe src="https://example.com/evil"></iframe><!-- /wp:paragraph -->`,
			gone:    "<iframe",
		},
		{
			name: "form",
			content: `<!-- wp:paragraph --><form action="https://example.com/steal">` +
				`<input name="pw"/></form><!-- /wp:paragraph -->`,
			gone: "<form",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			got := publichtml.Sanitize(tc.content)

			if strings.Contains(got, tc.gone) {
				t.Errorf("Sanitize() = %q, want %q stripped", got, tc.gone)
			}
			if !strings.Contains(got, "<!-- wp:paragraph -->") {
				t.Errorf("Sanitize() = %q, want the block delimiters kept", got)
			}
		})
	}
}

func TestSanitizeRefusesForgedDelimiters(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name    string
		content string
		gone    string
	}{
		{
			name: "markup smuggled inside a delimiter body",
			content: `<!-- wp:a gophenbergblockdelimiter1gophenbergblockdelimiter ` +
				`<script>alert(1)</script> --><!-- wp:b -->`,
			gone: "<script",
		},
		{
			name:    "comment closed early with the bang form",
			content: `<!-- wp:paragraph --><p>hi</p><!-- wp:x --!><img src=x onerror=alert(1)>-->`,
			gone:    "onerror",
		},
		{
			name:    "delimiter shape opened inside an attribute value",
			content: `<p class="<!-- wp:a"><script>alert(1)</script>-->">hi</p>`,
			gone:    "<script",
		},
		{
			name:    "delimiter shape closed inside an attribute value",
			content: `<!-- wp:image --><img src="x.png" alt="<!-- wp:z " onerror=alert(1) -->"><!-- /wp:image -->`,
			gone:    "onerror",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			sanitized := publichtml.Sanitize(tc.content)
			rendered := string(publichtml.Render(tc.content))

			if strings.Contains(sanitized, tc.gone) {
				t.Errorf("Sanitize() = %q, want %q gone", sanitized, tc.gone)
			}
			if strings.Contains(rendered, tc.gone) {
				t.Errorf("Render() = %q, want %q gone", rendered, tc.gone)
			}
		})
	}
}

func TestSanitizeIsUnaffectedByContentImitatingItsPlaceholder(t *testing.T) {
	t.Parallel()

	content := `<!-- wp:paragraph --><p>gophenbergblockdelimiter0gophenbergblockdelimiter</p><!-- /wp:paragraph -->`

	got := publichtml.Sanitize(content)

	if got != content {
		t.Errorf("Sanitize() = %q, want it byte stable at %q", got, content)
	}
}

func TestSanitizeKeepsCommentDelimitersTheParserNeeds(t *testing.T) {
	t.Parallel()

	content := `<!-- wp:group {"layout":{"type":"constrained"}} --><div class="wp-block-group">` +
		`<!-- wp:paragraph --><p>Inner</p><!-- /wp:paragraph --></div><!-- /wp:group -->`

	got := publichtml.Sanitize(content)

	for _, delimiter := range []string{
		`<!-- wp:group {"layout":{"type":"constrained"}} -->`,
		"<!-- wp:paragraph -->",
		"<!-- /wp:paragraph -->",
		"<!-- /wp:group -->",
	} {
		if !strings.Contains(got, delimiter) {
			t.Errorf("Sanitize() = %q, want it to carry %q", got, delimiter)
		}
	}
}

func TestSanitizeDropsCommentsThatAreNotBlockDelimiters(t *testing.T) {
	t.Parallel()

	content := "<!-- wp:paragraph --><p>Hi</p><!-- a stray note --><!-- /wp:paragraph -->"

	got := publichtml.Sanitize(content)

	if strings.Contains(got, "a stray note") {
		t.Errorf("Sanitize() = %q, want the stray comment dropped", got)
	}
	if !strings.Contains(got, "<!-- /wp:paragraph -->") {
		t.Errorf("Sanitize() = %q, want the block delimiters kept", got)
	}
}

func TestRenderLeavesNoDelimiters(t *testing.T) {
	t.Parallel()

	content := `<!-- wp:group --><div class="wp-block-group"><!-- wp:paragraph --><p>Inner</p>` +
		`<!-- /wp:paragraph --></div><!-- /wp:group -->`

	got := string(publichtml.Render(content))

	if strings.Contains(got, "wp:") {
		t.Errorf("Render() = %q, want every delimiter gone", got)
	}
	if !strings.Contains(got, "<p>Inner</p>") {
		t.Errorf("Render() = %q, want the block markup kept", got)
	}
}

func TestRenderSanitizesBeforeStripping(t *testing.T) {
	t.Parallel()

	content := `<!-- wp:paragraph --><p onclick="steal()">Hi</p><script>alert(1)</script><!-- /wp:paragraph -->`

	got := string(publichtml.Render(content))

	if strings.Contains(got, "onclick") || strings.Contains(got, "alert(1)") {
		t.Errorf("Render() = %q, want the scriptable markup stripped", got)
	}
}

func TestRenderTrimsSurroundingSpace(t *testing.T) {
	t.Parallel()

	content := "<!-- wp:paragraph -->\n<p>Hi</p>\n<!-- /wp:paragraph -->"

	got := string(publichtml.Render(content))

	if got != "<p>Hi</p>" {
		t.Errorf("Render() = %q, want %q", got, "<p>Hi</p>")
	}
}

func TestSanitizeDropsUnfetchableImageSources(t *testing.T) {
	t.Parallel()

	content := `<!-- wp:image --><img src="data:text/html;base64,PHNjcmlwdD4=" alt="x"/><!-- /wp:image -->`

	got := publichtml.Sanitize(content)

	if strings.Contains(got, "data:text/html") {
		t.Errorf("Sanitize() = %q, want the data URL dropped", got)
	}
}

func TestSanitizeKeepsRelativeLinksAndEncodedText(t *testing.T) {
	t.Parallel()

	cases := []string{
		`<!-- wp:paragraph --><p><a href="/post/hello-world">x</a></p><!-- /wp:paragraph -->`,
		"<!-- wp:paragraph --><p>a &amp; b &lt; c</p><!-- /wp:paragraph -->",
		"<!-- wp:paragraph --><p>Maria Perez wrote in Espanol</p><!-- /wp:paragraph -->",
	}
	for _, content := range cases {
		if got := publichtml.Sanitize(content); got != content {
			t.Errorf("Sanitize() = %q, want it byte stable at %q", got, content)
		}
	}
}

func TestSanitizeLeavesStyleValuesUnfiltered(t *testing.T) {
	t.Parallel()

	content := `<!-- wp:paragraph --><p style="width:expression(alert(1))">x</p><!-- /wp:paragraph -->`

	got := publichtml.Sanitize(content)

	if got != content {
		t.Errorf("Sanitize() = %q, want the accepted passthrough %q", got, content)
	}
}

func TestSanitizeAndRenderHandleEmptyContent(t *testing.T) {
	t.Parallel()

	if got := publichtml.Sanitize(""); got != "" {
		t.Errorf("Sanitize(%q) = %q, want %q", "", got, "")
	}
	if got := string(publichtml.Render("")); got != "" {
		t.Errorf("Render(%q) = %q, want %q", "", got, "")
	}
}
