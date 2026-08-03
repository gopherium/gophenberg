// SPDX-License-Identifier: Apache-2.0

// Package feed serves the published posts as an RSS channel.
package feed

import (
	"context"
	"encoding/xml"
	"fmt"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/gopherium/gophenberg/sdk"
)

// pluginID names the plugin in the manifest and the host.
const pluginID = "feed"

// feedPath is the namespace relative address the channel is served at.
const feedPath = "/rss.xml"

// defaultTitle names the channel when the environment names none.
const defaultTitle = "Gophenberg"

// defaultItems caps the posts a channel carries when the environment caps none.
const defaultItems = 20

// postType is the type of post the channel carries.
const postType = "post"

// blockComment matches the delimiters the editor wraps blocks in.
var blockComment = regexp.MustCompile(`<!-- /?wp:[\s\S]*?-->`)

// plugin serves published posts as an RSS channel.
type plugin struct {
	posts sdk.PostReader
	title string
	items int
}

// Register returns the feed plugin configured from the environment.
func Register(deps sdk.Deps) (sdk.Plugin, error) {
	title := deps.Getenv("GOPHENBERG_FEED_TITLE")
	if title == "" {
		title = defaultTitle
	}
	items, err := itemsFrom(deps.Getenv("GOPHENBERG_FEED_ITEMS"))
	if err != nil {
		return nil, err
	}
	return &plugin{
		posts: deps.Posts,
		title: title,
		items: items,
	}, nil
}

// itemsFrom returns the cap the given value asks for, or the default when it is empty.
func itemsFrom(raw string) (int, error) {
	if raw == "" {
		return defaultItems, nil
	}
	items, err := strconv.Atoi(raw)
	if err != nil || items < 1 {
		return 0, fmt.Errorf("feed: GOPHENBERG_FEED_ITEMS must be a positive integer, got %q", raw)
	}
	return items, nil
}

// ID returns the plugin's identifier.
func (p *plugin) ID() string {
	return pluginID
}

// Start readies the plugin.
func (p *plugin) Start(_ context.Context) error {
	return nil
}

// Stop releases what the plugin holds.
func (p *plugin) Stop(_ context.Context) error {
	return nil
}

// PublicPaths returns the addresses served without a session.
func (p *plugin) PublicPaths() []string {
	return []string{feedPath}
}

// Routes returns the handler serving the channel.
func (p *plugin) Routes() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET "+feedPath, p.serveFeed)
	return mux
}

// originOf returns the scheme and host the request reached the site under.
func originOf(r *http.Request) string {
	scheme := r.Header.Get("X-Forwarded-Proto")
	if scheme == "" {
		scheme = "http"
		if r.TLS != nil {
			scheme = "https"
		}
	}
	return scheme + "://" + r.Host
}

// serveFeed writes the published posts as an RSS channel.
func (p *plugin) serveFeed(w http.ResponseWriter, r *http.Request) {
	posts, err := p.posts.ListPublished(r.Context(), postType, p.items)
	if err != nil {
		http.Error(w, "feed unavailable", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/rss+xml; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(xml.Header))
	if err := xml.NewEncoder(w).Encode(p.channelOf(originOf(r), posts)); err != nil {
		return
	}
}

// rss is the document a reader parses.
type rss struct {
	XMLName xml.Name   `xml:"rss"`
	Version string     `xml:"version,attr"`
	Channel rssChannel `xml:"channel"`
}

// rssChannel carries the channel metadata and its items.
type rssChannel struct {
	Title string    `xml:"title"`
	Link  string    `xml:"link"`
	Items []rssItem `xml:"item"`
}

// rssItem is one published post as the channel carries it.
type rssItem struct {
	Title       string  `xml:"title"`
	Link        string  `xml:"link"`
	Description string  `xml:"description"`
	GUID        rssGUID `xml:"guid"`
	PubDate     string  `xml:"pubDate"`
}

// rssGUID identifies a post without pointing at a page.
type rssGUID struct {
	IsPermaLink string `xml:"isPermaLink,attr"`
	Value       string `xml:",chardata"`
}

// channelOf returns the channel carrying the given posts under the given origin.
func (p *plugin) channelOf(origin string, posts []sdk.Post) rss {
	items := make([]rssItem, len(posts))
	for i, published := range posts {
		items[i] = rssItem{
			Title:       published.Title,
			Link:        origin + "/" + published.Type + "/" + published.Slug,
			Description: stripBlockComments(published.Content),
			GUID:        rssGUID{IsPermaLink: "false", Value: "urn:uuid:" + published.ID.String()},
			PubDate:     published.PublishedAt.Format(time.RFC1123Z),
		}
	}
	return rss{
		Version: "2.0",
		Channel: rssChannel{Title: p.title, Link: origin, Items: items},
	}
}

// stripBlockComments returns content without the delimiters the editor wraps blocks in.
func stripBlockComments(content string) string {
	return strings.TrimSpace(blockComment.ReplaceAllString(content, ""))
}
