// SPDX-License-Identifier: Apache-2.0

package app

import (
	"cmp"
	"errors"
	"fmt"

	"github.com/gopherium/framework/goncierge"
	"github.com/gopherium/framework/gonsole"
	"github.com/gopherium/framework/pluginkit"
	"github.com/gopherium/gouncer/avatar"

	"github.com/gopherium/gophenberg/internal/accountbridge"
	"github.com/gopherium/gophenberg/internal/contentbridge"
	"github.com/gopherium/gophenberg/internal/role"
	"github.com/gopherium/gophenberg/sdk"
)

// errNamesSentOut reports the Gravatar default image that would send letters of each name to Gravatar.
var errNamesSentOut = errors.New("this default sends names to Gravatar, the initials mode draws them on the site")

// compose builds the stores the address picks, every compiled plugin, their roles and their share, starting nothing.
func compose(cfg composeConfig, plugins registrations) (site, error) {
	built, err := openSite(cfg)
	if err != nil {
		return site{}, err
	}
	outcome := pluginkit.RegisterEach(plugins, built.depsFor)
	built.registered, built.failed = outcome.Registered, failuresOf(outcome.Failed)
	if err := built.ready(); err != nil {
		built.close()
		return site{}, errors.Join(built.failed, err)
	}
	built.accounts.Open()
	return built, nil
}

// openSite builds the stores the address picks, the account reader and what each plugin is lent, registering none.
func openSite(cfg composeConfig) (site, error) {
	lane, err := laneSettingsFrom(cfg.getenv)
	if err != nil {
		return site{}, err
	}
	avatars, err := avatarsFrom(cfg.getenv)
	if err != nil {
		return site{}, err
	}
	built, err := openStores(cfg)
	if err != nil {
		return site{}, err
	}
	built.lane, built.laneSettings, built.avatars = &pluginLane{sqlite: onSQLite(cfg.databaseURL)}, lane, avatars
	built.roles = cmp.Or(cfg.roles, goncierge.New())
	built.defaultRole = valueOr(settingsEnv(cfg.getenv), defaultRoleSetting, role.Member)
	built.accounts = accountbridge.New(built.users, accountbridge.Options{
		Avatar: avatarOf(avatars), Timeout: lane.queryTimeout,
	})
	built.deps = pluginDeps(cfg, built, built.accounts)
	return built, nil
}

// failuresOf returns one error naming each failure, nil for none.
func failuresOf(failures []pluginkit.Failure) error {
	errs := make([]error, 0, len(failures))
	for _, failure := range failures {
		errs = append(errs, failure.Err)
	}
	return errors.Join(errs...)
}

// avatarsFrom reads how the site draws avatars, each setting with its WordPress default.
func avatarsFrom(getenv func(string) string) (avatar.Options, error) {
	env := settingsEnv(getenv)
	mode, err := gonsole.Parse(env, "AVATARS", avatar.ModeInitials, avatar.ParseMode)
	if err != nil {
		return avatar.Options{}, err
	}
	fallback, err := gonsole.Parse(env, "AVATAR_DEFAULT", avatar.DefaultMystery, parseAvatarDefault)
	if err != nil {
		return avatar.Options{}, err
	}
	rating, err := gonsole.Parse(env, "AVATAR_RATING", avatar.RatingG, avatar.ParseRating)
	if err != nil {
		return avatar.Options{}, err
	}
	return avatar.Options{Mode: mode, Default: fallback, Rating: rating}, nil
}

// parseAvatarDefault returns the Gravatar default image value names, refusing the one that sends names out.
func parseAvatarDefault(value string) (avatar.Default, error) {
	fallback, err := avatar.ParseDefault(value)
	if err != nil {
		return "", err
	}
	if fallback == avatar.DefaultInitials {
		return "", fmt.Errorf("%w, got %q", errNamesSentOut, value)
	}
	return fallback, nil
}

// avatarOf returns the avatar builder the account reader draws with, under the site's avatar options.
func avatarOf(options avatar.Options) func(name, email string, size int) string {
	return func(name, email string, size int) string {
		return avatar.Address(name, email, size, options)
	}
}

// ready declares the core's and the registered plugins' roles, checks the default role, then opens the plugins' share.
func (s site) ready() error {
	if err := declareRoles(s.roles, role.Core(), s.registered); err != nil {
		return err
	}
	if err := checkDefaultRole(s.roles, s.defaultRole, nil); err != nil {
		return err
	}
	return s.openLane(len(s.registered))
}

// openLane opens the plugins' share of the PostgreSQL pool for count plugins, nothing off PostgreSQL.
func (s site) openLane(count int) error {
	if s.pgHandle == nil {
		return nil
	}
	return s.lane.open(s.pgHandle, count, s.laneSettings)
}

// depsFor returns what the site lends the plugin id, the same for every plugin.
func (s site) depsFor(string) sdk.Deps {
	return s.deps
}

// close releases the PostgreSQL handle and with it the plugins' share, nothing on SQLite or while describing.
func (s site) close() {
	s.release()
}

// pluginDeps returns what the site lends every plugin, a settings reader that is never nil and no address on SQLite.
func pluginDeps(cfg composeConfig, built site, accounts sdk.AccountReader) sdk.Deps {
	getenv := cfg.getenv
	if getenv == nil {
		getenv = func(string) string { return "" }
	}
	databaseURL := cfg.databaseURL
	if onSQLite(databaseURL) {
		databaseURL = ""
	}
	return sdk.Deps{
		DatabaseURL: databaseURL,
		DB:          built.lane,
		Content:     contentbridge.New(built.content, built.registry, built.library, built.settings),
		Accounts:    accounts,
		Getenv:      getenv,
		Env:         sdk.Env{Prefix: settingsPrefix, Getenv: getenv},
	}
}
