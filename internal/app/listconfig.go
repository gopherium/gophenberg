// SPDX-License-Identifier: Apache-2.0

package app

import (
	"fmt"
	"math"
	"slices"
	"time"

	"github.com/gopherium/framework/gonsole"
	"github.com/gopherium/framework/gonsole/locale"

	"github.com/gopherium/gophenberg/internal/server"
)

// fewestPageSizes and mostPageSizes bound how many sizes a list offers, the span its page size menu shows for.
const (
	fewestPageSizes = 2
	mostPageSizes   = 6
)

// largestPageCap is the most items one admin page may carry and still be asked of the database.
const largestPageCap = math.MaxInt32

// longestTimer is the longest wait a browser timer holds.
const longestTimer = math.MaxInt32 * time.Millisecond

// browserTimer bounds a duration a browser timer counts down: whole milliseconds, at most the longest wait it holds.
var browserTimer = []gonsole.Bound{gonsole.WholeMilliseconds(), gonsole.AtMost(int64(longestTimer))}

// listSettingsFrom reads what the admin lists read once from the environment.
func listSettingsFrom(env gonsole.Env) (server.ListSettings, error) {
	held, err := listPagingFrom(env)
	if err != nil {
		return server.ListSettings{}, err
	}
	held.ToastDuration, err = env.Duration("TOAST_DURATION", server.DefaultToastDuration, browserTimer...)
	if err != nil {
		return server.ListSettings{}, err
	}
	held.ToastActionDuration, err = env.Duration("TOAST_ACTION_DURATION", server.DefaultToastActionDuration,
		browserTimer...)
	if err != nil {
		return server.ListSettings{}, err
	}
	held.FormatLocale, err = locale.Tag(env, "FORMAT_LOCALE", server.DefaultFormatLocale)
	if err != nil {
		return server.ListSettings{}, err
	}
	held.DialogWriteTimeout, err = env.Duration("DIALOG_WRITE_TIMEOUT", server.DefaultDialogWriteTimeout,
		browserTimer...)
	if err != nil {
		return server.ListSettings{}, err
	}
	return held, nil
}

// listPagingFrom reads the page sizes a list offers, the ones it and the picker open at, and the most one page carries.
func listPagingFrom(env gonsole.Env) (server.ListSettings, error) {
	ceiling, err := env.Count("LIST_PAGE_CAP", server.DefaultListPageCap, gonsole.AtMost(largestPageCap))
	if err != nil {
		return server.ListSettings{}, err
	}
	sizes, err := env.Counts("LIST_PAGE_SIZES", server.DefaultListPageSizes(),
		gonsole.Entries(fewestPageSizes, mostPageSizes), gonsole.AtMost(int64(ceiling)))
	if err != nil {
		return server.ListSettings{}, fmt.Errorf("%w (the Items per page menu shows only for %d to %d sizes, "+
			"and no size may stand past %s %d)", err, fewestPageSizes, mostPageSizes, env.Key("LIST_PAGE_CAP"), ceiling)
	}
	if largest := slices.Max(sizes); largest > ceiling {
		return server.ListSettings{}, fmt.Errorf("%s: must stand at or above the largest page size %d, got %d",
			env.Key("LIST_PAGE_CAP"), largest, ceiling)
	}
	size, err := offeredSize(env, "LIST_PAGE_SIZE", sizes, server.DefaultListPageSize)
	if err != nil {
		return server.ListSettings{}, err
	}
	picker, err := offeredSize(env, "PICKER_PAGE_SIZE", sizes, server.DefaultPickerPageSizeAmong(sizes, size))
	if err != nil {
		return server.ListSettings{}, err
	}
	return server.ListSettings{PageSizes: sizes, PageSize: size, PickerPageSize: picker, PageCap: ceiling}, nil
}

// offeredSize returns the named page size, the fallback when it is empty, an error when the sizes do not offer it.
func offeredSize(env gonsole.Env, name string, sizes []int, fallback int) (int, error) {
	size, err := env.Count(name, fallback)
	if err != nil {
		return 0, err
	}
	if !slices.Contains(sizes, size) {
		return 0, fmt.Errorf("%s: must be one of %s %v, got %d", env.Key(name), env.Key("LIST_PAGE_SIZES"), sizes, size)
	}
	return size, nil
}
