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
{ "gophenberg": "%VERSION%", "api": 1 }
```

One request tells you the site runs Gophenberg, which version, and
which API generation.

## Listing posts

```sh
curl "https://example.com/api/content/v1/posts?type=post&page=1&per_page=20"
```

| Parameter | Default | Meaning |
| --- | --- | --- |
| `type` | `post` | The post type to list |
| `page` | `1` | Which page |
| `per_page` | `20` | Posts per page, capped at 100 |

```json
{
  "items": [
    {
      "id": "0198f2c1-0000-7000-8000-000000000001",
      "type": "post",
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

## One post

```sh
curl https://example.com/api/content/v1/posts/post/hello-world
```

The response is the summary plus a `content` field: the post's
block markup as HTML, sanitized for public delivery, with the
block comment markers intact so a parser can identify each block.

Nothing published at that address answers
`404 {"error":"post: not found"}`.

Detail responses carry an `ETag`. Send it back as `If-None-Match`
and an unchanged post answers `304` with no body, worth doing if
you poll. Cross-origin browser scripts cannot read the `ETag`
header, so this is for servers and command lines.
