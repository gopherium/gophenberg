// SPDX-License-Identifier: Apache-2.0

package features_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"regexp"
	"slices"

	"github.com/cucumber/godog"
)

// quotedWords finds every quoted title a step names.
var quotedWords = regexp.MustCompile(`"([^"]*)"`)

// quotedTitles returns the titles a step names, in the order it names them.
func quotedTitles(listed string) []string {
	found := quotedWords.FindAllStringSubmatch(listed, -1)
	titles := make([]string, len(found))
	for i, match := range found {
		titles[i] = match[1]
	}
	return titles
}

// adminPage is one page of the admin content list as its readers decode it.
type adminPage struct {
	Items []struct {
		ID         string `json:"id"`
		Title      string `json:"title"`
		AuthorName string `json:"author_name"`
	} `json:"items"`
	Total   int `json:"total"`
	PerPage int `json:"per_page"`
}

// listPage asks the admin list of the type for what the query names, keeping the answer.
func listPage(ctx context.Context, typeKey string, query url.Values) error {
	w, err := worldOf(ctx)
	if err != nil {
		return err
	}
	query.Set("type", typeKey)
	return w.get(contentPath + "?" + query.Encode())
}

// theAdministratorListsThePosts asks for the first page of posts.
func theAdministratorListsThePosts(ctx context.Context) error {
	return listPage(ctx, "post", url.Values{})
}

// theAdministratorListsThePostsWithTheStatus asks for the posts holding any of the statuses.
func theAdministratorListsThePostsWithTheStatus(ctx context.Context, statuses string) error {
	return listPage(ctx, "post", url.Values{"status": {statuses}})
}

// accountID returns the identity of the stored account carrying the email.
func accountID(ctx context.Context, email string) (string, error) {
	w, err := worldOf(ctx)
	if err != nil {
		return "", err
	}
	held, err := w.users.UserByEmail(ctx, email)
	if err != nil {
		return "", fmt.Errorf("finding %s: %w", email, err)
	}
	return held.ID.String(), nil
}

// theAdministratorListsThePostsTheAuthorWrote asks for the posts the author wrote.
func theAdministratorListsThePostsTheAuthorWrote(ctx context.Context) error {
	id, err := accountID(ctx, authorEmail)
	if err != nil {
		return err
	}
	return listPage(ctx, "post", url.Values{"author": {id}})
}

// theAdministratorListsThePostsTheAuthorDidNotWrite asks for the posts every other account wrote.
func theAdministratorListsThePostsTheAuthorDidNotWrite(ctx context.Context) error {
	id, err := accountID(ctx, authorEmail)
	if err != nil {
		return err
	}
	return listPage(ctx, "post", url.Values{"author_exclude": {id}})
}

// theAdministratorListsThePostsWrittenBy asks for the posts an author value names.
func theAdministratorListsThePostsWrittenBy(ctx context.Context, author string) error {
	return listPage(ctx, "post", url.Values{"author": {author}})
}

// theAdministratorListsThePostsDatedBefore asks for the posts dated before the start of the day.
func theAdministratorListsThePostsDatedBefore(ctx context.Context, day string) error {
	return listPage(ctx, "post", url.Values{"before": {day + "T00:00:00Z"}})
}

// theAdministratorListsThePostsDatedAfter asks for the posts dated after the start of the day.
func theAdministratorListsThePostsDatedAfter(ctx context.Context, day string) error {
	return listPage(ctx, "post", url.Values{"after": {day + "T00:00:00Z"}})
}

// theAccountListsThePostsSortedBy asks for the posts sorted by the column in the direction.
func theAccountListsThePostsSortedBy(ctx context.Context, column, direction string) error {
	return listPage(ctx, "post", url.Values{"orderby": {column}, "order": {direction}})
}

// theAdministratorListsThePagesSortedBy asks for the pages sorted by the column in the direction.
func theAdministratorListsThePagesSortedBy(ctx context.Context, column, direction string) error {
	return listPage(ctx, "page", url.Values{"orderby": {column}, "order": {direction}})
}

// theAdministratorListsThePagesNested asks for the pages by title, each child under its parent.
func theAdministratorListsThePagesNested(ctx context.Context) error {
	return listPage(ctx, "page", url.Values{
		"orderby": {"title"}, "order": {"asc"}, "orderby_hierarchy": {"true"},
	})
}

// theAdministratorListsThePostsAtATime asks for the posts in pages of the size.
func theAdministratorListsThePostsAtATime(ctx context.Context, size int) error {
	return listPage(ctx, "post", url.Values{"per_page": {fmt.Sprint(size)}})
}

// listed returns the page the last answer carried.
func listed(ctx context.Context) (adminPage, error) {
	w, err := worldOf(ctx)
	if err != nil {
		return adminPage{}, err
	}
	if err := w.expect(http.StatusOK); err != nil {
		return adminPage{}, fmt.Errorf("listing the content: %w", err)
	}
	var page adminPage
	if err := w.answer.decode(&page); err != nil {
		return adminPage{}, err
	}
	return page, nil
}

// listedTitles returns the titles of the last page, in the order it carried them.
func listedTitles(ctx context.Context) ([]string, error) {
	page, err := listed(ctx)
	if err != nil {
		return nil, err
	}
	titles := make([]string, len(page.Items))
	for i, item := range page.Items {
		titles[i] = item.Title
	}
	return titles, nil
}

// theListHolds asserts the last page carries exactly the titles, in any order.
func theListHolds(ctx context.Context, names string) error {
	titles, err := listedTitles(ctx)
	if err != nil {
		return err
	}
	want := quotedTitles(names)
	slices.Sort(titles)
	slices.Sort(want)
	if !slices.Equal(titles, want) {
		return fmt.Errorf("the list holds %q, want %q", titles, want)
	}
	return nil
}

// theListReadsInThatOrder asserts the last page carries exactly the titles, in that order.
func theListReadsInThatOrder(ctx context.Context, names string) error {
	titles, err := listedTitles(ctx)
	if err != nil {
		return err
	}
	if want := quotedTitles(names); !slices.Equal(titles, want) {
		return fmt.Errorf("the list reads %q, want %q", titles, want)
	}
	return nil
}

// theListEndsWith asserts the last page carries the title last.
func theListEndsWith(ctx context.Context, title string) error {
	titles, err := listedTitles(ctx)
	if err != nil {
		return err
	}
	if len(titles) == 0 || titles[len(titles)-1] != title {
		return fmt.Errorf("the list reads %q, want it to end with %q", titles, title)
	}
	return nil
}

// theItemIsCreditedTo asserts the listed item names the account that wrote it.
func theItemIsCreditedTo(ctx context.Context, title, name string) error {
	page, err := listed(ctx)
	if err != nil {
		return err
	}
	for _, item := range page.Items {
		if item.Title == title {
			if item.AuthorName != name {
				return fmt.Errorf("%q is credited to %q, want %q", title, item.AuthorName, name)
			}
			return nil
		}
	}
	return fmt.Errorf("the list carries no %q", title)
}

// theListWasPagedAtATime asserts the list answered the page size it used.
func theListWasPagedAtATime(ctx context.Context, size int) error {
	page, err := listed(ctx)
	if err != nil {
		return err
	}
	if page.PerPage != size {
		return fmt.Errorf("the list was paged %d at a time, want %d", page.PerPage, size)
	}
	return nil
}

// theItemWasPublishedOn stamps the day a stored item went public.
func theItemWasPublishedOn(ctx context.Context, title, day string) error {
	w, err := worldOf(ctx)
	if err != nil {
		return err
	}
	return w.publishedOn(title, day)
}

// theAccountListsTheAuthors asks for the accounts that may write.
func theAccountListsTheAuthors(ctx context.Context) error {
	w, err := worldOf(ctx)
	if err != nil {
		return err
	}
	return w.get("/api/authors")
}

// theAuthorsAre asserts the authors list names exactly the accounts, in that order.
func theAuthorsAre(ctx context.Context, names string) error {
	w, err := worldOf(ctx)
	if err != nil {
		return err
	}
	if err := w.expect(http.StatusOK); err != nil {
		return fmt.Errorf("listing the authors: %w", err)
	}
	var authors struct {
		Items []struct {
			ID   string `json:"id"`
			Name string `json:"name"`
		} `json:"items"`
	}
	if err := w.answer.decode(&authors); err != nil {
		return err
	}
	held := make([]string, len(authors.Items))
	for i, author := range authors.Items {
		held[i] = author.Name
	}
	if want := quotedTitles(names); !slices.Equal(held, want) {
		return fmt.Errorf("the authors are %q, want %q", held, want)
	}
	return nil
}

// theAdministratorCreatesThePostWith stores a post carrying its excerpt and content from the start.
func theAdministratorCreatesThePostWith(ctx context.Context, title, excerpt, body string) error {
	w, err := worldOf(ctx)
	if err != nil {
		return err
	}
	encoded, err := json.Marshal(map[string]string{
		"type": "post", "title": title, "excerpt": excerpt, "content": body,
	})
	if err != nil {
		return fmt.Errorf("building the request: %w", err)
	}
	if err := w.postJSON(contentPath, string(encoded)); err != nil {
		return err
	}
	if err := w.expect(http.StatusCreated); err != nil {
		return err
	}
	var stored nestedContent
	if err := w.answer.decode(&stored); err != nil {
		return err
	}
	if w.nested == nil {
		w.nested = make(map[string]nestedContent)
	}
	w.nested[title] = stored
	return nil
}

// thePostHoldsTheExcerptAndTheContent asserts the stored post carries the excerpt and the content.
func thePostHoldsTheExcerptAndTheContent(ctx context.Context, title, excerpt, body string) error {
	w, err := worldOf(ctx)
	if err != nil {
		return err
	}
	held, found := w.nested[title]
	if !found {
		return fmt.Errorf("the scenario stored nothing titled %q", title)
	}
	if err := w.get(contentPath + "/" + held.ID); err != nil {
		return err
	}
	if err := w.expect(http.StatusOK); err != nil {
		return err
	}
	var stored struct {
		Excerpt string `json:"excerpt"`
		Content string `json:"content"`
	}
	if err := w.answer.decode(&stored); err != nil {
		return err
	}
	if stored.Excerpt != excerpt || stored.Content != body {
		return fmt.Errorf("the post holds %q and %q, want %q and %q", stored.Excerpt, stored.Content, excerpt, body)
	}
	return nil
}

// theAccountEmptiesTheTrashOf asks to empty the trash of the type.
func theAccountEmptiesTheTrashOf(ctx context.Context, typeKey string) error {
	w, err := worldOf(ctx)
	if err != nil {
		return err
	}
	return w.deleteAt(contentPath + "/trash?" + url.Values{"type": {typeKey}}.Encode())
}

// theTrashAnswerCounts asserts how many items emptying the trash deleted and kept.
func theTrashAnswerCounts(ctx context.Context, deleted, kept int) error {
	w, err := worldOf(ctx)
	if err != nil {
		return err
	}
	if err := w.expect(http.StatusOK); err != nil {
		return fmt.Errorf("emptying the trash: %w", err)
	}
	var emptied struct {
		Deleted int `json:"deleted"`
		Kept    int `json:"kept"`
	}
	if err := w.answer.decode(&emptied); err != nil {
		return err
	}
	if emptied.Deleted != deleted || emptied.Kept != kept {
		return fmt.Errorf("the trash answer counts %d deleted and %d kept, want %d and %d",
			emptied.Deleted, emptied.Kept, deleted, kept)
	}
	return nil
}

// storedStatus reads the stored item the scenario remembers under the title, answering its status code and status.
func storedStatus(ctx context.Context, title string) (int, string, error) {
	w, err := worldOf(ctx)
	if err != nil {
		return 0, "", err
	}
	held, found := w.nested[title]
	if !found {
		return 0, "", fmt.Errorf("the scenario stored nothing titled %q", title)
	}
	read, err := w.send(http.MethodGet, contentPath+"/"+held.ID, "", nil)
	if err != nil {
		return 0, "", err
	}
	var stored struct {
		Status string `json:"status"`
	}
	if read.status == http.StatusOK {
		if err := read.decode(&stored); err != nil {
			return 0, "", err
		}
	}
	return read.status, stored.Status, nil
}

// theItemIsGone asserts the item the scenario stored is no longer stored.
func theItemIsGone(ctx context.Context, title string) error {
	code, _, err := storedStatus(ctx, title)
	if err != nil {
		return err
	}
	if code != http.StatusNotFound {
		return fmt.Errorf("reading %q answered %d, want it gone", title, code)
	}
	return nil
}

// theItemIsStillInTheTrash asserts the item the scenario stored still waits in the trash.
func theItemIsStillInTheTrash(ctx context.Context, title string) error {
	code, status, err := storedStatus(ctx, title)
	if err != nil {
		return err
	}
	if code != http.StatusOK || status != "trash" {
		return fmt.Errorf("reading %q answered %d holding %q, want it in the trash", title, code, status)
	}
	return nil
}

// theAccountReadsTheSettings asks for what the site chose and what the lists read.
func theAccountReadsTheSettings(ctx context.Context) error {
	w, err := worldOf(ctx)
	if err != nil {
		return err
	}
	return w.get("/api/settings")
}

// listSettingsRead is what the settings answer carries for the lists.
type listSettingsRead struct {
	PageSizes         []int  `json:"list_page_sizes"`
	PageSize          int    `json:"list_page_size"`
	ToastMilliseconds int    `json:"toast_milliseconds"`
	ToastNameLength   int    `json:"toast_name_length"`
	FormatLocale      string `json:"format_locale"`
}

// settingsRead returns what the last settings answer carried for the lists.
func settingsRead(ctx context.Context) (listSettingsRead, error) {
	w, err := worldOf(ctx)
	if err != nil {
		return listSettingsRead{}, err
	}
	if err := w.expect(http.StatusOK); err != nil {
		return listSettingsRead{}, fmt.Errorf("reading the settings: %w", err)
	}
	var held listSettingsRead
	if err := w.answer.decode(&held); err != nil {
		return listSettingsRead{}, err
	}
	return held, nil
}

// theSettingsOfferThePageSizes asserts the sizes a list offers and the one it opens at.
func theSettingsOfferThePageSizes(ctx context.Context, first, second, third, fourth, opening int) error {
	held, err := settingsRead(ctx)
	if err != nil {
		return err
	}
	if want := []int{first, second, third, fourth}; !slices.Equal(held.PageSizes, want) || held.PageSize != opening {
		return fmt.Errorf("the settings offer %v opening at %d, want %v opening at %d",
			held.PageSizes, held.PageSize, want, opening)
	}
	return nil
}

// theSettingsKeepAToast asserts how long a toast stays and how much of a name it shows.
func theSettingsKeepAToast(ctx context.Context, milliseconds, length int) error {
	held, err := settingsRead(ctx)
	if err != nil {
		return err
	}
	if held.ToastMilliseconds != milliseconds || held.ToastNameLength != length {
		return fmt.Errorf("the settings keep a toast %d milliseconds naming %d characters, want %d and %d",
			held.ToastMilliseconds, held.ToastNameLength, milliseconds, length)
	}
	return nil
}

// theSettingsWriteDatesAndNumbersIn asserts the locale dates and numbers are written in.
func theSettingsWriteDatesAndNumbersIn(ctx context.Context, locale string) error {
	held, err := settingsRead(ctx)
	if err != nil {
		return err
	}
	if held.FormatLocale != locale {
		return fmt.Errorf("the settings write in %q, want %q", held.FormatLocale, locale)
	}
	return nil
}

// initializeContentList registers the steps of the content list feature.
func initializeContentList(sc *godog.ScenarioContext) {
	sc.Before(provisionWorld)
	sc.After(retireWorld)
	sc.Given(`^a running Gophenberg with the default content types$`, aRunningGophenbergWithTheDefaultContentTypes)
	sc.Given(`^a signed in administrator$`, aSignedInAdministrator)
	sc.Given(`^a signed in author$`, aSignedInAuthor)
	sc.Given(
		`^the type "([^"]*)" labeled "([^"]*)" and "([^"]*)" under "([^"]*)" that nests$`,
		theNestingTypeExists,
	)
	sc.Given(`^the post "([^"]*)"$`, thePostExists)
	sc.Given(`^the published post "([^"]*)"$`, thePublishedPost)
	sc.Given(`^the page "([^"]*)"$`, thePageExists)
	sc.Given(`^the page "([^"]*)" filed under "([^"]*)"$`, thePageFiledUnder)
	sc.Given(`^"([^"]*)" was published on "([^"]*)"$`, theItemWasPublishedOn)
	sc.Step(`^the (?:administrator|account) trashes "([^"]*)"$`, theAdministratorDeletes)
	sc.When(`^the administrator lists the posts$`, theAdministratorListsThePosts)
	sc.When(`^the administrator lists the posts with the status "([^"]*)"$`, theAdministratorListsThePostsWithTheStatus)
	sc.When(`^the administrator lists the posts the author wrote$`, theAdministratorListsThePostsTheAuthorWrote)
	sc.When(`^the administrator lists the posts the author did not write$`,
		theAdministratorListsThePostsTheAuthorDidNotWrite)
	sc.When(`^the administrator lists the posts written by "([^"]*)"$`, theAdministratorListsThePostsWrittenBy)
	sc.When(`^the administrator lists the posts dated before "([^"]*)"$`, theAdministratorListsThePostsDatedBefore)
	sc.When(`^the administrator lists the posts dated after "([^"]*)"$`, theAdministratorListsThePostsDatedAfter)
	sc.When(`^the (?:administrator|account) lists the posts sorted by "([^"]*)" "([^"]*)"$`,
		theAccountListsThePostsSortedBy)
	sc.When(`^the administrator lists the pages sorted by "([^"]*)" "([^"]*)"$`, theAdministratorListsThePagesSortedBy)
	sc.When(`^the administrator lists the pages by title nested under their parents$`,
		theAdministratorListsThePagesNested)
	sc.When(`^the administrator lists the posts (\d+) at a time$`, theAdministratorListsThePostsAtATime)
	sc.When(`^the account lists the authors$`, theAccountListsTheAuthors)
	sc.When(`^the administrator creates the post "([^"]*)" with the excerpt "([^"]*)" and the content "([^"]*)"$`,
		theAdministratorCreatesThePostWith)
	sc.When(`^the account empties the trash of "([^"]*)"$`, theAccountEmptiesTheTrashOf)
	sc.When(`^the account reads the settings$`, theAccountReadsTheSettings)
	sc.Then(`^the list holds ((?:"[^"]*"(?:, | and )?)+)$`, theListHolds)
	sc.Then(`^the list reads ((?:"[^"]*"(?:, | and )?)+) in that order$`, theListReadsInThatOrder)
	sc.Then(`^the list ends with "([^"]*)"$`, theListEndsWith)
	sc.Then(`^"([^"]*)" is credited to "([^"]*)"$`, theItemIsCreditedTo)
	sc.Then(`^the list was paged (\d+) at a time$`, theListWasPagedAtATime)
	sc.Then(`^the authors are ((?:"[^"]*"(?:, | and )?)+)$`, theAuthorsAre)
	sc.Then(`^the post "([^"]*)" holds the excerpt "([^"]*)" and the content "([^"]*)"$`,
		thePostHoldsTheExcerptAndTheContent)
	sc.Then(`^the trash answer counts (\d+) deleted and (\d+) kept$`, theTrashAnswerCounts)
	sc.Then(`^"([^"]*)" is gone$`, theItemIsGone)
	sc.Then(`^"([^"]*)" is still in the trash$`, theItemIsStillInTheTrash)
	sc.Then(`^the request is refused with the code "([^"]*)"$`, theRequestIsRefusedWithTheCode)
	sc.Then(`^the settings offer the page sizes (\d+), (\d+), (\d+) and (\d+) opening at (\d+)$`,
		theSettingsOfferThePageSizes)
	sc.Then(`^the settings keep a toast (\d+) milliseconds naming at most (\d+) characters$`, theSettingsKeepAToast)
	sc.Then(`^the settings write dates and numbers in "([^"]*)"$`, theSettingsWriteDatesAndNumbersIn)
}
