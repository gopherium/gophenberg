// SPDX-License-Identifier: Apache-2.0

package app

import (
	"fmt"
	"log/slog"
	"net/url"

	"github.com/gopherium/framework/gonsole"
	"github.com/gopherium/framework/mailkit"
	"github.com/gopherium/framework/mailkit/smtp"
	"github.com/gopherium/gouncer/authkit"

	"github.com/gopherium/gophenberg/internal/mail"
	"github.com/gopherium/gophenberg/internal/publicsite"
)

// highestPort is the largest port a mail relay may listen on.
const highestPort = 65535

// relayOnly lists the settings that mean nothing without a relay host.
var relayOnly = []string{"SMTP_PORT", "SMTP_USERNAME", "SMTP_PASSWORD", "SMTP_FROM", "SMTP_TLS", "MAIL_TEMPLATE_DIR"}

// mailSettings is the relay a site mails its invitations through and the folder its templates may come from.
type mailSettings struct {
	host        string
	port        int
	username    string
	password    string
	from        string
	tls         smtp.TLS
	templateDir string
}

// invitationsFrom reads the relay that mails the invitations and how long an activation link lives into held.
func invitationsFrom(env gonsole.Env, held *runConfig) error {
	relay, err := mailSettingsFrom(env, held.publicURL)
	if err != nil {
		return err
	}
	held.mail = relay
	held.inviteTTL, err = env.Duration("INVITE_TTL", authkit.DefaultInviteTTL)
	return err
}

// mailSettingsFrom reads the relay settings, rejecting a relay without a sender address or a public address.
func mailSettingsFrom(env gonsole.Env, publicURL *url.URL) (mailSettings, error) {
	host := env.Value("SMTP_HOST")
	if host == "" {
		return mailSettings{}, strayRelaySetting(env)
	}
	port, err := env.Count("SMTP_PORT", smtp.DefaultPort, gonsole.AtMost(highestPort))
	if err != nil {
		return mailSettings{}, err
	}
	tlsPolicy, err := gonsole.Parse(env, "SMTP_TLS", smtp.TLSMandatory, parseMailTLS)
	if err != nil {
		return mailSettings{}, err
	}
	from := env.Value("SMTP_FROM")
	if from == "" {
		return mailSettings{}, requiredWithRelay(env, "SMTP_FROM")
	}
	if publicURL == nil {
		return mailSettings{}, requiredWithRelay(env, "PUBLIC_URL")
	}
	return mailSettings{
		host: host, port: port, username: env.Value("SMTP_USERNAME"), password: env.Value("SMTP_PASSWORD"),
		from: from, tls: tlsPolicy, templateDir: env.Value("MAIL_TEMPLATE_DIR"),
	}, nil
}

// strayRelaySetting rejects the first relay setting set while no relay host is.
func strayRelaySetting(env gonsole.Env) error {
	for _, name := range relayOnly {
		if env.Value(name) != "" {
			return fmt.Errorf("%s is set but %s is not", env.Key(name), env.Key("SMTP_HOST"))
		}
	}
	return nil
}

// requiredWithRelay rejects a relay that leaves the setting name empty.
func requiredWithRelay(env gonsole.Env, name string) error {
	return fmt.Errorf("%s is required when %s is set", env.Key(name), env.Key("SMTP_HOST"))
}

// parseMailTLS returns the transport security a relay is held to.
func parseMailTLS(value string) (smtp.TLS, error) {
	switch held := smtp.TLS(value); held {
	case smtp.TLSMandatory, smtp.TLSOpportunistic, smtp.TLSNone:
		return held, nil
	}
	return "", fmt.Errorf("must be mandatory, opportunistic or none, got %q", value)
}

// smtpConfigOf returns the config of the relay, greeting it as the host of public.
func smtpConfigOf(relay mailSettings, public *url.URL) smtp.Config {
	return smtp.Config{
		Host: relay.host, Port: relay.port, Username: relay.username, Password: relay.password,
		From: relay.from, TLS: relay.tls, HELO: public.Hostname(),
	}
}

// inviteSenderFrom returns what mails the invitations through the relay the settings name, none without a relay.
func inviteSenderFrom(settings runConfig, logger *slog.Logger) (authkit.InviteSender, error) {
	if settings.mail.host == "" {
		return nil, nil
	}
	sender, err := smtp.New(smtpConfigOf(settings.mail, settings.publicURL))
	if err != nil {
		return nil, fmt.Errorf("mail relay: %w", err)
	}
	mailer, err := mailerFrom(sender, settings, logger)
	if err != nil {
		return nil, err
	}
	return mailer, nil
}

// logMailRelay logs the relay the invitations go out through, or that their links come back without one.
func logMailRelay(logger *slog.Logger, relay mailSettings) {
	if relay.host == "" {
		logger.Info("no mail relay configured, invitations answer their activation link")
		return
	}
	logger.Info("mail relay configured", "host", relay.host, "port", relay.port,
		"templates", templateSource(relay.templateDir))
}

// mailerFrom returns the invitation mailer over sender, naming the site and reading the templates the settings name.
func mailerFrom(sender mailkit.Sender, settings runConfig, logger *slog.Logger) (*mail.Mailer, error) {
	mailer, err := mail.New(sender, settings.mail.templateDir, publicsite.TitleOf(settings.siteTitle), logger)
	if err != nil {
		return nil, fmt.Errorf("%sMAIL_TEMPLATE_DIR: %w", settingsPrefix, err)
	}
	return mailer, nil
}

// templateSource names where the invitation template is read from.
func templateSource(overrideDir string) string {
	if overrideDir == "" {
		return "built in"
	}
	return overrideDir
}
