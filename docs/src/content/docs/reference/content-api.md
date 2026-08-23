---
title: Content API
description: The public read-only JSON API serving published posts.
---

Everything published on a Gophenberg site is readable over a small
JSON API. Themes are built on it, and so can anything else be: a
mobile app, a static site generator, another server.

## Conventions

- No authentication. The API serves only published content.
- Every response allows cross-origin reads
  (`Access-Control-Allow-Origin: *`).
- Responses are cacheable: `Cache-Control: public, s-maxage=60,
  stale-while-revalidate=300`.
- Errors are JSON: `{"error": "<message>"}` with the status code
  telling the kind.
- Timestamps are UTC in RFC 3339.

## The handshake

```sh
curl https://example.com/api/content/v1
```

```json
{
  "gophenberg": "%VERSION%",
  "api": 0,
  "kit": ["%KIT_VERSION%"],
  "types": [
    {
      "key": "post",
      "singular_label": "Post",
      "plural_label": "Posts",
      "route_word": "",
      "hierarchical": false,
      "page_kind": "single",
      "default": true,
      "fields": [
        {
          "key": "categories",
          "label": "Categories",
          "kind": "relation",
          "relates_to": "category",
          "many": true,
          "required": false
        }
      ]
    }
  ]
}
```

One request tells you the site runs Gophenberg, which version, which
API generation, and every content type it serves. The `route_word` is
the first segment of that type's addresses, and the type carrying an
empty one answers at the root of the site.

`kit` lists the [theme kit](/themes/writing-a-theme/) versions this
site serves, and `api` is the major version of the newest of them. A
site serving more than one kit major lists them all, so a reader can
tell whether the shape it was built against is still answered here.

Each type also lists the [fields](/guides/fields/) it declares, so a
reader knows what an item's values mean before fetching any. A
`page_kind` of `archive` marks a type whose items answer with a term
page, covered below.

## Listing items

```sh
curl "https://example.com/api/content/v1/items?type=post&page=1&per_page=20"
```

| Parameter | Default | Meaning |
| --- | --- | --- |
| `type` | the default type | The content type to list |
| `page` | `1` | Which page |
| `per_page` | `20` | Items per page, capped at 100 |

```json
{
  "items": [
    {
      "id": "0198f2c1-0000-7000-8000-000000000001",
      "type": "post",
      "path": "hello-world",
      "slug": "hello-world",
      "title": "Hello world",
      "excerpt": "The first post.",
      "published_at": "2026-08-01T10:00:00Z",
      "updated_at": "2026-08-02T09:30:00Z"
    }
  ],
  "total": 1,
  "page": 1,
  "per_page": 20
}
```

Listings never carry content, only summaries. Asking for a
`per_page` above 100 quietly serves 100. A page or `per_page`
below 1, or not a whole number, answers
`400 {"error":"invalid list parameters"}`. A type with nothing
published serves an empty page, not an error.

## Resolving an address

Every public address, from the front page to the deepest nested page,
is read through one endpoint.

```sh
curl "https://example.com/api/content/v1/resolve?path=/hello-world"
```

The answer names what lives there. A `kind` of `item` carries the
item itself, and a `kind` of `archive` carries a page of items.

```json
{
  "kind": "item",
  "type": { "key": "post", "route_word": "", "default": true },
  "item": {
    "id": "0198f2c1-0000-7000-8000-000000000001",
    "type": "post",
    "path": "hello-world",
    "slug": "hello-world",
    "title": "Hello world",
    "excerpt": "The first post.",
    "content": "<!-- wp:paragraph --><p>The first post.</p><!-- /wp:paragraph -->",
    "fields": {
      "categories": [
        {
          "id": "0198f2c1-0000-7000-8000-0000000000c1",
          "title": "News",
          "path": "categories/news"
        }
      ]
    },
    "published_at": "2026-08-01T10:00:00Z",
    "updated_at": "2026-08-02T09:30:00Z"
  }
}
```

An item's `content` is its block markup as HTML, sanitized for public
delivery, with the block comment markers intact so a parser can
identify each block.

The `fields` object carries the item's values, keyed by field key.
A scalar field holds its value as it was typed. A relation field
holds a list of the items it points at, each named and addressed so
a theme can link to it without another request. Only published
targets of active types appear, so a draft category never leaks
through a published post.

Archive addresses answer with a page instead. The root of a type is
its `route_word`, the front page is the default type, and both
paginate behind `/page/{n}`.

```sh
curl "https://example.com/api/content/v1/resolve?path=/pages/page/2"
```

```json
{
  "kind": "archive",
  "type": { "key": "page", "route_word": "pages", "hierarchical": true },
  "page": { "items": [], "total": 0, "page": 2, "per_page": 20 }
}
```

An address holding an item beats one that reads as a listing, so a page
genuinely filed at `pages/page/2` is served instead of the second
archive page. Nothing published at an address answers
`404 {"error":"content: not found"}`, and so does a page number below
one.

## Term pages

An item of a type whose `page_kind` is `archive` answers with a third
kind, `term`. The answer carries the item and a page together: the
item is the term itself, a category for example, and the page lists
the published content pointing at it, newest first.

```sh
curl "https://example.com/api/content/v1/resolve?path=/categories/news"
```

```json
{
  "kind": "term",
  "type": { "key": "category", "route_word": "categories", "page_kind": "archive" },
  "item": { "title": "News", "path": "categories/news" },
  "page": { "items": [], "total": 0, "page": 1, "per_page": 20 }
}
```

An item filed under the term through two different fields is listed
once. Term pages paginate behind `/page/{n}` like archives, and a
page suffix on an ordinary single item stays `404`.

Item answers carry an `ETag`. Send it back as `If-None-Match` and an
unchanged answer comes back as `304` with no body, worth doing if you
poll. The tag covers the whole answer, so it also moves when the type
or its fields change.
Cross-origin browser scripts cannot read the `ETag` header, so this is
for servers and command lines. Term answers carry no `ETag`, because
a term page changes when other items publish, which the term's own
timestamp cannot witness.
