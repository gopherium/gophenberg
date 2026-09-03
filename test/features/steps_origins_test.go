// SPDX-License-Identifier: Apache-2.0

package features_test

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/gopherium/gophenberg/internal/content"
)

// thePluginDeclaredTheGroup stores a group the named plugin owns, placed on one content type.
func thePluginDeclaredTheGroup(ctx context.Context, plugin, title, typeKey string) error {
	w, err := worldOf(ctx)
	if err != nil {
		return err
	}
	var location content.Rules
	if err := json.Unmarshal([]byte(namingType(typeKey)), &location); err != nil {
		return fmt.Errorf("reading the location: %w", err)
	}
	_, err = w.contentTypes.CreateGroup(ctx, content.Group{Title: title, Location: location, Origin: plugin})
	return err
}

// theGroupIsListedAsDeclaredBy asserts the listed group names the plugin that declared it.
func theGroupIsListedAsDeclaredBy(ctx context.Context, title, plugin string) error {
	w, err := worldOf(ctx)
	if err != nil {
		return err
	}
	held, err := groupNamed(w, title)
	if err != nil {
		return err
	}
	if held.Origin != plugin {
		return fmt.Errorf("the group %q names %q as its origin, want %q", title, held.Origin, plugin)
	}
	if held.Active {
		return fmt.Errorf("the group %q is still awake, want it resting", title)
	}
	return nil
}
