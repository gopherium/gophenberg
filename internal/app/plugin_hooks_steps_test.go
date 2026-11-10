// SPDX-License-Identifier: Apache-2.0

package app

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"testing"

	"github.com/cucumber/godog"
	"github.com/gopherium/framework/pluginkit/wire"

	"github.com/gopherium/gophenberg/sdk"
)

// pluginHooksFeature is the feature a site answers to while its plugins add tags to the head of its pages.
const pluginHooksFeature = "../../test/features/features/plugin-hooks.feature"

// hookFailedLine is the warning the site logs for each error of a plugin inside a hook.
const hookFailedLine = "plugin hook failed"

// hookPanickedLine is the error the site logs for each panic of a plugin inside a hook.
const hookPanickedLine = "plugin hook panicked"

// hookOutOfTimeLine is the note the site logs for each plugin still working when a hook's time ran out.
const hookOutOfTimeLine = "plugin hook ran out of time"

// headTagsOf is what a pausing plugin answers when the site asks for the head tags of a page.
type headTagsOf = func(ctx context.Context, page sdk.PageMeta) ([]sdk.HeadTag, error)

// headedPlugin is a pausing plugin that adds the head tags the scenario set.
type headedPlugin struct {
	*pausingPlugin
}

// HeadTags answers what the scenario set the plugin to add to the head of page.
func (h headedPlugin) HeadTags(ctx context.Context, page sdk.PageMeta) ([]sdk.HeadTag, error) {
	return h.head(ctx, page)
}

// quotedValue reads the quoted value at the end of the description of a tag outside the rules.
var quotedValue = regexp.MustCompile(`"([^"]*)"$`)

// bindHooks binds the steps of the scenarios where plugins list hooks and add tags to the head of the site's pages.
func (p *pausingScenario) bindHooks(sc *godog.ScenarioContext) {
	sc.Given(`^"([^"]*)" adds a head tag naming the page$`, p.namesThePage)
	sc.Given(`^"([^"]*)" adds a head tag marking itself$`, p.marksItself)
	sc.Given(`^"([^"]*)" adds a head tag marking itself and lists its hook twice$`, p.marksAndListsTwice)
	sc.Given(`^"([^"]*)" adds (.+) beside a head tag marking itself$`, p.addsOutsideTheRules)
	sc.Given(`^the head tags of "([^"]*)" fail with "([^"]*)"$`, p.headFails)
	sc.Given(`^the head tags of "([^"]*)" panic with "([^"]*)"$`, p.headPanics)
	sc.Given(`^the head tags of "([^"]*)" wait until the hook's time runs out$`, p.headWaits)
	sc.Given(`^"([^"]*)" lists the hook "([^"]*)" with no method for it$`, p.listsWithoutMethod)
	sc.Given(`^"([^"]*)" has a head tags method and lists no hook$`, p.headsUnlisted)
	sc.Then(`^the head of "([^"]*)" holds the meta tag "([^"]*)" saying "([^"]*)" once$`, p.holdsMetaOnce)
	sc.Then(`^the head of "([^"]*)" holds the mark of "([^"]*)"$`, p.holdsMark)
	sc.Then(`^the head of "([^"]*)" holds the mark of "([^"]*)" before the mark of "([^"]*)"$`, p.marksInOrder)
	sc.Then(`^the head of "([^"]*)" holds no tag carrying "([^"]*)"$`, p.holdsNoTagCarrying)
	sc.Then(`^the head of "([^"]*)" links no feed$`, p.linksNoFeed)
	sc.Then(`^the page "([^"]*)" answers (\d+)$`, p.pageAnswers)
	sc.Then(`^the site logged that the hook "([^"]*)" of "([^"]*)" failed with "(.*)"$`, p.loggedHookFailed)
	sc.Then(`^the site logged that the hook "([^"]*)" of "([^"]*)" panicked with "([^"]*)", with its stack$`,
		p.loggedHookPanicked)
	sc.Then(`^the site logged that the hook "([^"]*)" of "([^"]*)" ran out of its time$`, p.loggedHookOutOfTime)
}

// adds has the plugin id add the head tags head answers and list the head tags hook in its manifest.
func (p *pausingScenario) adds(id string, head headTagsOf) error {
	p.plugins[id].head = head
	return p.list(id, sdk.HeadTagsHook)
}

// list has the plugin id list the hooks in its manifest, after the ones it lists already.
func (p *pausingScenario) list(id string, hooks ...string) error {
	p.plugins[id].listed = append(p.plugins[id].listed, hooks...)
	manifests := make([]wire.Manifest, 0, len(p.compiled))
	for _, compiled := range p.compiled {
		listed := p.plugins[compiled.ID].listed
		manifests = append(manifests, wire.Manifest{ID: compiled.ID, Name: compiled.ID, Hooks: listed})
	}
	index, err := json.Marshal(manifests)
	p.index = index
	return err
}

// markOf returns the meta tag the plugin id adds when it marks a page.
func markOf(id string) sdk.HeadTag {
	return sdk.HeadTag{Tag: "meta", Attrs: []sdk.Attr{{Key: "name", Value: id + "-mark"}}}
}

// marking answers the mark of the plugin id on every page.
func marking(id string) headTagsOf {
	return func(context.Context, sdk.PageMeta) ([]sdk.HeadTag, error) {
		return []sdk.HeadTag{markOf(id)}, nil
	}
}

// namesThePage has the plugin id add a meta tag saying the kind and the address of each page.
func (p *pausingScenario) namesThePage(id string) error {
	return p.adds(id, func(_ context.Context, page sdk.PageMeta) ([]sdk.HeadTag, error) {
		return []sdk.HeadTag{{Tag: "meta", Attrs: []sdk.Attr{
			{Key: "name", Value: id + "-page"}, {Key: "content", Value: string(page.Kind) + " " + page.Path},
		}}}, nil
	})
}

// marksItself has the plugin id add its mark to every page.
func (p *pausingScenario) marksItself(id string) error {
	return p.adds(id, marking(id))
}

// marksAndListsTwice has the plugin id add its mark to every page and list the head tags hook twice.
func (p *pausingScenario) marksAndListsTwice(id string) error {
	return errors.Join(p.adds(id, marking(id)), p.list(id, sdk.HeadTagsHook))
}

// addsOutsideTheRules has the plugin id add the tag the description names beside its mark.
func (p *pausingScenario) addsOutsideTheRules(id, description string) error {
	outside, err := tagDescribed(description)
	if err != nil {
		return err
	}
	return p.adds(id, func(context.Context, sdk.PageMeta) ([]sdk.HeadTag, error) {
		return []sdk.HeadTag{outside, markOf(id)}, nil
	})
}

// tagDescribed returns the tag outside the rules a scenario describes.
func tagDescribed(description string) (sdk.HeadTag, error) {
	value := ""
	if quoted := quotedValue.FindStringSubmatch(description); quoted != nil {
		value = quoted[1]
	}
	switch strings.TrimSuffix(description, ` "`+value+`"`) {
	case "a script tag":
		return sdk.HeadTag{Tag: "script", Attrs: []sdk.Attr{{Key: "src", Value: "/app.js"}}}, nil
	case "a link to":
		return sdk.HeadTag{Tag: "link", Attrs: []sdk.Attr{{Key: "rel", Value: "next"}, {Key: "href", Value: value}}}, nil
	case "a meta tag whose attribute is":
		return sdk.HeadTag{Tag: "meta", Attrs: []sdk.Attr{{Key: value, Value: "x"}}}, nil
	}
	return sdk.HeadTag{}, fmt.Errorf("no tag is described as %q", description)
}

// headFails has the plugin id answer text as its error when asked for head tags.
func (p *pausingScenario) headFails(id, text string) error {
	return p.adds(id, func(context.Context, sdk.PageMeta) ([]sdk.HeadTag, error) {
		return nil, errors.New(text)
	})
}

// headPanics has the plugin id panic with text when asked for head tags.
func (p *pausingScenario) headPanics(id, text string) error {
	return p.adds(id, func(context.Context, sdk.PageMeta) ([]sdk.HeadTag, error) {
		panic(text)
	})
}

// headWaits has the plugin id wait for the hook's time to run out when asked for head tags.
func (p *pausingScenario) headWaits(id string) error {
	return p.adds(id, func(ctx context.Context, _ sdk.PageMeta) ([]sdk.HeadTag, error) {
		<-ctx.Done()
		return nil, ctx.Err()
	})
}

// listsWithoutMethod has the plugin id list the hook name with no method that answers it.
func (p *pausingScenario) listsWithoutMethod(id, name string) error {
	return p.list(id, name)
}

// headsUnlisted gives the plugin id a head tags method and leaves the hook out of its manifest.
func (p *pausingScenario) headsUnlisted(id string) {
	p.plugins[id].head = marking(id)
}

// headOf returns the head of the page the site serves at address.
func (p *pausingScenario) headOf(address string) (string, error) {
	got, err := read(p.served.client.Get(p.served.base + address))
	if err != nil {
		return "", fmt.Errorf("the page %s got no answer: %w", address, err)
	}
	before, _, closed := strings.Cut(string(got.body), "</head>")
	_, head, opened := strings.Cut(before, "<head>")
	if !closed || !opened {
		return "", fmt.Errorf("the page %s answered %d %q, want a head", address, got.status, got.body)
	}
	return head, nil
}

// headCarries fails unless the head of the page at address carries text count times.
func (p *pausingScenario) headCarries(address, text string, count int) error {
	head, err := p.headOf(address)
	if err != nil {
		return err
	}
	if got := strings.Count(head, text); got != count {
		return fmt.Errorf("the head of %s carries %q %d times, want %d:\n%s", address, text, got, count, head)
	}
	return nil
}

// holdsMetaOnce fails unless the head of the page at address holds the meta tag called name saying says once.
func (p *pausingScenario) holdsMetaOnce(address, name, says string) error {
	return p.headCarries(address, fmt.Sprintf(`<meta name=%q content=%q>`, name, says), 1)
}

// holdsMark fails unless the head of the page at address holds the mark of the plugin id once.
func (p *pausingScenario) holdsMark(address, id string) error {
	return p.headCarries(address, `<meta name="`+id+`-mark">`, 1)
}

// marksInOrder fails unless the head of the page at address holds the mark of first before the mark of second.
func (p *pausingScenario) marksInOrder(address, first, second string) error {
	head, err := p.headOf(address)
	if err != nil {
		return err
	}
	before := strings.Index(head, `<meta name="`+first+`-mark">`)
	after := strings.Index(head, `<meta name="`+second+`-mark">`)
	if before < 0 || after < before {
		return fmt.Errorf("the head of %s holds the marks at %d and %d, want %s before %s:\n%s", address, before,
			after, first, second, head)
	}
	return nil
}

// holdsNoTagCarrying fails when the head of the page at address carries text.
func (p *pausingScenario) holdsNoTagCarrying(address, text string) error {
	return p.headCarries(address, text, 0)
}

// linksNoFeed fails when the head of the page at address links the feed.
func (p *pausingScenario) linksNoFeed(address string) error {
	return p.headCarries(address, "rss.xml", 0)
}

// pageAnswers fails unless the page at address answers status.
func (p *pausingScenario) pageAnswers(address string, status int) error {
	got, err := read(p.served.client.Get(p.served.base + address))
	if err != nil || got.status != status {
		return fmt.Errorf("the page %s answered %d (%v), want %d", address, got.status, err, status)
	}
	return nil
}

// loggedHookFailed fails unless the site warned that the hook of the plugin id failed with text.
func (p *pausingScenario) loggedHookFailed(hook, id, text string) error {
	return p.loggedLine(fmt.Sprintf("level=WARN msg=%q plugin=%s hook=%s error=%q", hookFailedLine, id, hook,
		"pluginkit: "+id+" "+hook+": "+text))
}

// loggedHookPanicked fails unless the site logged that the hook of the plugin id panicked with text, with the stack.
func (p *pausingScenario) loggedHookPanicked(hook, id, text string) error {
	return p.loggedLine(fmt.Sprintf("level=ERROR msg=%q plugin=%s hook=%s error=%q stack=\"goroutine ",
		hookPanickedLine, id, hook, "pluginkit: "+id+" "+hook+" panicked: "+text))
}

// loggedHookOutOfTime fails unless the site noted that the plugin id was still working when the hook's time ran out.
func (p *pausingScenario) loggedHookOutOfTime(hook, id string) error {
	return p.loggedLine(fmt.Sprintf("level=INFO msg=%q plugin=%s hook=%s error=%q", hookOutOfTimeLine, id, hook,
		"pluginkit: "+id+" "+hook+": context deadline exceeded"))
}

func TestPluginHooks(t *testing.T) {
	t.Parallel()

	runOperatorFeature(t, pluginHooksFeature, postgresOperators())
}
