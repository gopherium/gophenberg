// SPDX-License-Identifier: Apache-2.0

// Package mail sends the invitation mail of the site.
package mail

import (
	"context"
	"embed"
	"fmt"
	"log/slog"

	"github.com/gopherium/framework/mailkit"
)

// inviteTemplate is the template file an invitation renders.
const inviteTemplate = "invite.tmpl"

//go:embed invite.tmpl
var defaults embed.FS

// Mailer renders and delivers the invitation mail of one site.
type Mailer struct {
	sender    mailkit.Sender
	templates *mailkit.Templates
	site      string
	logger    *slog.Logger
}

// New returns a Mailer delivering through sender for site, preferring the templates in overrideDir.
func New(sender mailkit.Sender, overrideDir, site string, logger *slog.Logger) (*Mailer, error) {
	templates, err := mailkit.NewTemplates(defaults, overrideDir)
	if err != nil {
		return nil, err
	}
	return &Mailer{sender: sender, templates: templates, site: site, logger: logger}, nil
}

// SendInvite mails link to the invited address to, greeting name.
func (m *Mailer) SendInvite(ctx context.Context, to, name, link string) error {
	if err := m.sendInvite(ctx, to, name, link); err != nil {
		m.logger.ErrorContext(ctx, "invitation mail not delivered", "error", err)
		return err
	}
	return nil
}

// sendInvite renders the invitation for the recipient and delivers it.
func (m *Mailer) sendInvite(ctx context.Context, to, name, link string) error {
	held, err := m.templates.Render(inviteTemplate, struct {
		Name string
		Link string
		Site string
	}{Name: name, Link: link, Site: m.site})
	if err != nil {
		return err
	}
	held.To = to
	if err := m.sender.Send(ctx, held); err != nil {
		return fmt.Errorf("mail: send the invitation: %w", err)
	}
	return nil
}
