// SPDX-License-Identifier: Apache-2.0

package content_test

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"golang.org/x/text/unicode/norm"

	"github.com/gopherium/gophenberg/internal/content"
)

var errEntropy = errors.New("entropy source failed")

// postType returns the built-in post type as the registry holds it.
func postType() content.Type {
	now := time.Now().UTC()
	return content.Type{
		Key:           content.TypePost,
		SingularLabel: "Post",
		PluralLabel:   "Posts",
		Description:   "Manage the posts on this site.",
		Revisions:     true,
		RevisionCap:   100,
		PageKind:      content.PageKindSingle,
		Default:       true,
		Active:        true,
		CreatedAt:     now,
		UpdatedAt:     now,
	}
}

type failingReader struct{}

// Read fails with the canned entropy error.
func (failingReader) Read([]byte) (int, error) {
	return 0, errEntropy
}

func TestNewReportsIDGenerationFailure(t *testing.T) {
	author := uuid.Must(uuid.NewV7())
	uuid.SetRand(failingReader{})
	defer uuid.SetRand(nil)

	_, err := content.New(postType(), nil, "Hello World", author)

	if !errors.Is(err, errEntropy) {
		t.Fatalf("New() error = %v, want the entropy failure in its chain", err)
	}
}

func TestNewReturnsADraftWithGeneratedFields(t *testing.T) {
	t.Parallel()

	author := uuid.Must(uuid.NewV7())
	before := time.Now().UTC()

	p, err := content.New(postType(), nil, "Hello World", author)

	if err != nil {
		t.Fatalf("New() error = %v, want nil", err)
	}
	if p.ID == uuid.Nil {
		t.Error("ID is nil, want a generated identifier")
	}
	if p.Status != content.StatusDraft {
		t.Errorf("Status = %q, want %q", p.Status, content.StatusDraft)
	}
	if p.Slug != "hello-world" {
		t.Errorf("Slug = %q, want %q", p.Slug, "hello-world")
	}
	if p.Title != "Hello World" {
		t.Errorf("Title = %q, want the title preserved", p.Title)
	}
	if p.Type != "post" {
		t.Errorf("Type = %q, want %q", p.Type, "post")
	}
	if p.AuthorID != author {
		t.Errorf("AuthorID = %v, want %v", p.AuthorID, author)
	}
	if p.PublishedAt != nil {
		t.Errorf("PublishedAt = %v, want nil on a draft", p.PublishedAt)
	}
	if !p.CreatedAt.Equal(p.UpdatedAt) {
		t.Errorf("CreatedAt = %v, UpdatedAt = %v, want them equal on creation", p.CreatedAt, p.UpdatedAt)
	}
	if p.CreatedAt.Before(before) {
		t.Errorf("CreatedAt = %v, want an instant at or after %v", p.CreatedAt, before)
	}
	if p.CreatedAt.Location() != time.UTC {
		t.Errorf("CreatedAt location = %v, want UTC", p.CreatedAt.Location())
	}
}

func TestNewTitlesWithoutSlugCharactersFallBackToUntitled(t *testing.T) {
	t.Parallel()

	p, err := content.New(postType(), nil, "   ", uuid.Must(uuid.NewV7()))

	if err != nil {
		t.Fatalf("New() error = %v, want nil", err)
	}
	if p.Slug != "untitled" {
		t.Errorf("Slug = %q, want %q", p.Slug, "untitled")
	}
}

func TestNewRejectsInvalidInput(t *testing.T) {
	t.Parallel()

	inactive := postType()
	inactive.Active = false
	tests := map[string]struct {
		contentType content.Type
		author      uuid.UUID
		want        error
	}{
		"unregistered type": {contentType: content.Type{}, author: uuid.Must(uuid.NewV7()), want: content.ErrInvalidType},
		"inactive type":     {contentType: inactive, author: uuid.Must(uuid.NewV7()), want: content.ErrTypeInactive},
		"missing author":    {contentType: postType(), author: uuid.Nil, want: content.ErrInvalidAuthor},
	}

	for testName, tc := range tests {
		t.Run(testName, func(t *testing.T) {
			t.Parallel()

			_, err := content.New(tc.contentType, nil, "Hello", tc.author)

			if !errors.Is(err, tc.want) {
				t.Errorf("New() error = %v, want %v", err, tc.want)
			}
		})
	}
}

func TestSlugify(t *testing.T) {
	t.Parallel()

	tests := map[string]struct {
		in   string
		want string
	}{
		"lowercases and joins words":   {in: "Hello World", want: "hello-world"},
		"collapses runs of separators": {in: "multiple   spaces", want: "multiple-spaces"},
		"trims leading and trailing":   {in: "---edged---", want: "edged"},
		"drops apostrophes":            {in: "How's it going", want: "hows-it-going"},
		"splits en and em dashes":      {in: "News–Events—Index", want: "news-events-index"},
		"keeps digits":                 {in: "Top 10 Reasons", want: "top-10-reasons"},
		"keeps unicode letters":        {in: "Cafe Munchen", want: "cafe-munchen"},
		"lowercases unicode":           {in: "MARIA PEREZ", want: "maria-perez"},
		"replaces punctuation":         {in: "Hello, World! Again?", want: "hello-world-again"},
		"empty falls back":             {in: "", want: "untitled"},
		"separators only fall back":    {in: "!!! ???", want: "untitled"},
	}

	for testName, tc := range tests {
		t.Run(testName, func(t *testing.T) {
			t.Parallel()

			if got := content.Slugify(tc.in); got != tc.want {
				t.Errorf("Slugify(%q) = %q, want %q", tc.in, got, tc.want)
			}
		})
	}
}

func TestSlugifyStripsAccentsLikeWordPress(t *testing.T) {
	t.Parallel()

	tests := map[string]struct {
		in   string
		want string
	}{
		"precomposed accent":                         {in: "Café", want: "cafe"},
		"decomposed accent":                          {in: "Cafe\u0301", want: "cafe"},
		"decomposed accent mid-word":                 {in: "Cafe\u0301s", want: "cafes"},
		"several accents":                            {in: "Crème Brûlée", want: "creme-brulee"},
		"sharp s expands":                            {in: "Straße", want: "strasse"},
		"ligatures expand":                           {in: "Ærøskøbing Œuvre", want: "aeroskobing-oeuvre"},
		"stroked and barred letters":                 {in: "Łódź Đà Nẵng", want: "lodz-da-nang"},
		"caron and ring":                             {in: "Dvořák Ångström", want: "dvorak-angstrom"},
		"dotted capital i":                           {in: "İstanbul", want: "istanbul"},
		"thorn and eth":                              {in: "Þórr Ðú", want: "thorr-du"},
		"ordinal indicators fold":                    {in: "1ª 2º", want: "1a-2o"},
		"schwa and turned e fold to e":               {in: "Əbə Ǝǝ", want: "ebe-ee"},
		"latin alpha folds to a":                     {in: "ɑb", want: "ab"},
		"cjk is kept":                                {in: "東京タワー", want: "東京タワー"},
		"cyrillic is kept intact":                    {in: "Йога Привет", want: "йога-привет"},
		"greek is kept":                              {in: "Αθήνα", want: "αθήνα"},
		"separate accent on a greek capital":         {in: "\u03aa\u0301\u03b1", want: "\u0390\u03b1"},
		"accented ligature folds to its table form":  {in: "Ǽsop Ǿ", want: "aesop-o"},
		"decomposed accented ligature folds too":     {in: "\u00c6\u0301sop", want: "aesop"},
		"separate mark after a latin letter dropped": {in: "J\u030cana", want: "jana"},
		"separate mark mid-word dropped":             {in: "x\u0301ray", want: "xray"},
		"stacked separate marks dropped":             {in: "a\u0301\u0302bc", want: "abc"},
		"spacing mark after a folded accent kept":    {in: "é\u1bf2z", want: "e\u1bf2z"},
		"accent after a digit dropped":               {in: "7\u0301up", want: "7up"},
		"accent after a cyrillic letter dropped":     {in: "и\u0301ра", want: "ира"},
		"devanagari marks are kept":                  {in: "हिन्दी भाषा", want: "हिन्दी-भाषा"},
		"thai marks are kept":                        {in: "สวัสดี", want: "สวัสดี"},
		"tamil marks are kept":                       {in: "தமிழ் மொழி", want: "தமிழ்-மொழி"},
		"mark after a separator separates":           {in: "a \u093fb", want: "a-b"},
		"grave after a digit dropped":                {in: "7\U00000300up", want: "7up"},
		"macron after a digit dropped":               {in: "7\U00000304up", want: "7up"},
		"caron after a digit dropped":                {in: "7\U0000030Cup", want: "7up"},
		"grapheme joiner after a digit dropped":      {in: "7\U0000034Fup", want: "7up"},
		"other mark after a digit kept":              {in: "3\U00000308d", want: "3\U00000308d"},
		"other mark after a cyrillic letter kept": {
			in:   "\U00000440\U00000435\U0000030F\U0000043A\U00000430",
			want: "\U00000440\U00000435\U0000030F\U0000043A\U00000430",
		},
		"nukta behind a grapheme joiner composes":  {in: "\U00000928\U0000034F\U0000093C", want: "\U00000929"},
		"hangul jamo across an apostrophe compose": {in: "\U00001100'\U00001161", want: "\U0000AC00"},
		"compatibility digraph is left as is":      {in: "Ǆ", want: "ǆ"},
	}

	for testName, tc := range tests {
		t.Run(testName, func(t *testing.T) {
			t.Parallel()

			if got := content.Slugify(tc.in); got != tc.want {
				t.Errorf("Slugify(%q) = %q, want %q", tc.in, got, tc.want)
			}
		})
	}
}

func TestSlugifyKeepsUnderscoresAndDropsWordPressStrippedSymbols(t *testing.T) {
	t.Parallel()

	tests := map[string]struct {
		in   string
		want string
	}{
		"keeps underscores":               {in: "my_post", want: "my_post"},
		"keeps underscores beside words":  {in: "My_Post title", want: "my_post-title"},
		"drops ampersand without a break": {in: "Q&A", want: "qa"},
		"drops the degree sign":           {in: "Bake at 180°C", want: "bake-at-180c"},
		"multiplication sign becomes x":   {in: "2×4", want: "2x4"},
		"drops the soft hyphen in a word": {in: "so\u00adft", want: "soft"},
		"mark after a dropped symbol":     {in: "Noe&\U00000308l", want: "noe-l"},
		"emoji leaves no selector":        {in: "Favorites\U00002764\U0000FE0F", want: "favorites"},
		"ideographic comma splits":        {in: "\U00004E00\U00003001\U00004E8C", want: "\U00004E00-\U00004E8C"},
		"fraction slash splits":           {in: "1\U00002044" + "2", want: "1-2"},
		"non-breaking hyphen splits":      {in: "non\U00002011breaking", want: "non-breaking"},
		"dots and slashes split":          {in: "v1.2/notes", want: "v1-2-notes"},
		"ascii punctuation drops":         {in: "C++ & C#", want: "c-c"},
		"listed trademark sign drops":     {in: "Brand\U00002122 Name", want: "brand-name"},
		"listed modifier acute drops":     {in: "Q\U000002CAA", want: "qa"},
	}

	for testName, tc := range tests {
		t.Run(testName, func(t *testing.T) {
			t.Parallel()

			if got := content.Slugify(tc.in); got != tc.want {
				t.Errorf("Slugify(%q) = %q, want %q", tc.in, got, tc.want)
			}
		})
	}
}

func TestRouteWordBreaksAtUnderscoresTheWaySlugsNoLongerDo(t *testing.T) {
	t.Parallel()

	tests := map[string]struct {
		in   string
		want string
	}{
		"breaks a label's underscores":  {in: "FAQ_Items", want: "faq-items"},
		"breaks a leading underscore":   {in: "_posts", want: "posts"},
		"folds accents as Slugify does": {in: "Café_Ærø", want: "cafe-aero"},
	}

	for testName, tc := range tests {
		t.Run(testName, func(t *testing.T) {
			t.Parallel()

			if got := content.RouteWord(tc.in); got != tc.want {
				t.Errorf("RouteWord(%q) = %q, want %q", tc.in, got, tc.want)
			}
		})
	}
}

func TestSlugifyTruncatesLongTitlesWithoutTrailingSeparator(t *testing.T) {
	t.Parallel()

	got := content.Slugify(strings.Repeat("a", 199) + " b")

	if want := strings.Repeat("a", 199); got != want {
		t.Errorf("Slugify() = %q with length %d, want %d characters and no trailing hyphen", got, len(got), len(want))
	}
}

func TestSlugifyTruncatesBeforeACharacterCutFromItsMarks(t *testing.T) {
	t.Parallel()

	got := content.Slugify(strings.Repeat("a", 198) + " कि")

	if want := strings.Repeat("a", 198); got != want {
		t.Errorf("Slugify() = %q, want %q", got, want)
	}
}

func FuzzSlugifyGivesTheSameSlugBack(f *testing.F) {
	for _, seed := range []string{
		"Crème Brûlée",
		"\U00000939\U0000093F\U00000928\U00000926\U00000940 \U0000092D\U00000937\U0000093E",
		"\U00000928\U0000034F\U0000093C",
		"\U00001100'\U00001161",
		"admin'\U0000FE0F",
		"admin&\U0000FE0F",
		"Noe&\U00000308l",
		"Favorites\U00002764\U0000FE0F",
		"\U00000915" + strings.Repeat("\U00000951", 40),
		strings.Repeat("a", 198) + " \U00000915\U0000093F",
	} {
		f.Add(seed)
	}
	f.Fuzz(func(t *testing.T, title string) {
		slug := content.Slugify(title)
		if again := content.Slugify(slug); again != slug || !norm.NFC.IsNormalString(slug) {
			t.Errorf("Slugify(%q) = %q, which is not NFC or slugifies again to %q", title, slug, again)
		}
	})
}
