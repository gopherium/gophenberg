// SPDX-License-Identifier: Apache-2.0

// Package feed serves the published posts as an RSS channel.
package feed

import (
	"context"
	"encoding/xml"
	"net/http"
	"regexp"
	"strings"
	"time"

	"github.com/gopherium/gophenberg/sdk"
)

// pluginID names the plugin in the manifest and the host.
const pluginID = "feed"

// feedPath is the namespace relative address the channel is served at.
const feedPath = "/rss.xml"

// titleSetting is the key of the setting that names the channel.
const titleSetting = "title"

// itemsSetting is the key of the setting that caps the posts a channel carries.
const itemsSetting = "items"

// postType is the type of post the channel carries.
const postType = "post"

// blockComment matches the delimiters the editor wraps blocks in.
var blockComment = regexp.MustCompile(`<!-- /?wp:[\s\S]*?-->`)

// plugin serves published posts as an RSS channel.
type plugin struct {
	posts    sdk.ContentReader
	settings sdk.Settings
}

// Register returns the feed plugin, which reads its settings on each request.
func Register(deps sdk.Deps) (sdk.Plugin, error) {
	return &plugin{posts: deps.Content, settings: deps.Settings}, nil
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

// HeadTags links the channel from the head of every page, titled with the site's name.
func (p *plugin) HeadTags(_ context.Context, page sdk.PageMeta) ([]sdk.HeadTag, error) {
	return []sdk.HeadTag{{Tag: "link", Attrs: []sdk.Attr{
		{Key: "rel", Value: "alternate"}, {Key: "type", Value: "application/rss+xml"},
		{Key: "title", Value: page.SiteTitle}, {Key: "href", Value: sdk.PluginRoutesPrefix + pluginID + feedPath},
	}}}, nil
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
	posts, err := p.posts.ListPublished(r.Context(), postType, p.settings.Count(itemsSetting))
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
func (p *plugin) channelOf(origin string, posts []sdk.Item) rss {
	items := make([]rssItem, len(posts))
	for i, published := range posts {
		items[i] = rssItem{
			Title:       published.Title,
			Link:        origin + "/" + published.Path,
			Description: stripBlockComments(published.Content),
			GUID:        rssGUID{IsPermaLink: "false", Value: "urn:uuid:" + published.ID.String()},
			PubDate:     published.PublishedAt.Format(time.RFC1123Z),
		}
	}
	return rss{
		Version: "2.0",
		Channel: rssChannel{Title: p.settings.Text(titleSetting), Link: origin, Items: items},
	}
}

// stripBlockComments returns content without the delimiters the editor wraps blocks in.
func stripBlockComments(content string) string {
	return strings.TrimSpace(blockComment.ReplaceAllString(content, ""))
}
