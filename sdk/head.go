// SPDX-License-Identifier: Apache-2.0

package sdk

import "context"

// PluginRoutesPrefix is the address every plugin's routes answer under, before the plugin's id.
const PluginRoutesPrefix = "/api/plugins/"

// HeadTagsHook is the name a plugin lists under hooks in its plugin.json when it implements HeadContributor.
const HeadTagsHook = "public.head-tags"

// PageKind names the kind of public page the built-in site draws.
type PageKind string

const (
	// PageArchive is a listing of one content type, the home listing among them.
	PageArchive PageKind = "archive"
	// PageItem is one post, page or other item.
	PageItem PageKind = "item"
	// PageTerm is an item shown above the items that point at it.
	PageTerm PageKind = "term"
	// PageNotFound is the page that says nothing lives at the address.
	PageNotFound PageKind = "notfound"
)

// Attr is one attribute of a head tag.
type Attr struct {
	Key   string
	Value string
}

// HeadTag is one meta or link element a plugin adds to the head of a page, its attributes in the order given.
type HeadTag struct {
	Tag   string
	Attrs []Attr
}

// PageMeta describes the public page the built-in site draws.
type PageMeta struct {
	Kind PageKind
	// Type is the key of the content type the page shows, empty on the not found page.
	Type string
	// Path is the address the visitor asked for, such as /blog/page/2.
	Path string
	// Page is the number of the listing page, 1 on the first and 0 on a page that lists nothing.
	Page int
	// Item is the item an item or term page shows, its Content and Fields left empty, and nil on other pages.
	Item *Item
	// SiteTitle is the name of the site.
	SiteTitle string
}

// HeadContributor is implemented by plugins that add tags to the head of every page the built-in site draws.
type HeadContributor interface {
	HeadTags(ctx context.Context, page PageMeta) ([]HeadTag, error)
}
