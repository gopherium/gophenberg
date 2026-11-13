// SPDX-License-Identifier: Apache-2.0

package app

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"slices"
	"strings"
	"testing"

	"github.com/cucumber/godog"

	"github.com/gopherium/gophenberg/sdk"
)

// pluginContentChangesFeature is the feature a site answers to while its plugins hear content changes.
const pluginContentChangesFeature = "../../test/features/features/plugin-content-changes.feature"

// heldPost is a post the administrator wrote: its id and the version its next write is prepared against.
type heldPost struct {
	id        string
	updatedAt json.RawMessage
}

// writtenPost is the part of a written item a scenario keeps.
type writtenPost struct {
	ID        string          `json:"id"`
	Title     string          `json:"title"`
	Status    string          `json:"status"`
	UpdatedAt json.RawMessage `json:"updated_at"`
}

// listeningPlugin is a pausing plugin that hears content changes as the scenario set.
type listeningPlugin struct {
	*pausingPlugin
}

// ContentChanged keeps the change the plugin heard, then fails or panics as the scenario set.
func (l listeningPlugin) ContentChanged(_ context.Context, change sdk.ContentChange) error {
	l.scenario.mu.Lock()
	l.heard = append(l.heard, change)
	failure, boom := l.hearErr, l.hearPanic
	l.scenario.mu.Unlock()
	if boom != "" {
		panic(boom)
	}
	if failure != "" {
		return errors.New(failure)
	}
	return nil
}

// bindContentChanges binds the steps of the scenarios where plugins hear content changes.
func (p *pausingScenario) bindContentChanges(sc *godog.ScenarioContext) {
	sc.Given(`^"([^"]*)" hears content changes$`, p.hearsChanges)
	sc.Given(`^the content changes "([^"]*)" hears (fail|panic) with "([^"]*)"$`, p.hearingGoesWrong)
	sc.When(`^the administrator writes the draft "([^"]*)"$`, func(title string) error {
		return p.writesDraft(title, map[string]any{"type": "post"})
	})
	sc.When(`^the administrator writes the draft page "([^"]*)"$`, func(title string) error {
		return p.writesDraft(title, map[string]any{"type": "page"})
	})
	sc.When(`^the administrator writes the draft page "([^"]*)" under "([^"]*)"$`, p.writesChildPage)
	sc.When(`^the administrator adds the type "([^"]*)" labeled "([^"]*)" and "([^"]*)" under "([^"]*)" that nests$`,
		p.addsNestingType)
	sc.When(`^the administrator changes the slug of "([^"]*)" to "([^"]*)"$`, func(title, slug string) error {
		return p.patch(title, title, map[string]any{"slug": slug})
	})
	sc.When(`^the administrator publishes "([^"]*)"$`, func(title string) error {
		return p.patch(title, title, map[string]any{"status": "published"})
	})
	sc.When(`^the administrator renames "([^"]*)" to "([^"]*)"$`, func(title, renamed string) error {
		return p.patch(title, renamed, map[string]any{"title": renamed})
	})
	sc.When(`^the administrator sets "([^"]*)" back to draft$`, func(title string) error {
		return p.patch(title, title, map[string]any{"status": "draft"})
	})
	sc.When(`^the administrator moves "([^"]*)" to the trash$`, func(title string) error {
		return p.writeHeld(http.MethodDelete, title, "", http.StatusOK)
	})
	sc.When(`^the administrator restores "([^"]*)"$`, func(title string) error {
		return p.writeHeld(http.MethodPost, title, "/restore", http.StatusOK)
	})
	sc.When(`^the administrator deletes "([^"]*)" for good$`, func(title string) error {
		return p.writeHeld(http.MethodDelete, title, "?force=true", http.StatusNoContent)
	})
	sc.When(`^the administrator empties the trash of posts$`, func() error {
		_, err := p.send(http.MethodDelete, "/api/content/trash?type=post", "", http.StatusOK)
		return err
	})
	sc.Then(`^"([^"]*)" heard these changes in order:$`, p.heardInOrder)
	sc.Then(`^"([^"]*)" heard no change$`, p.heardNothing)
	sc.Then(`^the post "([^"]*)" is (published)$`, p.postIs)
}

// hearsChanges has the plugin id hear content changes and list the hook in its manifest.
func (p *pausingScenario) hearsChanges(id string) error {
	p.plugins[id].listens = true
	return p.list(id, sdk.ContentChangedHook)
}

// hearingGoesWrong has the plugin id fail or panic with text each time it hears a change.
func (p *pausingScenario) hearingGoesWrong(id, how, text string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if how == "panic" {
		p.plugins[id].hearPanic = text
		return
	}
	p.plugins[id].hearErr = text
}

// addsNestingType has the administrator add the type key with its labels under the route word, nesting its items.
func (p *pausingScenario) addsNestingType(key, singular, plural, routeWord string) error {
	body, err := json.Marshal(map[string]string{
		"key": key, "singular_label": singular, "plural_label": plural, "route_word": routeWord,
	})
	if err != nil {
		return err
	}
	if _, err := p.send(http.MethodPost, "/api/types", string(body), http.StatusCreated); err != nil {
		return err
	}
	_, err = p.send(http.MethodPatch, "/api/types/"+key, `{"hierarchical":true}`, http.StatusOK)
	return err
}

// writesChildPage has the administrator write a draft page titled title under the page titled parent.
func (p *pausingScenario) writesChildPage(title, parent string) error {
	held, err := p.held(parent)
	if err != nil {
		return err
	}
	return p.writesDraft(title, map[string]any{"type": "page", "parent_id": held.id})
}

// writesDraft has the administrator write a draft titled title with the fields asked.
func (p *pausingScenario) writesDraft(title string, asked map[string]any) error {
	asked["title"] = title
	body, err := json.Marshal(asked)
	if err != nil {
		return err
	}
	written, err := p.send(http.MethodPost, "/api/content", string(body), http.StatusCreated)
	if err != nil {
		return err
	}
	return p.keep(title, written)
}

// patch has the administrator apply change to the post titled title, keeping it under kept.
func (p *pausingScenario) patch(title, kept string, change map[string]any) error {
	held, err := p.held(title)
	if err != nil {
		return err
	}
	change["updated_at"] = held.updatedAt
	body, err := json.Marshal(change)
	if err != nil {
		return err
	}
	written, err := p.send(http.MethodPatch, "/api/content/"+held.id, string(body), http.StatusOK)
	if err != nil {
		return err
	}
	delete(p.posts, title)
	return p.keep(kept, written)
}

// writeHeld sends method to the address of the post titled title with suffix after it, wanting status.
func (p *pausingScenario) writeHeld(method, title, suffix string, status int) error {
	held, err := p.held(title)
	if err != nil {
		return err
	}
	written, err := p.send(method, "/api/content/"+held.id+suffix, "", status)
	if err != nil || status == http.StatusNoContent {
		return err
	}
	return p.keep(title, written)
}

// held returns the post the administrator wrote under title.
func (p *pausingScenario) held(title string) (heldPost, error) {
	held, ok := p.posts[title]
	if !ok {
		return heldPost{}, fmt.Errorf("the administrator wrote no post titled %q", title)
	}
	return held, nil
}

// keep keeps the post the site answered under title.
func (p *pausingScenario) keep(title string, written []byte) error {
	var post writtenPost
	if err := json.Unmarshal(written, &post); err != nil {
		return fmt.Errorf("reading the written post %s: %w", written, err)
	}
	p.posts[title] = heldPost{id: post.ID, updatedAt: post.UpdatedAt}
	return nil
}

// send sends body to path with method as the administrator, failing unless the site answers status.
func (p *pausingScenario) send(method, path, body string, status int) ([]byte, error) {
	request, err := http.NewRequestWithContext(p.t.Context(), method, p.served.base+path, strings.NewReader(body))
	if err != nil {
		return nil, err
	}
	request.Header.Set("Content-Type", "application/json")
	got, err := read(p.served.client.Do(request))
	if err != nil || got.status != status {
		return nil, fmt.Errorf("%s %s answered %d %s (%v), want %d", method, path, got.status, got.body, err, status)
	}
	return got.body, nil
}

// heardColumns reads each column a table of heard changes may hold from a change.
var heardColumns = map[string]func(sdk.ContentChange) string{
	"change":        func(c sdk.ContentChange) string { return string(c.Kind) },
	"title":         func(c sdk.ContentChange) string { return c.Item.Title },
	"path":          func(c sdk.ContentChange) string { return c.Item.Path },
	"previous path": func(c sdk.ContentChange) string { return c.PreviousPath },
}

// heardBy returns the changes the plugin id heard, in order.
func (p *pausingScenario) heardBy(id string) []sdk.ContentChange {
	p.mu.Lock()
	defer p.mu.Unlock()
	return slices.Clone(p.plugins[id].heard)
}

// heardInOrder fails unless the plugin id heard the changes the table lists, in its order, read by its columns.
func (p *pausingScenario) heardInOrder(id string, table *godog.Table) error {
	reads := make([]func(sdk.ContentChange) string, 0, len(table.Rows[0].Cells))
	for _, cell := range table.Rows[0].Cells {
		read, ok := heardColumns[cell.Value]
		if !ok {
			return fmt.Errorf("a heard change has no column %q", cell.Value)
		}
		reads = append(reads, read)
	}
	want := make([]string, 0, len(table.Rows))
	for _, row := range table.Rows[1:] {
		cells := make([]string, 0, len(row.Cells))
		for _, cell := range row.Cells {
			cells = append(cells, cell.Value)
		}
		want = append(want, strings.Join(cells, " | "))
	}
	heard := p.heardBy(id)
	got := make([]string, 0, len(heard))
	for _, change := range heard {
		cells := make([]string, 0, len(reads))
		for _, read := range reads {
			cells = append(cells, read(change))
		}
		got = append(got, strings.Join(cells, " | "))
	}
	if !slices.Equal(got, want) {
		return fmt.Errorf("%s heard %q, want %q", id, got, want)
	}
	return nil
}

// heardNothing fails unless the plugin id heard no change.
func (p *pausingScenario) heardNothing(id string) error {
	if got := p.heardBy(id); len(got) != 0 {
		return fmt.Errorf("%s heard %q, want no change", id, got)
	}
	return nil
}

// postIs fails unless the post titled title stands in status.
func (p *pausingScenario) postIs(title, status string) error {
	held, err := p.held(title)
	if err != nil {
		return err
	}
	got, err := p.served.get("/api/content/" + held.id)
	var post writtenPost
	if err == nil {
		err = json.Unmarshal(got.body, &post)
	}
	if err != nil || post.Status != status {
		return fmt.Errorf("the post %s answered %d %s (%v), want it %s", title, got.status, got.body, err, status)
	}
	return nil
}

func TestPluginContentChanges(t *testing.T) {
	t.Parallel()

	runOperatorFeature(t, pluginContentChangesFeature, postgresOperators())
}
