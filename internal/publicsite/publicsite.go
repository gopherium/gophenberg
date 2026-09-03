// SPDX-License-Identifier: Apache-2.0

// Package publicsite renders published content as the built-in public site.
package publicsite

import (
	"bytes"
	"context"
	"embed"
	"errors"
	"html/template"
	"net/http"
	"strconv"
	"time"

	"github.com/google/uuid"

	"github.com/gopherium/gophenberg/internal/content"
	"github.com/gopherium/gophenberg/internal/i18n"
	"github.com/gopherium/gophenberg/internal/publichtml"
	"github.com/gopherium/gophenberg/internal/version"
)

//go:embed templates
var files embed.FS

// defaultTitle names the site when the configuration names none.
const defaultTitle = "Gophenberg"

// dateAttr is how a machine reads a publication date.
const dateAttr = time.DateOnly

// notFoundTitle names the missing page in the document title.
var notFoundTitle = i18n.Msgid("Not found")

var (
	indexTemplate    = template.Must(template.ParseFS(files, "templates/shell.html", "templates/index.html"))
	postTemplate     = template.Must(template.ParseFS(files, "templates/shell.html", "templates/content.html"))
	notFoundTemplate = template.Must(template.ParseFS(files, "templates/shell.html", "templates/notfound.html"))
	termTemplate     = template.Must(template.ParseFS(files, "templates/shell.html", "templates/term.html"))
)

// Reader reads the published content the site serves.
type Reader interface {
	List(ctx context.Context, f content.Filter) ([]content.Content, int, error)
	PublishedByPath(ctx context.Context, path string) (content.Content, error)
	RelatedTo(ctx context.Context, target uuid.UUID, page, perPage int) ([]content.Content, int, error)
}

// Config carries what the site renders with.
type Config struct {
	// Content reads the published content the site shows.
	Content Reader
	// Locale answers the language the site chose for itself. Nil leaves the site in en-US.
	Locale SiteLocale
	// Types answers which content type an address belongs to.
	Types content.TypeReader
	// Title names the site in its chrome.
	Title string
	// Version is the application version the pages report.
	Version string
}

// SiteLocale answers the language the site chose for itself.
type SiteLocale interface {
	Lookup(ctx context.Context, key string) (string, bool, error)
}

// Shell is the chrome every page carries.
type Shell struct {
	SiteTitle string
	Title     string
	Generator string
	Lang      string
	T         *i18n.Translator
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

// termData is what a term page renders with, the item and what points at it.
type termData struct {
	Shell
	Post  detail
	Posts []summary
	Older string
	Newer string
}

// site serves published content as HTML pages.
type site struct {
	posts     Reader
	addresses *content.Resolver
	title     string
	generator string
	locale    SiteLocale
}

// New returns the handler serving the built-in public site.
func New(cfg Config) http.Handler {
	title := cfg.Title
	if title == "" {
		title = defaultTitle
	}
	return &site{
		posts:     cfg.Content,
		addresses: content.NewResolver(cfg.Content, cfg.Types),
		title:     title,
		generator: version.Generator(cfg.Version),
		locale:    cfg.Locale,
	}
}

// ServeHTTP renders the page the request addresses.
func (s *site) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	held, err := s.addresses.Resolve(r.Context(), r.URL.Path)
	if errors.Is(err, content.ErrNotFound) {
		s.serveNotFound(w, r)
		return
	}
	if err != nil {
		serveError(w)
		return
	}
	if held.Kind == content.KindArchive {
		s.serveList(w, r, held.Type, held.Page)
		return
	}
	if held.Kind == content.KindTerm {
		s.serveTerm(w, r, held)
		return
	}
	s.serveItem(w, r, held.Item)
}

// serveTerm renders a term page, the item itself above what points at it.
func (s *site) serveTerm(w http.ResponseWriter, r *http.Request, held content.Address) {
	perPage := s.pageSize(r.Context())
	rows, total, err := s.posts.RelatedTo(r.Context(), held.Item.ID, held.Page, perPage)
	if err != nil {
		serveError(w)
		return
	}
	shell := s.shell(r.Context(), held.Item.Title+" | "+s.title)
	summaries := make([]summary, len(rows))
	for i, p := range rows {
		summaries[i] = newSummary(p, shell.T)
	}
	s.render(w, termTemplate, http.StatusOK, termData{
		Shell: shell,
		Post:  newDetail(held.Item, shell.T),
		Posts: summaries,
		Older: olderLink(held.Item.Path, held.Page, total, perPage),
		Newer: newerLink(held.Item.Path, held.Page),
	})
}

// serveList renders one page of the published content of a type.
func (s *site) serveList(w http.ResponseWriter, r *http.Request, listed content.Type, page int) {
	perPage := s.pageSize(r.Context())
	rows, total, err := s.posts.List(r.Context(), content.Filter{
		Type:    listed.Key,
		Status:  content.StatusPublished,
		OrderBy: content.OrderByDate,
		Order:   content.OrderDesc,
		Page:    page,
		PerPage: perPage,
	})
	if err != nil {
		serveError(w)
		return
	}
	shell := s.shell(r.Context(), s.listingTitle(listed))
	summaries := make([]summary, len(rows))
	for i, p := range rows {
		summaries[i] = newSummary(p, shell.T)
	}
	s.render(w, indexTemplate, http.StatusOK, listData{
		Shell: shell,
		Posts: summaries,
		Older: olderLink(listed.RouteWord, page, total, perPage),
		Newer: newerLink(listed.RouteWord, page),
	})
}

// listingTitle returns the document title a listing carries.
func (s *site) listingTitle(listed content.Type) string {
	if listed.RouteWord == "" {
		return s.title
	}
	return listed.PluralLabel + " | " + s.title
}

// serveItem renders one published content item.
func (s *site) serveItem(w http.ResponseWriter, r *http.Request, held content.Content) {
	shell := s.shell(r.Context(), held.Title+" | "+s.title)
	s.render(w, postTemplate, http.StatusOK, detailData{Shell: shell, Post: newDetail(held, shell.T)})
}

// serveNotFound renders the page reporting that nothing lives at an address.
func (s *site) serveNotFound(w http.ResponseWriter, r *http.Request) {
	shell := s.shell(r.Context(), "")
	shell.Title = shell.T.Get(notFoundTitle) + " | " + s.title
	s.render(w, notFoundTemplate, http.StatusNotFound, listData{Shell: shell})
}

// serveError reports that the site could not be rendered.
func serveError(w http.ResponseWriter) {
	http.Error(w, "the site is unavailable", http.StatusInternalServerError)
}

// pageSize returns how many summaries a listing carries, as the site chose or by default.
func (s *site) pageSize(ctx context.Context) int {
	if s.locale == nil {
		return content.DefaultPerPage
	}
	held, found, err := s.locale.Lookup(ctx, content.PerPageSettingKey)
	if err != nil {
		return content.DefaultPerPage
	}
	return content.ResolvePerPage(held, found)
}

// language returns the language the site chose for itself, en-US when it chose none.
func (s *site) language(ctx context.Context) string {
	if s.locale == nil {
		return i18n.DefaultLocale
	}
	held, found, err := s.locale.Lookup(ctx, content.LocaleSettingKey)
	if err != nil || !found || held == "" {
		return i18n.DefaultLocale
	}
	return held
}

// shell returns the chrome a page carries under the given document title.
func (s *site) shell(ctx context.Context, title string) Shell {
	locale := s.language(ctx)
	return Shell{
		SiteTitle: s.title,
		Title:     title,
		Generator: s.generator,
		Lang:      locale,
		T:         i18n.For(locale),
	}
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

// newSummary returns the listing view of a published content.
func newSummary(p content.Content, t *i18n.Translator) summary {
	at := publishedAt(p)
	return summary{
		Title:    p.Title,
		URL:      "/" + p.Path,
		Excerpt:  p.Excerpt,
		DateText: t.Date(at),
		DateAttr: at.Format(dateAttr),
	}
}

// newDetail returns the page view of a published content.
func newDetail(p content.Content, t *i18n.Translator) detail {
	at := publishedAt(p)
	return detail{
		Title:    p.Title,
		DateText: t.Date(at),
		DateAttr: at.Format(dateAttr),
		Content:  publichtml.Render(p.Content),
	}
}

// publishedAt returns when the post went public, in UTC.
func publishedAt(p content.Content) time.Time {
	if p.PublishedAt != nil {
		return p.PublishedAt.UTC()
	}
	return p.UpdatedAt.UTC()
}

// olderLink returns the address of the page after this one, empty at the last.
func olderLink(routeWord string, page, total, perPage int) string {
	if page >= (total+perPage-1)/perPage {
		return ""
	}
	return numberedPage(routeWord, page+1)
}

// newerLink returns the address of the page before this one, empty at the first.
func newerLink(routeWord string, page int) string {
	if page <= 1 {
		return ""
	}
	if page == 2 {
		return "/" + routeWord
	}
	return numberedPage(routeWord, page-1)
}

// numberedPage returns the address a numbered listing page answers at.
func numberedPage(routeWord string, page int) string {
	return "/" + content.AddressUnder(routeWord, content.PageWord+"/"+strconv.Itoa(page))
}
