// SPDX-License-Identifier: Apache-2.0

// Package publichtml prepares stored block content for public delivery.
package publichtml

import (
	"crypto/rand"
	"html/template"
	"regexp"
	"strconv"
	"strings"

	"github.com/microcosm-cc/bluemonday"
	"golang.org/x/net/html"
)

// blockDelimiter matches the comment delimiters the editor wraps blocks in, whose attributes carry
// no unescaped angle bracket, ampersand, or double hyphen.
var blockDelimiter = regexp.MustCompile(
	`<!-- /?wp:[a-z][a-z0-9_-]*(?:/[a-z][a-z0-9_-]*)?(?: \{[^<>&]*\})? ?/?-->`)

// policy is the markup a public page may carry.
var policy = newPolicy()

// newPolicy returns the policy allowing what the editor's static blocks serialize.
func newPolicy() *bluemonday.Policy {
	p := bluemonday.NewPolicy()
	p.RequireParseableURLs(true)
	p.AllowRelativeURLs(true)
	p.AllowURLSchemes("mailto", "http", "https")
	p.AllowElements(
		"p", "h1", "h2", "h3", "h4", "h5", "h6", "blockquote", "cite", "pre", "code",
		"ul", "ol", "li", "dl", "dt", "dd", "figure", "figcaption", "div", "span", "details", "summary",
		"strong", "em", "b", "i", "u", "s", "del", "ins", "mark", "sub", "sup", "kbd", "small",
		"table", "thead", "tbody", "tfoot", "tr", "th", "td", "caption", "colgroup", "col",
		"br", "hr",
	)
	p.AllowAttrs("href", "target").OnElements("a")
	p.AllowAttrs("src", "alt", "width", "height", "loading", "decoding", "srcset", "sizes").OnElements("img")
	p.AllowAttrs("colspan", "rowspan").OnElements("td", "th")
	p.AllowAttrs("start", "reversed", "type").OnElements("ol")
	p.AllowAttrs("open").OnElements("details")
	p.AllowAttrs("class", "id", "style", "title", "lang", "dir").Globally()
	p.AllowAttrs("role", "aria-hidden", "aria-label", "aria-labelledby", "aria-describedby").Globally()
	return p
}

// Sanitize returns content carrying only markup a public page may serve, block delimiters intact.
func Sanitize(content string) string {
	mark := "gophenberg" + strings.ToLower(rand.Text())
	swapped, marked := swapDelimiters(content, mark)
	return marked.Replace(policy.Sanitize(swapped))
}

// swapDelimiters returns content with each delimiter comment swapped for a numbered mark, and the replacer undoing it.
func swapDelimiters(content, mark string) (string, *strings.Replacer) {
	var swapped strings.Builder
	var pairs []string
	tokens := html.NewTokenizer(strings.NewReader(content))
	for kind := tokens.Next(); kind != html.ErrorToken; kind = tokens.Next() {
		raw := string(tokens.Raw())
		if kind != html.CommentToken || blockDelimiter.FindString(raw) != raw {
			swapped.WriteString(raw)
			continue
		}
		numbered := mark + strconv.Itoa(len(pairs)) + "-"
		pairs = append(pairs, numbered, raw)
		swapped.WriteString(numbered)
	}
	return swapped.String(), strings.NewReplacer(pairs...)
}

// Render returns content as public page markup, with the block delimiters removed.
func Render(content string) template.HTML {
	stripped := blockDelimiter.ReplaceAllString(Sanitize(content), "")
	return template.HTML(strings.TrimSpace(stripped)) //nolint:gosec // Sanitize applied the policy.
}
