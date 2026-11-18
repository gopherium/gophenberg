// SPDX-License-Identifier: Apache-2.0

package app

import (
	"context"
	"fmt"
	"io"

	accounts "github.com/gopherium/framework/gonsole/auth"
	"github.com/gopherium/gouncer"

	"github.com/gopherium/gophenberg/internal/content"
	"github.com/gopherium/gophenberg/internal/media"
	"github.com/gopherium/gophenberg/internal/mediahost"
	"github.com/gopherium/gophenberg/internal/role"
	"github.com/gopherium/gophenberg/internal/seed"
)

// demoStores holds the stores the demo data set is stored through.
type demoStores struct {
	users    gouncer.Store
	types    content.TypeStore
	content  content.Store
	settings mediahost.Settings
	media    media.Store
}

// demoStoresOf returns the stores of built the demo data set is stored through.
func demoStoresOf(built site) demoStores {
	return demoStores{
		users: built.users, types: built.types, content: built.content, settings: built.settings, media: built.library,
	}
}

// seedDemoData stores the demo data set through the stores of a migrated database, opening no store of its own.
func seedDemoData(ctx context.Context, stores demoStores, getenv func(string) string, stdout io.Writer) error {
	depth, err := fieldDepthFrom(getenv)
	if err != nil {
		return err
	}
	if err := accounts.EnsureAccounts(ctx, stores.users, demoAccounts(), stdout); err != nil {
		return err
	}
	if err := seedDemoContent(ctx, stores, depth); err != nil {
		return err
	}
	if err := seedDemoMedia(ctx, getenv, stores); err != nil {
		return err
	}
	_, err = fmt.Fprintln(stdout, "seeded demo data, an account created above signs in with "+seed.AdminPassword+
		", a kept account keeps its own password")
	return err
}

// demoAccounts returns the demo account of each role, the admin first, all under one password.
func demoAccounts() []accounts.Account {
	return []accounts.Account{
		{Email: seed.AdminEmail, Name: seed.AdminName, Password: seed.AdminPassword, Role: role.Admin},
		{Email: seed.EditorEmail, Name: seed.EditorName, Password: seed.AdminPassword, Role: role.Editor},
		{Email: seed.AuthorEmail, Name: seed.AuthorName, Password: seed.AdminPassword, Role: role.Author},
		{Email: seed.MemberEmail, Name: seed.MemberName, Password: seed.AdminPassword, Role: role.Member},
	}
}

// seedDemoContent registers the demo types and stores the content they hold, nesting no deeper than depth.
func seedDemoContent(ctx context.Context, stores demoStores, depth int) error {
	types := registryFrom(depth, stores.types)
	if err := seed.Types(ctx, types); err != nil {
		return err
	}
	if err := seed.Featured(ctx, types); err != nil {
		return err
	}
	if err := seed.Posts(ctx, stores.content, types, stores.users); err != nil {
		return err
	}
	if err := seed.Pages(ctx, stores.content, types, stores.users); err != nil {
		return err
	}
	if err := seed.Categories(ctx, stores.content, types, stores.users); err != nil {
		return err
	}
	if err := seed.Containers(ctx, types); err != nil {
		return err
	}
	if err := seed.Flexible(ctx, types); err != nil {
		return err
	}
	if err := seed.Conditions(ctx, types); err != nil {
		return err
	}
	return seed.Backlinks(ctx, types)
}

// seedDemoMedia stores the demo pictures when a media directory is configured.
func seedDemoMedia(ctx context.Context, getenv func(string) string, stores demoStores) error {
	dir := mediaDirFrom(settingsEnv(getenv))
	if dir == "" {
		return nil
	}
	library := mediahost.New(mediahost.Config{Dir: dir, Settings: stores.settings})
	return seed.Media(ctx, library, stores.media, stores.users)
}
