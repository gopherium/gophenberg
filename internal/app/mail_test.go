// SPDX-License-Identifier: Apache-2.0

package app

import (
	"bytes"
	"io"
	"maps"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/gopherium/framework/gonsole"
	"github.com/gopherium/framework/gonsole/testkit"
	"github.com/gopherium/framework/mailkit/smtp"
	mailtestkit "github.com/gopherium/framework/mailkit/testkit"

	"github.com/gopherium/gophenberg/internal/updater"
	"github.com/gopherium/gophenberg/internal/version"
)

// relayEnv returns the settings of a site mailing through a relay, with extra laid over them.
func relayEnv(extra map[string]string) map[string]string {
	env := map[string]string{
		"GOPHENBERG_DATABASE_URL": unreachableDatabaseURL,
		"GOPHENBERG_SMTP_HOST":    "smtp.example.com",
		"GOPHENBERG_SMTP_FROM":    "Example Site <site@example.com>",
		"GOPHENBERG_PUBLIC_URL":   "https://cms.example.com",
	}
	maps.Copy(env, extra)
	return env
}

// relayConfig returns the run config the relay settings with extra laid over them load into.
func relayConfig(t *testing.T, extra map[string]string) runConfig {
	t.Helper()
	settings, err := loadRunConfig(testkit.Getenv(relayEnv(extra)))
	if err != nil {
		t.Fatalf("loadRunConfig() error = %v, want the relay settings read", err)
	}
	return settings
}

// rejectedSetting fails unless err names the setting.
func rejectedSetting(t *testing.T, err error, setting string) {
	t.Helper()
	if err == nil || !strings.Contains(err.Error(), setting) {
		t.Errorf("error = %v, want %s named", err, setting)
	}
}

func TestNoRelayHostLeavesTheSiteWithoutMail(t *testing.T) {
	t.Parallel()

	settings, err := loadRunConfig(testkit.Getenv(map[string]string{"GOPHENBERG_DATABASE_URL": unreachableDatabaseURL}))

	if err != nil || settings.mail != (mailSettings{}) {
		t.Errorf("loadRunConfig() = %+v, %v, want no relay", settings.mail, err)
	}
	if settings.inviteTTL != 168*time.Hour {
		t.Errorf("inviteTTL = %v, want the week authkit gives an invitation", settings.inviteTTL)
	}
}

func TestEachRelaySettingWithoutAHostIsRejected(t *testing.T) {
	t.Parallel()

	for _, name := range []string{"SMTP_PORT", "SMTP_USERNAME", "SMTP_PASSWORD", "SMTP_FROM", "SMTP_TLS",
		"MAIL_TEMPLATE_DIR"} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			_, err := loadRunConfig(testkit.Getenv(map[string]string{
				"GOPHENBERG_DATABASE_URL": unreachableDatabaseURL, "GOPHENBERG_" + name: "set",
			}))

			want := "GOPHENBERG_" + name + " is set but GOPHENBERG_SMTP_HOST is not"
			if err == nil || err.Error() != want {
				t.Errorf("loadRunConfig() error = %v, want %q", err, want)
			}
		})
	}
}

func TestEveryRelaySettingIsRead(t *testing.T) {
	t.Parallel()

	settings := relayConfig(t, map[string]string{
		"GOPHENBERG_SMTP_PORT": "2525", "GOPHENBERG_SMTP_USERNAME": "site", "GOPHENBERG_SMTP_PASSWORD": "a relay secret",
		"GOPHENBERG_SMTP_TLS": "opportunistic", "GOPHENBERG_MAIL_TEMPLATE_DIR": "/srv/mail",
		"GOPHENBERG_INVITE_TTL": "48h",
	})

	want := mailSettings{
		host: "smtp.example.com", port: 2525, username: "site", password: "a relay secret",
		from: "Example Site <site@example.com>", tls: smtp.TLSOpportunistic, templateDir: "/srv/mail",
	}
	if settings.mail != want || settings.inviteTTL != 48*time.Hour {
		t.Errorf("read %+v and %v, want %+v and 48h", settings.mail, settings.inviteTTL, want)
	}
}

func TestARelayDefaultsToTheSubmissionPortAndMandatoryTransportSecurity(t *testing.T) {
	t.Parallel()

	settings := relayConfig(t, nil)

	if settings.mail.port != smtp.DefaultPort || settings.mail.tls != smtp.TLSMandatory {
		t.Errorf("port %d and tls %q, want %d and mandatory", settings.mail.port, settings.mail.tls, smtp.DefaultPort)
	}
}

func TestARelaySettingOutOfShapeIsRejected(t *testing.T) {
	t.Parallel()

	for setting, value := range map[string]string{
		"GOPHENBERG_SMTP_PORT":  "65536",
		"GOPHENBERG_SMTP_TLS":   "starttls",
		"GOPHENBERG_INVITE_TTL": "soon",
	} {
		t.Run(setting, func(t *testing.T) {
			t.Parallel()

			_, err := loadRunConfig(testkit.Getenv(relayEnv(map[string]string{setting: value})))

			rejectedSetting(t, err, setting+":")
		})
	}
}

func TestARelayNeedsASenderAndAPublicAddress(t *testing.T) {
	t.Parallel()

	for _, setting := range []string{"GOPHENBERG_SMTP_FROM", "GOPHENBERG_PUBLIC_URL"} {
		t.Run(setting, func(t *testing.T) {
			t.Parallel()

			_, err := loadRunConfig(testkit.Getenv(relayEnv(map[string]string{setting: ""})))

			want := setting + " is required when GOPHENBERG_SMTP_HOST is set"
			if err == nil || err.Error() != want {
				t.Errorf("loadRunConfig() error = %v, want %q", err, want)
			}
		})
	}
}

func TestTheRelayIsGreetedAsThePublicHost(t *testing.T) {
	t.Parallel()

	settings := relayConfig(t, nil)

	if held := smtpConfigOf(settings.mail, settings.publicURL); held.HELO != "cms.example.com" {
		t.Errorf("HELO = %q, want the public host", held.HELO)
	}
}

func TestNoRelayAnswersNoSender(t *testing.T) {
	t.Parallel()

	sender, err := inviteSenderFrom(runConfig{}, testLogger(io.Discard))

	if sender != nil || err != nil {
		t.Errorf("inviteSenderFrom() = %v, %v, want no sender", sender, err)
	}
}

func TestARelayAnswersASender(t *testing.T) {
	t.Parallel()

	sender, err := inviteSenderFrom(relayConfig(t, nil), testLogger(io.Discard))

	if sender == nil || err != nil {
		t.Errorf("inviteSenderFrom() = %v, %v, want a sender", sender, err)
	}
}

func TestNoRelayIsLoggedAsLinksComingBack(t *testing.T) {
	t.Parallel()

	var logged bytes.Buffer

	logMailRelay(testLogger(&logged), mailSettings{})

	if !strings.Contains(logged.String(), "no mail relay configured, invitations answer their activation link") {
		t.Errorf("logged %q, want the missing relay said", logged.String())
	}
}

func TestARelayIsLoggedWithItsHostPortAndTemplates(t *testing.T) {
	t.Parallel()

	for folder, want := range map[string]string{"": `templates="built in"`, "/srv/mail": "templates=/srv/mail"} {
		t.Run(want, func(t *testing.T) {
			t.Parallel()

			var logged bytes.Buffer

			logMailRelay(testLogger(&logged), mailSettings{host: "smtp.example.com", port: 587, templateDir: folder})

			for _, line := range []string{"mail relay configured", "host=smtp.example.com", "port=587", want} {
				if !strings.Contains(logged.String(), line) {
					t.Errorf("logged %q, want %q", logged.String(), line)
				}
			}
		})
	}
}

func TestServeLogsTheRelayOnceItCountedTheStart(t *testing.T) {
	t.Parallel()

	result := runServe(t, relayEnv(map[string]string{
		"GOPHENBERG_DATABASE_WAIT": "0", installRootSetting: switchedRoot(t, func(*updater.State) {}),
	}))

	if !strings.Contains(result.Stderr, "mail relay configured") {
		t.Errorf("serve stderr %q, want the relay logged", result.Stderr)
	}
}

func TestTheInvitationNamesTheSiteOrGophenbergWithoutATitle(t *testing.T) {
	t.Parallel()

	for title, want := range map[string]string{"": "Gophenberg", "Example Site": "Example Site"} {
		t.Run(want, func(t *testing.T) {
			t.Parallel()

			outbox := &mailtestkit.Sender{}
			mailer, err := mailerFrom(outbox, runConfig{siteTitle: title}, testLogger(io.Discard))
			if err != nil {
				t.Fatalf("mailerFrom() = %v, want a mailer", err)
			}

			if err := mailer.SendInvite(t.Context(), "maria@example.com", "Maria Perez", "/admin/activate"); err != nil {
				t.Fatalf("SendInvite() = %v, want the mail sent", err)
			}
			if got := outbox.Messages[0].Subject; got != "You have been invited to "+want {
				t.Errorf("subject = %q, want the invitation to %s", got, want)
			}
		})
	}
}

func TestARelayTheSenderCannotUseStopsTheStart(t *testing.T) {
	t.Parallel()

	for name, extra := range map[string]map[string]string{
		"bad from": {"GOPHENBERG_SMTP_FROM": "not an address"},
		"credentials": {
			"GOPHENBERG_SMTP_USERNAME": "site", "GOPHENBERG_SMTP_PASSWORD": "a secret", "GOPHENBERG_SMTP_TLS": "none",
		},
		"templates": {"GOPHENBERG_MAIL_TEMPLATE_DIR": filepath.Join(t.TempDir(), "missing")},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			sender, err := inviteSenderFrom(relayConfig(t, extra), testLogger(io.Discard))

			if sender != nil || err == nil {
				t.Errorf("inviteSenderFrom() = %v, %v, want the relay rejected", sender, err)
			}
		})
	}
}

func TestCheckRejectsASenderAddressTheRelayCannotUse(t *testing.T) {
	t.Parallel()

	env := testkit.Getenv(relayEnv(map[string]string{"GOPHENBERG_SMTP_FROM": "not an address"}))

	got := testkit.Run(t, Program(env, noPlugins), "", "check")

	if got.Code != gonsole.ExitFailed || !strings.Contains(got.Stderr, "from address") {
		t.Errorf("check = %d with stderr %q, want 1 and the sender address rejected", got.Code, got.Stderr)
	}
}

func TestServeRejectsASenderAddressBeforeItCountsTheStart(t *testing.T) {
	t.Parallel()

	root := switchedRoot(t, triedThreeTimes)

	result := runServe(t, relayEnv(map[string]string{
		"GOPHENBERG_SMTP_FROM": "not an address", installRootSetting: root,
	}))

	if result.Code != gonsole.ExitFailed || !strings.Contains(result.Stderr, "from address") {
		t.Errorf("serve = %d with stderr %q, want 1 and the sender address rejected", result.Code, result.Stderr)
	}
	held, err := os.Readlink(filepath.Join(root, updater.Current))
	if held != filepath.Join("versions", version.Version()) {
		t.Errorf("the current link names %q (%v), want the start left uncounted", held, err)
	}
}
