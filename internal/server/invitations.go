// SPDX-License-Identifier: Apache-2.0

package server

import (
	"net/url"

	"github.com/gopherium/framework/goncierge"
	"github.com/gopherium/gouncer/authkit"
	"github.com/gopherium/gouncer/authkit/ratelimit"
)

// activatePath is the admin page an activation link opens.
const activatePath = adminPrefix + "/activate"

// activationURLOf returns the address an activation link opens under public, or the path alone without one.
func activationURLOf(public *url.URL) string {
	if public == nil {
		return activatePath
	}
	return public.String() + activatePath
}

// invitationsOf returns the invite, resend and activate routes over the accounts cfg holds.
func invitationsOf(
	cfg Config, roles *goncierge.Registry, auth *authkit.Handlers, admin *authkit.AdminHandlers,
) *authkit.Invitations {
	return authkit.NewInvitations(authkit.InvitationsConfig{
		Invites:       authkit.NewInvites(authkit.InvitesConfig{Store: cfg.Users, InviteTTL: cfg.InviteTTL, Roles: roles}),
		Sessions:      auth,
		Admin:         admin,
		Sender:        cfg.InviteSender,
		ActivationURL: activationURLOf(cfg.PublicURL),
		Limiter:       ratelimit.NewLimiter(ratelimit.Config{}),
		ClientKey:     ratelimit.ClientIP,
	})
}
