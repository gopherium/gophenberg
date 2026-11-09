// SPDX-License-Identifier: Apache-2.0

// Package headtag checks the tags plugins add to the head of a page and writes them as HTML.
package headtag

import (
	"errors"
	"fmt"
	"html/template"
	"regexp"
	"slices"
	"strings"

	"github.com/gopherium/gophenberg/sdk"
)

// keptTags are the element names a plugin may add to the head of a page.
var keptTags = []string{"meta", "link"}

// addressKeys are the attributes whose value must be an http, https or site address.
var addressKeys = []string{"href", "src"}

// attributeName matches an attribute name in lowercase letters, digits and hyphens, a letter first.
var attributeName = regexp.MustCompile(`^[a-z][a-z0-9-]*$`)

// Keep returns the tags inside the rules, in order, and an error naming each tag it dropped and why.
func Keep(tags []sdk.HeadTag) ([]sdk.HeadTag, error) {
	kept := make([]sdk.HeadTag, 0, len(tags))
	var dropped []error
	for _, tag := range tags {
		if err := check(tag); err != nil {
			dropped = append(dropped, fmt.Errorf("dropped the head tag %s: %w", tag.Tag, err))
			continue
		}
		kept = append(kept, tag)
	}
	return kept, errors.Join(dropped...)
}

// check refuses a tag that is not meta or link, an attribute name outside the rules or an address outside them.
func check(tag sdk.HeadTag) error {
	if !slices.Contains(keptTags, tag.Tag) {
		return errors.New("only meta and link are kept")
	}
	for _, attr := range tag.Attrs {
		if !attributeName.MatchString(attr.Key) {
			return fmt.Errorf("its attribute %q is not a lowercase name", attr.Key)
		}
		if slices.Contains(addressKeys, attr.Key) && !isAddress(attr.Value) {
			return fmt.Errorf("its %s must be an http, https or site address", attr.Key)
		}
	}
	return nil
}

// isAddress reports whether value is an http or https address, or a path on the site that starts with one slash.
func isAddress(value string) bool {
	if strings.HasPrefix(value, "http:") || strings.HasPrefix(value, "https:") {
		return true
	}
	path, ok := strings.CutPrefix(value, "/")
	if !ok {
		return false
	}
	return !strings.HasPrefix(path, "/") && !strings.HasPrefix(path, `\`)
}

// Write returns the tags as HTML, one element a line, each value escaped so it stays inside its quotes.
func Write(tags []sdk.HeadTag) template.HTML {
	var written strings.Builder
	for _, tag := range tags {
		written.WriteString("<" + tag.Tag)
		for _, attr := range tag.Attrs {
			written.WriteString(" " + attr.Key + `="` + template.HTMLEscapeString(attr.Value) + `"`)
		}
		written.WriteString(">\n")
	}
	return template.HTML(written.String()) //nolint:gosec // Keep checked the tags and every value is escaped.
}
