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

// longestToast is the longest toast a browser timer holds.
const longestToast = math.MaxInt32 * time.Millisecond

// listSettingsFrom reads what the admin lists read once from the environment.
func listSettingsFrom(env gonsole.Env) (server.ListSettings, error) {
	held, err := listPagingFrom(env)
	if err != nil {
		return server.ListSettings{}, err
	}
	held.ToastDuration, err = env.Duration("TOAST_DURATION", server.DefaultToastDuration,
		gonsole.WholeMilliseconds(), gonsole.AtMost(int64(longestToast)))
	if err != nil {
		return server.ListSettings{}, err
	}
	held.ToastNameLength, err = env.Count("TOAST_NAME_LENGTH", server.DefaultToastNameLength)
	if err != nil {
		return server.ListSettings{}, err
	}
	held.FormatLocale, err = locale.Tag(env, "FORMAT_LOCALE", server.DefaultFormatLocale)
	if err != nil {
		return server.ListSettings{}, err
	}
	return held, nil
}

// listPagingFrom reads the page sizes a list offers, the one it opens at and the most one page carries.
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
	size, err := env.Count("LIST_PAGE_SIZE", server.DefaultListPageSize)
	if err != nil {
		return server.ListSettings{}, err
	}
	if !slices.Contains(sizes, size) {
		return server.ListSettings{}, fmt.Errorf("%s: must be one of %s %v, got %d",
			env.Key("LIST_PAGE_SIZE"), env.Key("LIST_PAGE_SIZES"), sizes, size)
	}
	return server.ListSettings{PageSizes: sizes, PageSize: size, PageCap: ceiling}, nil
}
