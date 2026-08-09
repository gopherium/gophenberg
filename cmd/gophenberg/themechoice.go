// SPDX-License-Identifier: Apache-2.0

package main

import (
	"context"
	"fmt"
)

// activeThemeKey names the stored choice of the serving theme.
const activeThemeKey = "theme.active"

// settingReader reads a stored setting by key.
type settingReader interface {
	Get(ctx context.Context, key string) (string, error)
}

// choice is the theme a boot serves and whether an operator pinned it.
type choice struct {
	name   string
	pinned bool
}

// themeChoice returns the theme to serve, preferring an operator pin over the stored choice.
func themeChoice(ctx context.Context, pinned string, settings settingReader) (choice, error) {
	if pinned != "" {
		return choice{name: pinned, pinned: true}, nil
	}
	stored, err := settings.Get(ctx, activeThemeKey)
	if err != nil {
		return choice{}, fmt.Errorf("reading the active theme: %w", err)
	}
	return choice{name: stored}, nil
}
