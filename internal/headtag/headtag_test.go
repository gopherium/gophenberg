// SPDX-License-Identifier: Apache-2.0

package headtag_test

import (
	"html/template"
	"slices"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/gopherium/gophenberg/internal/headtag"
	"github.com/gopherium/gophenberg/sdk"
)

// tagOf returns the head tag called name with the attributes given as key and value pairs.
func tagOf(name string, pairs ...string) sdk.HeadTag {
	tag := sdk.HeadTag{Tag: name}
	for i := 0; i+1 < len(pairs); i += 2 {
		tag.Attrs = append(tag.Attrs, sdk.Attr{Key: pairs[i], Value: pairs[i+1]})
	}
	return tag
}

func TestKeepKeepsMetaAndLinkTagsInsideTheRules(t *testing.T) {
	t.Parallel()

	tags := []sdk.HeadTag{
		tagOf("meta", "name", "description", "content", "A site about gophers"),
		tagOf("meta", "http-equiv", "refresh", "content", "30"),
		tagOf("link", "rel", "alternate", "href", "https://example.com/rss.xml"),
		tagOf("link", "rel", "preconnect", "href", "http://example.com"),
		tagOf("link", "rel", "icon", "href", "/icon.svg"),
		tagOf("meta", "property", "og:image", "src", "/image.png"),
		tagOf("meta"),
	}

	kept, err := headtag.Keep(tags)

	if err != nil || !slices.EqualFunc(kept, tags, sameTag) {
		t.Errorf("Keep() = %+v, %v, want every tag kept and no error", kept, err)
	}
}

func TestKeepDropsEachTagOutsideTheRulesAndNamesWhy(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name string
		tag  sdk.HeadTag
		want string
	}{
		{"a script", tagOf("script", "src", "/app.js"), "dropped the head tag script: only meta and link are kept"},
		{"a capital tag name", tagOf("META", "name", "x"), "dropped the head tag META: only meta and link are kept"},
		{
			"a script address", tagOf("link", "rel", "x", "href", "javascript:alert(1)"),
			"dropped the head tag link: its href must be an http, https or site address",
		},
		{
			"a protocol relative address", tagOf("link", "href", "//example.com/x.css"),
			"dropped the head tag link: its href must be an http, https or site address",
		},
		{
			"a backslash after the slash", tagOf("link", "href", `/\example.com/x.css`),
			"dropped the head tag link: its href must be an http, https or site address",
		},
		{
			"a space before the scheme", tagOf("meta", "src", " https://example.com"),
			"dropped the head tag meta: its src must be an http, https or site address",
		},
		{
			"a capital scheme", tagOf("link", "href", "HTTPS://example.com"),
			"dropped the head tag link: its href must be an http, https or site address",
		},
		{
			"an empty address", tagOf("link", "href", ""),
			"dropped the head tag link: its href must be an http, https or site address",
		},
		{
			"a space in a key", tagOf("meta", "on top", "x"),
			`dropped the head tag meta: its attribute "on top" is not a lowercase name`,
		},
		{
			"a digit first", tagOf("meta", "2col", "x"),
			`dropped the head tag meta: its attribute "2col" is not a lowercase name`,
		},
		{
			"a capital in a key", tagOf("meta", "Name", "x"),
			`dropped the head tag meta: its attribute "Name" is not a lowercase name`,
		},
		{"an empty key", tagOf("meta", "", "x"), `dropped the head tag meta: its attribute "" is not a lowercase name`},
		{
			"an event attribute", tagOf("link", "rel", "stylesheet", "href", "/style.css", "onload", "alert(1)"),
			`dropped the head tag link: its attribute "onload" runs script or styles the page`,
		},
		{
			"an error event attribute", tagOf("link", "rel", "preload", "href", "/x", "onerror", "alert(1)"),
			`dropped the head tag link: its attribute "onerror" runs script or styles the page`,
		},
		{
			"a style attribute", tagOf("meta", "name", "x", "style", "display:none"),
			`dropped the head tag meta: its attribute "style" runs script or styles the page`,
		},
		{
			"a tab after the first slash", tagOf("link", "href", "/\t/example.com/x.css"),
			"dropped the head tag link: its href must be an http, https or site address",
		},
		{
			"a line break after the first slash", tagOf("link", "href", "/\n/example.com/x.css"),
			"dropped the head tag link: its href must be an http, https or site address",
		},
		{
			"a carriage return after the scheme", tagOf("link", "href", "https:\r//example.com/x.css"),
			"dropped the head tag link: its href must be an http, https or site address",
		},
		{
			"a space inside the address", tagOf("link", "href", "/ /example.com/x.css"),
			"dropped the head tag link: its href must be an http, https or site address",
		},
		{
			"a delete character", tagOf("link", "href", "/x\x7f.css"),
			"dropped the head tag link: its href must be an http, https or site address",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			good := tagOf("meta", "name", "kept")
			kept, err := headtag.Keep([]sdk.HeadTag{good, tc.tag})

			if err == nil || err.Error() != tc.want {
				t.Errorf("Keep() error = %v, want %q", err, tc.want)
			}
			if !slices.EqualFunc(kept, []sdk.HeadTag{good}, sameTag) {
				t.Errorf("Keep() = %+v, want only the good tag", kept)
			}
		})
	}
}

func TestKeepNamesEveryTagItDropped(t *testing.T) {
	t.Parallel()

	_, err := headtag.Keep([]sdk.HeadTag{tagOf("script"), tagOf("style")})

	want := "dropped the head tag script: only meta and link are kept\n" +
		"dropped the head tag style: only meta and link are kept"
	if err == nil || err.Error() != want {
		t.Errorf("Keep() error = %v, want %q", err, want)
	}
}

func TestKeepAnswersATagListOfItsOwn(t *testing.T) {
	t.Parallel()

	given := []sdk.HeadTag{tagOf("meta", "name", "first"), tagOf("script")}

	kept, _ := headtag.Keep(given)
	kept[0] = tagOf("link")

	if given[0].Tag != "meta" {
		t.Errorf("Keep() answered the list it was given, the caller changed it to %+v", given[0])
	}
}

func TestKeepKeepsTheAttributesItChecked(t *testing.T) {
	t.Parallel()

	given := []sdk.HeadTag{tagOf("link", "rel", "icon", "href", "/icon.svg")}

	kept, err := headtag.Keep(given)
	given[0].Attrs[0] = sdk.Attr{Key: "onload", Value: "alert(1)"}
	given[0].Attrs[1].Value = "javascript:alert(1)"

	if want := tagOf("link", "rel", "icon", "href", "/icon.svg"); err != nil || !sameTag(kept[0], want) {
		t.Errorf("Keep() = %+v, %v, want the attributes it checked %+v whatever the plugin changes after", kept, err, want)
	}
}

func TestWriteWritesOneLinePerTagWithItsAttributesInOrder(t *testing.T) {
	t.Parallel()

	got := headtag.Write([]sdk.HeadTag{
		tagOf("link", "rel", "alternate", "type", "application/rss+xml", "title", "Gophenberg",
			"href", "/api/plugins/feed/rss.xml"),
		tagOf("meta", "name", "robots", "content", "noindex"),
		tagOf("meta"),
	})

	want := template.HTML(`<link rel="alternate" type="application/rss+xml" title="Gophenberg" ` +
		`href="/api/plugins/feed/rss.xml">` + "\n" + `<meta name="robots" content="noindex">` + "\n<meta>\n")
	if got != want {
		t.Errorf("Write() = %q, want %q", got, want)
	}
}

func TestWriteWritesNothingForNoTags(t *testing.T) {
	t.Parallel()

	if got := headtag.Write(nil); got != "" {
		t.Errorf("Write(nil) = %q, want nothing", got)
	}
}

func TestWriteEscapesAValueSoItStaysInsideItsQuotes(t *testing.T) {
	t.Parallel()

	for value, want := range map[string]string{
		"Plain words":             "Plain words",
		`Ann & "Bob's" <shop>`:    "Ann &amp; &#34;Bob&#39;s&#34; &lt;shop&gt;",
		"C++ notes":               "C++ notes",
		"nul\x00here":             "nul" + string(utf8.RuneError) + "here",
		"Café ünïcode":            "Café ünïcode",
		`" onload="alert(1)`:      "&#34; onload=&#34;alert(1)",
		"application/rss+xml":     "application/rss+xml",
		"`backtick` = sign":       "`backtick` = sign",
		"line\nbreak and\ttabbed": "line\nbreak and\ttabbed",
	} {
		got := headtag.Write([]sdk.HeadTag{tagOf("meta", "content", value)})

		if string(got) != `<meta content="`+want+`">`+"\n" {
			t.Errorf("Write() of %q = %q, want the value written as %q", value, got, want)
		}
	}
}

func TestWriteWritesTheFeedLinkAsTheShellTemplateWroteIt(t *testing.T) {
	t.Parallel()

	shell := template.Must(template.New("link").Parse(
		`<link rel="alternate" type="application/rss+xml" title="{{.}}" href="/api/plugins/feed/rss.xml">`))
	for _, title := range []string{"Gophenberg", "A Test Site", `Ann & "Bob's" shop`} {
		var want strings.Builder
		if err := shell.Execute(&want, title); err != nil {
			t.Fatalf("executing the template over %q: %v", title, err)
		}

		got := headtag.Write([]sdk.HeadTag{tagOf("link", "rel", "alternate", "type", "application/rss+xml",
			"title", title, "href", "/api/plugins/feed/rss.xml")})

		if string(got) != want.String()+"\n" {
			t.Errorf("Write() for the title %q = %q, want %q as the template wrote it", title, got, want.String()+"\n")
		}
	}
}

// sameTag reports whether both tags carry the same name and the same attributes in the same order.
func sameTag(a, b sdk.HeadTag) bool {
	return a.Tag == b.Tag && slices.Equal(a.Attrs, b.Attrs)
}
