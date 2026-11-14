// SPDX-License-Identifier: Apache-2.0

// Package pluginhooks builds the hooks the site fires over its started plugins and checks the hooks each one lists.
package pluginhooks

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"slices"
	"time"

	"github.com/gopherium/framework/pluginkit"

	"github.com/gopherium/gophenberg/internal/headtag"
	"github.com/gopherium/gophenberg/sdk"
)

// failedLine is the warning the site logs for each error of a plugin inside a hook.
const failedLine = "plugin hook failed"

// panickedLine is the error the site logs for each panic of a plugin inside a hook.
const panickedLine = "plugin hook panicked"

// outOfTimeLine is the note the site logs for each plugin still working when a hook's time ran out.
const outOfTimeLine = "plugin hook ran out of time"

// Settings are the values every hook runs under.
type Settings struct {
	// Budget is the time one firing gives all its plugins together.
	Budget time.Duration
	// SaveBudget is the time one firing after a saved write gives all its plugins together.
	SaveBudget time.Duration
	// TimedFirings is how many firings after an overrun time each plugin.
	TimedFirings int
}

// hook is one hook the site fires: the name a plugin lists, the method it needs and the check that it has it.
type hook struct {
	name       string
	method     string
	implements func(sdk.Plugin) bool
}

// known lists every hook the site fires.
var known = []hook{
	{name: sdk.HeadTagsHook, method: "HeadTags", implements: implements[sdk.HeadContributor]},
	{name: sdk.ContentChangedHook, method: "ContentChanged", implements: implements[sdk.ContentListener]},
}

// implements reports whether plugin implements the interface C.
func implements[C any](plugin sdk.Plugin) bool {
	_, ok := plugin.(C)
	return ok
}

// Check refuses a listed hook the plugin has no method for or nobody defines, one listed twice, a method not listed.
func Check(plugin sdk.Plugin, listed, defined []string) error {
	refused := make([]error, 0, len(listed)+len(known))
	for i := range listed {
		refused = append(refused, checkAt(plugin, listed, i, defined))
	}
	return errors.Join(append(refused, unlisted(plugin, listed)...)...)
}

// checkAt refuses the hook the plugin lists at i when it listed it before, lacks its method or nobody defines it.
func checkAt(plugin sdk.Plugin, listed []string, i int, defined []string) error {
	name := listed[i]
	if slices.Contains(listed[:i], name) {
		return fmt.Errorf("lists %s twice", name)
	}
	return checkListed(plugin, name, defined)
}

// unlisted returns an error for each hook the plugin has the method of but does not list.
func unlisted(plugin sdk.Plugin, listed []string) []error {
	var refused []error
	for _, h := range known {
		if h.implements(plugin) && !slices.Contains(listed, h.name) {
			refused = append(refused, fmt.Errorf("has a %s method but does not list %s", h.method, h.name))
		}
	}
	return refused
}

// checkListed refuses the hook name a plugin lists when the plugin lacks its method or nobody defines it.
func checkListed(plugin sdk.Plugin, name string, defined []string) error {
	for _, h := range known {
		if h.name != name {
			continue
		}
		if h.implements(plugin) {
			return nil
		}
		return fmt.Errorf("lists %s but has no %s method", name, h.method)
	}
	if slices.Contains(defined, name) {
		return nil
	}
	return fmt.Errorf("lists %s, a hook neither the site nor a plugin defines", name)
}

// Set holds the hooks the site fires, each over the started plugins that implement it.
type Set struct {
	head    *pluginkit.Hook[sdk.HeadContributor]
	content *pluginkit.Hook[sdk.ContentListener]
	heads   bool
	listens bool
}

// New returns the hooks over the plugins under settings, logging each failure of a plugin to logger.
func New(plugins []sdk.Plugin, settings Settings, logger *slog.Logger) (*Set, error) {
	report := reporter(logger)
	head, err := pluginkit.NewHook[sdk.HeadContributor](sdk.HeadTagsHook, plugins, pluginkit.HookSettings{
		Budget: settings.Budget, TimedFirings: settings.TimedFirings, Report: report,
	})
	if err != nil {
		return nil, err
	}
	content, err := pluginkit.NewHook[sdk.ContentListener](sdk.ContentChangedHook, plugins, pluginkit.HookSettings{
		Budget: settings.SaveBudget, TimedFirings: settings.TimedFirings, Report: report,
	})
	if err != nil {
		return nil, err
	}
	return &Set{
		head: head, content: content,
		heads: len(head.Stats().Plugins) > 0, listens: len(content.Stats().Plugins) > 0,
	}, nil
}

// Listens reports whether any plugin of the set hears content changes.
func (s *Set) Listens() bool {
	return s != nil && s.listens
}

// ContentChanged tells each plugin that hears content changes about change, in order, each with fields of its own.
func (s *Set) ContentChanged(ctx context.Context, change sdk.ContentChange) {
	if !s.Listens() {
		return
	}
	s.content.Fire(ctx, func(ctx context.Context, plugin sdk.ContentListener) error {
		owned := change
		owned.Item.Fields = cloneFields(change.Item.Fields)
		return plugin.ContentChanged(ctx, owned)
	})
}

// cloneFields returns a deep copy of fields decoded the way encoding/json decodes them.
func cloneFields(fields map[string]any) map[string]any {
	if fields == nil {
		return nil
	}
	cloned := make(map[string]any, len(fields))
	for key, value := range fields {
		cloned[key] = cloneValue(value)
	}
	return cloned
}

// cloneValue returns a deep copy of a decoded value, its objects and lists copied and the rest kept.
func cloneValue(value any) any {
	if object, ok := value.(map[string]any); ok {
		return cloneFields(object)
	}
	list, ok := value.([]any)
	if !ok {
		return value
	}
	cloned := make([]any, len(list))
	for i, item := range list {
		cloned[i] = cloneValue(item)
	}
	return cloned
}

// Heads reports whether any plugin of the set adds head tags.
func (s *Set) Heads() bool {
	return s != nil && s.heads
}

// HeadTags returns the tags each plugin adds to the head of the page, in order, dropping those outside the rules.
func (s *Set) HeadTags(ctx context.Context, page sdk.PageMeta) []sdk.HeadTag {
	if !s.Heads() {
		return nil
	}
	var tags []sdk.HeadTag
	s.head.Fire(ctx, func(ctx context.Context, plugin sdk.HeadContributor) error {
		added, err := plugin.HeadTags(ctx, pageOwnedBy(page))
		if err != nil {
			return err
		}
		kept, dropped := headtag.Keep(added)
		tags = append(tags, kept...)
		return dropped
	})
	return tags
}

// pageOwnedBy returns page holding a copy of its item, so a plugin that changes it changes nothing another plugin sees.
func pageOwnedBy(page sdk.PageMeta) sdk.PageMeta {
	if page.Item == nil {
		return page
	}
	item := *page.Item
	page.Item = &item
	return page
}

// reporter returns the report that logs each failure of a plugin inside a hook to logger.
func reporter(logger *slog.Logger) func(pluginkit.HookFailure) {
	return func(failure pluginkit.HookFailure) {
		if failure.Panicked {
			logger.Error(panickedLine, "plugin", failure.ID, "hook", failure.Hook, "error", failure.Err,
				"stack", failure.Stack)
			return
		}
		if failure.ContextEnded {
			logger.Info(outOfTimeLine, "plugin", failure.ID, "hook", failure.Hook, "error", failure.Err)
			return
		}
		logger.Warn(failedLine, "plugin", failure.ID, "hook", failure.Hook, "error", failure.Err)
	}
}
