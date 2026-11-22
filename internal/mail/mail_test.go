// SPDX-License-Identifier: Apache-2.0

package mail_test

import (
	"bytes"
	"errors"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gopherium/framework/mailkit/testkit"

	"github.com/gopherium/gophenberg/internal/mail"
)

// mariaLink is the activation link the tests mail.
const mariaLink = "https://example.com/admin/activate?token=a-link"

// discard is a logger that keeps nothing.
var discard = slog.New(slog.DiscardHandler)

// newMailer returns a Mailer for the site Example Site over sender, preferring overrideDir.
func newMailer(t *testing.T, sender *testkit.Sender, overrideDir string, logger *slog.Logger) *mail.Mailer {
	t.Helper()
	mailer, err := mail.New(sender, overrideDir, "Example Site", logger)
	if err != nil {
		t.Fatalf("New() = %v, want a mailer", err)
	}
	return mailer
}

// overrideWith returns a folder holding invite.tmpl with the given words.
func overrideWith(t *testing.T, words string) string {
	t.Helper()
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "invite.tmpl"), []byte(words), 0o600); err != nil {
		t.Fatalf("writing the override template: %v", err)
	}
	return dir
}

func TestAnInvitationMailsTheSiteTheNameAndTheLinkToTheAddress(t *testing.T) {
	t.Parallel()

	sender := &testkit.Sender{}
	mailer := newMailer(t, sender, "", discard)

	if err := mailer.SendInvite(t.Context(), "maria@example.com", "Maria Perez", mariaLink); err != nil {
		t.Fatalf("SendInvite() = %v, want the mail sent", err)
	}

	if len(sender.Messages) != 1 {
		t.Fatalf("sent %d mails, want 1", len(sender.Messages))
	}
	sent := sender.Messages[0]
	if sent.To != "maria@example.com" || sent.Subject != "You have been invited to Example Site" {
		t.Errorf("sent to %q with the subject %q, want maria@example.com invited to Example Site", sent.To, sent.Subject)
	}
	for _, want := range []string{"Hello Maria Perez,", "Someone invited you to Example Site.", mariaLink} {
		if !strings.Contains(sent.Body, want) {
			t.Errorf("body %q does not hold %q", sent.Body, want)
		}
	}
}

func TestAnOverrideTemplateWinsOverTheBuiltInOne(t *testing.T) {
	t.Parallel()

	sender := &testkit.Sender{}
	mailer := newMailer(t, sender, overrideWith(t, "Welcome to {{.Site}}\n{{.Name}}: {{.Link}}\n"), discard)

	if err := mailer.SendInvite(t.Context(), "maria@example.com", "Maria Perez", mariaLink); err != nil {
		t.Fatalf("SendInvite() = %v, want the mail sent", err)
	}

	if len(sender.Messages) != 1 || sender.Messages[0].Subject != "Welcome to Example Site" {
		t.Fatalf("sent %+v, want one mail with the override's subject", sender.Messages)
	}
	if body := sender.Messages[0].Body; body != "Maria Perez: "+mariaLink+"\n" {
		t.Errorf("body = %q, want the override's words", body)
	}
}

func TestNewRejectsAMissingOverrideFolder(t *testing.T) {
	t.Parallel()

	_, err := mail.New(&testkit.Sender{}, filepath.Join(t.TempDir(), "missing"), "Example Site", discard)

	if err == nil {
		t.Error("New() = nil, want a missing override folder rejected")
	}
}

func TestABrokenOverrideFailsAtTheSend(t *testing.T) {
	t.Parallel()

	sender := &testkit.Sender{}
	mailer := newMailer(t, sender, overrideWith(t, "{{.Site"), discard)

	err := mailer.SendInvite(t.Context(), "maria@example.com", "Maria Perez", mariaLink)

	if err == nil || len(sender.Messages) != 0 {
		t.Errorf("SendInvite() = %v after sending %d mails, want an error and none sent", err, len(sender.Messages))
	}
}

func TestAFailedSendIsReturnedAndLoggedWithItsReason(t *testing.T) {
	t.Parallel()

	rejected := errors.New("relay rejected the message")
	var logged bytes.Buffer
	mailer := newMailer(t, &testkit.Sender{Err: rejected}, "", slog.New(slog.NewTextHandler(&logged, nil)))

	err := mailer.SendInvite(t.Context(), "maria@example.com", "Maria Perez", mariaLink)

	if !errors.Is(err, rejected) {
		t.Errorf("SendInvite() = %v, want the relay's error", err)
	}
	if line := logged.String(); !strings.Contains(line, "invitation mail not delivered") ||
		!strings.Contains(line, "relay rejected the message") {
		t.Errorf("logged %q, want the undelivered invitation with the relay's reason", line)
	}
}

func TestADeliveredInvitationLogsNothing(t *testing.T) {
	t.Parallel()

	var logged bytes.Buffer
	mailer := newMailer(t, &testkit.Sender{}, "", slog.New(slog.NewTextHandler(&logged, nil)))

	if err := mailer.SendInvite(t.Context(), "maria@example.com", "Maria Perez", mariaLink); err != nil {
		t.Fatalf("SendInvite() = %v, want the mail sent", err)
	}

	if logged.Len() != 0 {
		t.Errorf("logged %q, want nothing", logged.String())
	}
}
