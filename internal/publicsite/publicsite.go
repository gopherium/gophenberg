// SPDX-License-Identifier: Apache-2.0

// Package publicsite renders published posts as the built-in public site.
package publicsite

import (
	"bytes"
	"context"
	"embed"
	"errors"
	"html/template"
	"net/http"
	"path"
	"strconv"
	"strings"
	"time"

	"github.com/gopherium/gophenberg/internal/post"
	"github.com/gopherium/gophenberg/internal/publichtml"
	"github.com/gopherium/gophenberg/internal/version"
)

//go:embed templates
var files embed.FS

// postsPerPage is how many summaries a listing carries.
const postsPerPage = 20

// defaultTitle names the site when the configuration names none.
const defaultTitle = "Gophenberg"

// dateText and dateAttr are how a publication date reads and how a machine reads it.
const (
	dateText = "2 January 2006"
	dateAttr = time.DateOnly
)

var (
	indexTemplate    = template.Must(template.ParseFS(files, "templates/shell.html", "templates/index.html"))
	postTemplate     = template.Must(template.ParseFS(files, "templates/shell.html", "templates/post.html"))
	notFoundTemplate = template.Must(template.ParseFS(files, "templates/shell.html", "templates/notfound.html"))
)

// Reader reads the published posts the site serves.
type Reader interface {
	List(ctx context.Context, f post.Filter) ([]post.Post, int, error)
	PublishedBySlug(ctx context.Context, postType, slug string) (post.Post, error)
}

// Config carries what the site renders with.
type Config struct {
	// Posts reads the published content the site shows.
	Posts Reader
	// Title names the site in its chrome.
	Title string
	// Version is the application version the pages report.
	Version string
}

// Shell is the chrome every page carries.
type Shell struct {
	SiteTitle string
	Title     string
	Generator string
}

// summary is a published post as a listing shows it.
type summary struct {
	Title    string
	URL      string
	Excerpt  string
	DateText string
	DateAttr string
}

// detail is a published post as its own page shows it.
type detail struct {
	Title    string
	DateText string
	DateAttr string
	Content  template.HTML
}

// listData is what a listing page renders with.
type listData struct {
	Shell
	Posts []summary
	Older string
	Newer string
}

// detailData is what a single post page renders with.
type detailData struct {
	Shell
	Post detail
}

// site serves published posts as HTML pages.
type site struct {
	posts     Reader
	title     string
	generator string
}

// New returns the handler serving the built-in public site.
func New(cfg Config) http.Handler {
	title := cfg.Title
	if title == "" {
		title = defaultTitle
	}
	return &site{posts: cfg.Posts, title: title, generator: version.Generator(cfg.Version)}
}

// ServeHTTP renders the page the request addresses.
func (s *site) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	segments := pathSegments(r.URL.Path)
	switch {
	case len(segments) == 0:
		s.serveList(w, r, post.TypePost, 1)
	case len(segments) == 2:
		s.servePost(w, r, segments[0], segments[1])
	case len(segments) == 3 && segments[1] == "page":
		s.serveNumbered(w, r, segments[0], segments[2])
	default:
		s.serveNotFound(w, r)
	}
}

// pathSegments returns the non-empty segments of a URL path.
func pathSegments(raw string) []string {
	trimmed := strings.Trim(path.Clean(raw), "/")
	if trimmed == "" {
		return nil
	}
	return strings.Split(trimmed, "/")
}

// serveNumbered renders the numbered listing page of a post type.
func (s *site) serveNumbered(w http.ResponseWriter, r *http.Request, postType, raw string) {
	page, err := strconv.Atoi(raw)
	if err != nil {
		s.serveNotFound(w, r)
		return
	}
	s.serveList(w, r, postType, max(page, 1))
}

// serveList renders one page of published posts of a type.
func (s *site) serveList(w http.ResponseWriter, r *http.Request, postType string, page int) {
	rows, total, err := s.posts.List(r.Context(), post.Filter{
		Type:    postType,
		Status:  post.StatusPublished,
		OrderBy: post.OrderByDate,
		Order:   post.OrderDesc,
		Page:    page,
		PerPage: postsPerPage,
	})
	if err != nil {
		serveError(w)
		return
	}
	summaries := make([]summary, len(rows))
	for i, p := range rows {
		summaries[i] = newSummary(p)
	}
	s.render(w, indexTemplate, http.StatusOK, listData{
		Shell: s.shell(s.title),
		Posts: summaries,
		Older: olderLink(postType, page, total),
		Newer: newerLink(postType, page),
	})
}

// servePost renders one published post.
func (s *site) servePost(w http.ResponseWriter, r *http.Request, postType, slug string) {
	p, err := s.posts.PublishedBySlug(r.Context(), postType, slug)
	if errors.Is(err, post.ErrNotFound) {
		s.serveNotFound(w, r)
		return
	}
	if err != nil {
		serveError(w)
		return
	}
	s.render(w, postTemplate, http.StatusOK, detailData{
		Shell: s.shell(p.Title + " | " + s.title),
		Post:  newDetail(p),
	})
}

// serveNotFound renders the page reporting that nothing lives at an address.
func (s *site) serveNotFound(w http.ResponseWriter, _ *http.Request) {
	s.render(w, notFoundTemplate, http.StatusNotFound, listData{Shell: s.shell("Not found | " + s.title)})
}

// serveError reports that the site could not be rendered.
func serveError(w http.ResponseWriter) {
	http.Error(w, "the site is unavailable", http.StatusInternalServerError)
}

// shell returns the chrome a page carries under the given document title.
func (s *site) shell(title string) Shell {
	return Shell{SiteTitle: s.title, Title: title, Generator: s.generator}
}

// render writes the executed template, leaving a failed render an error rather than a partial page.
func (s *site) render(w http.ResponseWriter, tmpl *template.Template, status int, data any) {
	var page bytes.Buffer
	if err := tmpl.ExecuteTemplate(&page, "shell", data); err != nil {
		serveError(w)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(status)
	_, _ = page.WriteTo(w)
}

// newSummary returns the listing view of a published post.
func newSummary(p post.Post) summary {
	at := publishedAt(p)
	return summary{
		Title:    p.Title,
		URL:      "/" + p.Type + "/" + p.Slug,
		Excerpt:  p.Excerpt,
		DateText: at.Format(dateText),
		DateAttr: at.Format(dateAttr),
	}
}

// newDetail returns the page view of a published post.
func newDetail(p post.Post) detail {
	at := publishedAt(p)
	return detail{
		Title:    p.Title,
		DateText: at.Format(dateText),
		DateAttr: at.Format(dateAttr),
		Content:  publichtml.Render(p.Content),
	}
}

// publishedAt returns when the post went public, in UTC.
func publishedAt(p post.Post) time.Time {
	if p.PublishedAt != nil {
		return p.PublishedAt.UTC()
	}
	return p.UpdatedAt.UTC()
}

// olderLink returns the address of the page after this one, empty at the last.
func olderLink(postType string, page, total int) string {
	if page*postsPerPage >= total {
		return ""
	}
	return "/" + postType + "/page/" + strconv.Itoa(page+1)
}

// newerLink returns the address of the page before this one, empty at the first.
func newerLink(postType string, page int) string {
	if page <= 1 {
		return ""
	}
	if page == 2 && postType == post.TypePost {
		return "/"
	}
	return "/" + postType + "/page/" + strconv.Itoa(page-1)
}
