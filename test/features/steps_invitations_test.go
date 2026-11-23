// SPDX-License-Identifier: Apache-2.0

package features_test

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"regexp"
	"strings"

	"github.com/cucumber/godog"

	"github.com/gopherium/framework/mailkit/testkit"

	"github.com/gopherium/gophenberg/internal/mail"
	"github.com/gopherium/gophenberg/internal/server"
)

// relayError is the reply the rejecting relay gives for every message.
const relayError = "550 mailbox unavailable"

// mailedLink finds the activation link in a mail body.
var mailedLink = regexp.MustCompile(`\S*/admin/activate\?token=\S+`)

// inviteWorld is one scenario's outbox, the links it handed out and the client of the invited person.
type inviteWorld struct {
	outbox  *testkit.Sender
	links   []string
	invitee *http.Client
}

// into hands the server the scenario's outbox, leaving the server without mail when the scenario sends none.
func (i *inviteWorld) into(cfg *server.Config) error {
	if i.outbox == nil {
		return nil
	}
	mailer, err := mail.New(i.outbox, "", siteTitle, cfg.Logger)
	if err != nil {
		return fmt.Errorf("building the mailer: %w", err)
	}
	cfg.InviteSender = mailer
	return nil
}

// invitationsOf returns the scenario's invitation state, giving the world one when it holds none.
func invitationsOf(ctx context.Context) (*world, *inviteWorld, error) {
	w, err := worldOf(ctx)
	if err != nil {
		return nil, nil, err
	}
	if w.invites == nil {
		w.invites = &inviteWorld{}
	}
	return w, w.invites, nil
}

// aGophenbergMailing starts a server at the public address, mailing its invitations when mails is set.
func aGophenbergMailing(ctx context.Context, address string, mails bool) error {
	w, held, err := invitationsOf(ctx)
	if err != nil {
		return err
	}
	if mails {
		held.outbox = &testkit.Sender{}
	}
	w.publicURL = address
	return w.start(ctx)
}

// aGophenbergAtThePublicAddressThatMails starts a server at the public address that mails its invitations.
func aGophenbergAtThePublicAddressThatMails(ctx context.Context, address string) error {
	return aGophenbergMailing(ctx, address, true)
}

// aGophenbergAtThePublicAddressWithoutMail starts a server at the public address that sends no mail.
func aGophenbergAtThePublicAddressWithoutMail(ctx context.Context, address string) error {
	return aGophenbergMailing(ctx, address, false)
}

// aGophenbergThatMails starts a server with no public address that mails its invitations.
func aGophenbergThatMails(ctx context.Context) error {
	return aGophenbergMailing(ctx, "", true)
}

// aGophenbergWithoutMail starts a server with no public address that sends no mail.
func aGophenbergWithoutMail(ctx context.Context) error {
	return aGophenbergMailing(ctx, "", false)
}

// theRelayRejectsEveryMessage makes the scenario's relay reject whatever it is handed.
func theRelayRejectsEveryMessage(ctx context.Context) error {
	_, held, err := invitationsOf(ctx)
	if err != nil {
		return err
	}
	if held.outbox == nil {
		return errors.New("the scenario sends no mail, so no relay can reject it")
	}
	held.outbox.Err = errors.New(relayError)
	return nil
}

// sendInvitation posts body to path as the signed in account, keeping the link it hands out.
func sendInvitation(ctx context.Context, path, body string) error {
	w, held, err := invitationsOf(ctx)
	if err != nil {
		return err
	}
	mailed := held.mailed()
	if err := w.postJSON(path, body); err != nil {
		return err
	}
	return held.keepLink(w.answer, mailed)
}

// mailed returns how many messages the outbox holds.
func (i *inviteWorld) mailed() int {
	if i.outbox == nil {
		return 0
	}
	return len(i.outbox.Messages)
}

// keepLink keeps the link the answer carries, or the one a message mailed since before held.
func (i *inviteWorld) keepLink(got *answer, before int) error {
	var reply struct {
		ActivationLink string `json:"activation_link"`
	}
	if got.status != http.StatusOK {
		return nil
	}
	if err := got.decode(&reply); err != nil {
		return err
	}
	if reply.ActivationLink != "" {
		i.links = append(i.links, reply.ActivationLink)
		return nil
	}
	if i.mailed() == before {
		return nil
	}
	link := mailedLink.FindString(i.outbox.Messages[i.mailed()-1].Body)
	if link == "" {
		return fmt.Errorf("the mail %q carries no activation link", i.outbox.Messages[i.mailed()-1].Body)
	}
	i.links = append(i.links, link)
	return nil
}

// theAccountInvites asks the API to invite the address under the name and the role.
func theAccountInvites(ctx context.Context, email, name, role string) error {
	return sendInvitation(ctx, "/api/users/invite", fmt.Sprintf(`{"email":%q,"name":%q,"role":%q}`, email, name, role))
}

// theAccountInvited invites the address under the name and the role, failing unless the invitation went out.
func theAccountInvited(ctx context.Context, email, name, role string) error {
	if err := theAccountInvites(ctx, email, name, role); err != nil {
		return err
	}
	w, err := worldOf(ctx)
	if err != nil {
		return err
	}
	return w.expect(http.StatusOK)
}

// theAccountResends asks the API to send the invitation of the address again.
func theAccountResends(ctx context.Context, email string) error {
	return sendInvitation(ctx, "/api/users/invite/resend", fmt.Sprintf(`{"email":%q}`, email))
}

// theInvitationIsAnsweredAsSent fails unless the last answer says the link went out and carries none.
func theInvitationIsAnsweredAsSent(ctx context.Context) error {
	w, err := worldOf(ctx)
	if err != nil {
		return err
	}
	if err := w.expect(http.StatusOK); err != nil {
		return err
	}
	if got := strings.TrimSpace(string(w.answer.body)); got != `{"delivered":true}` {
		return fmt.Errorf("the invitation answered %s, want it sent with no link", got)
	}
	return nil
}

// theInvitationAnswersItsLink fails unless the last answer hands the link back undelivered.
func theInvitationAnswersItsLink(ctx context.Context) error {
	w, err := worldOf(ctx)
	if err != nil {
		return err
	}
	if err := w.expect(http.StatusOK); err != nil {
		return err
	}
	var reply struct {
		Delivered      bool   `json:"delivered"`
		ActivationLink string `json:"activation_link"`
	}
	if err := w.answer.decode(&reply); err != nil {
		return err
	}
	if reply.Delivered || reply.ActivationLink == "" {
		return fmt.Errorf("the invitation answered %s, want its link handed back undelivered", w.answer.body)
	}
	return nil
}

// messagesTo returns the messages the outbox holds for the address.
func (i *inviteWorld) messagesTo(email string) []string {
	var bodies []string
	if i.outbox == nil {
		return nil
	}
	for _, message := range i.outbox.Messages {
		if message.To == email {
			bodies = append(bodies, message.Subject+"\n"+message.Body)
		}
	}
	return bodies
}

// theAddressReceivedAnInvitation fails unless the address received one mail inviting it to the site and greeting name.
func theAddressReceivedAnInvitation(ctx context.Context, email, site, name string) error {
	_, held, err := invitationsOf(ctx)
	if err != nil {
		return err
	}
	received := held.messagesTo(email)
	if len(received) != 1 {
		return fmt.Errorf("%s received %d mails, want 1", email, len(received))
	}
	subject, body, _ := strings.Cut(received[0], "\n")
	if subject != "You have been invited to "+site || !strings.Contains(body, "Hello "+name+",") {
		return fmt.Errorf("%s received %q, want an invitation to %s greeting %s", email, received[0], site, name)
	}
	return nil
}

// theAddressReceivedInvitations fails unless the address received count mails.
func theAddressReceivedInvitations(ctx context.Context, email string, count int) error {
	_, held, err := invitationsOf(ctx)
	if err != nil {
		return err
	}
	if received := len(held.messagesTo(email)); received != count {
		return fmt.Errorf("%s received %d mails, want %d", email, received, count)
	}
	return nil
}

// noInvitationWasMailed fails when the outbox holds any message.
func noInvitationWasMailed(ctx context.Context) error {
	_, held, err := invitationsOf(ctx)
	if err != nil {
		return err
	}
	if sent := held.mailed(); sent != 0 {
		return fmt.Errorf("the relay took %d mails, want none", sent)
	}
	return nil
}

// linkAt returns the link handed out at index, the latest for -1.
func (i *inviteWorld) linkAt(index int) (string, error) {
	if len(i.links) == 0 {
		return "", errors.New("no invitation handed out a link")
	}
	if index < 0 {
		return i.links[len(i.links)-1], nil
	}
	return i.links[index], nil
}

// theLatestLinkOpens fails unless the latest link opens the address with only a token.
func theLatestLinkOpens(ctx context.Context, address string) error {
	_, held, err := invitationsOf(ctx)
	if err != nil {
		return err
	}
	link, err := held.linkAt(-1)
	if err != nil {
		return err
	}
	if !strings.HasPrefix(link, address+"?token=") {
		return fmt.Errorf("the latest link is %q, want it to open %s", link, address)
	}
	return nil
}

// freshClient returns a client of the running server that holds no cookie yet.
func (w *world) freshClient() (*http.Client, error) {
	jar, err := cookiejar.New(nil)
	if err != nil {
		return nil, fmt.Errorf("building the cookie jar: %w", err)
	}
	return &http.Client{Transport: w.client.Transport, Jar: jar}, nil
}

// openLink posts the token of link with the password from a client of its own, recording the answer.
func openLink(ctx context.Context, link, password string) error {
	w, held, err := invitationsOf(ctx)
	if err != nil {
		return err
	}
	parsed, err := url.Parse(link)
	if err != nil {
		return fmt.Errorf("reading the link %q: %w", link, err)
	}
	if held.invitee, err = w.freshClient(); err != nil {
		return err
	}
	body := fmt.Sprintf(`{"token":%q,"password":%q}`, parsed.Query().Get("token"), password)
	request, err := http.NewRequest(http.MethodPost, w.site.URL+"/api/auth/activate", strings.NewReader(body))
	if err != nil {
		return fmt.Errorf("building the activation request: %w", err)
	}
	request.Header.Set("Content-Type", "application/json")
	w.answer, err = w.answerFrom(held.invitee, request)
	return err
}

// theInvitedPersonOpensTheLink opens the link handed out at index, the latest for -1, choosing the password.
func theInvitedPersonOpensTheLink(ctx context.Context, index int, password string) error {
	_, held, err := invitationsOf(ctx)
	if err != nil {
		return err
	}
	link, err := held.linkAt(index)
	if err != nil {
		return err
	}
	return openLink(ctx, link, password)
}

// theInvitedPersonOpensTheLatestLink opens the latest link handed out, choosing the password.
func theInvitedPersonOpensTheLatestLink(ctx context.Context, password string) error {
	return theInvitedPersonOpensTheLink(ctx, -1, password)
}

// theInvitedPersonOpensTheFirstLink opens the first link handed out, choosing the password.
func theInvitedPersonOpensTheFirstLink(ctx context.Context, password string) error {
	return theInvitedPersonOpensTheLink(ctx, 0, password)
}

// theInvitedPersonOpenedTheLatestLink opens the latest link choosing the password, failing unless it signed in.
func theInvitedPersonOpenedTheLatestLink(ctx context.Context, password string) error {
	if err := theInvitedPersonOpensTheLatestLink(ctx, password); err != nil {
		return err
	}
	w, err := worldOf(ctx)
	if err != nil {
		return err
	}
	return w.expect(http.StatusOK)
}

// theInvitedPersonIsSignedInAs fails unless the invited person's client holds a session for the address.
func theInvitedPersonIsSignedInAs(ctx context.Context, email string) error {
	w, held, err := invitationsOf(ctx)
	if err != nil {
		return err
	}
	if err := w.expect(http.StatusOK); err != nil {
		return err
	}
	return sessionOf(w, held.invitee, email)
}

// sessionOf fails unless the client's session belongs to the address.
func sessionOf(w *world, client *http.Client, email string) error {
	request, err := http.NewRequest(http.MethodGet, w.site.URL+"/api/auth/session", nil)
	if err != nil {
		return fmt.Errorf("building the session request: %w", err)
	}
	session, err := w.answerFrom(client, request)
	if err != nil {
		return err
	}
	var identity struct {
		Email string `json:"email"`
	}
	if session.status != http.StatusOK {
		return fmt.Errorf("the session answered %d (%s), want one for %s", session.status, session.body, email)
	}
	if err := session.decode(&identity); err != nil {
		return err
	}
	if identity.Email != email {
		return fmt.Errorf("the session belongs to %q, want %s", identity.Email, email)
	}
	return nil
}

// theAddressSignsInWith fails unless the address signs in with the password from a client of its own.
func theAddressSignsInWith(ctx context.Context, email, password string) error {
	w, err := worldOf(ctx)
	if err != nil {
		return err
	}
	client, err := w.freshClient()
	if err != nil {
		return err
	}
	body := fmt.Sprintf(`{"email":%q,"password":%q}`, email, password)
	request, err := http.NewRequest(http.MethodPost, w.site.URL+"/api/auth/login", strings.NewReader(body))
	if err != nil {
		return fmt.Errorf("building the login request: %w", err)
	}
	request.Header.Set("Content-Type", "application/json")
	signed, err := w.answerFrom(client, request)
	if err != nil {
		return err
	}
	if signed.status != http.StatusOK {
		return fmt.Errorf("signing in %s answered %d (%s), want 200", email, signed.status, signed.body)
	}
	return nil
}

// listedAccount is one account as the user list shows it.
type listedAccount struct {
	Email     string `json:"email"`
	Role      string `json:"role"`
	Confirmed bool   `json:"confirmed"`
}

// listedAs fails unless the user list shows the address under the role, confirmed as given.
func listedAs(ctx context.Context, email, role string, confirmed bool) error {
	w, err := worldOf(ctx)
	if err != nil {
		return err
	}
	listed, err := w.send(http.MethodGet, "/api/users", "", nil)
	if err != nil {
		return err
	}
	var accounts []listedAccount
	if err := listed.decode(&accounts); err != nil {
		return err
	}
	want := listedAccount{Email: email, Role: role, Confirmed: confirmed}
	for _, account := range accounts {
		if account.Email == email && account == want {
			return nil
		}
	}
	return fmt.Errorf("the user list shows %+v, want %+v", accounts, want)
}

// listedNotConfirmed fails unless the user list shows the address under the role, not confirmed yet.
func listedNotConfirmed(ctx context.Context, email, role string) error {
	return listedAs(ctx, email, role, false)
}

// listedConfirmed fails unless the user list shows the address under the role, confirmed.
func listedConfirmed(ctx context.Context, email, role string) error {
	return listedAs(ctx, email, role, true)
}

// theServerLoggedTheUndeliveredInvitation fails unless the log holds the failed invitation with the relay's reason.
func theServerLoggedTheUndeliveredInvitation(ctx context.Context) error {
	w, err := worldOf(ctx)
	if err != nil {
		return err
	}
	for _, line := range strings.Split(w.logs.String(), "\n") {
		if strings.Contains(line, `msg="invitation mail not delivered"`) && strings.Contains(line, relayError) {
			return nil
		}
	}
	return fmt.Errorf("the server logged %q, want the undelivered invitation with %q", w.logs.String(), relayError)
}

// aVisitorOpensADeadLink posts a token no invitation issued from a client of its own.
func aVisitorOpensADeadLink(ctx context.Context) error {
	return openLink(ctx, "/admin/activate?token=a-dead-link", "a password Maria chose")
}

// aVisitorOpenedDeadLinks opens count dead links, failing unless each was rejected as dead.
func aVisitorOpenedDeadLinks(ctx context.Context, count int) error {
	for range count {
		if err := aVisitorOpensADeadLink(ctx); err != nil {
			return err
		}
		if err := theRequestIsRefusedWithTheCode(ctx, "token_invalid"); err != nil {
			return err
		}
	}
	return nil
}

// registerInvitationRequests binds the steps that invite, resend and open a dead link.
func registerInvitationRequests(sc *godog.ScenarioContext) {
	sc.When(`^the (?:administrator|account) invites "([^"]*)" as "([^"]*)" with the role "([^"]*)"$`, theAccountInvites)
	sc.When(`^the (?:administrator|account) resends the invitation to "([^"]*)"$`, theAccountResends)
	sc.When(`^a visitor opens a dead link$`, aVisitorOpensADeadLink)
}

// initializeAccountInvitations binds the steps the invitations feature uses.
func initializeAccountInvitations(sc *godog.ScenarioContext) {
	registerSharedSteps(sc)
	registerInvitationRequests(sc)
	sc.Given(`^a running Gophenberg at the public address "([^"]*)" that mails its invitations$`,
		aGophenbergAtThePublicAddressThatMails)
	sc.Given(`^a running Gophenberg at the public address "([^"]*)" that sends no mail$`,
		aGophenbergAtThePublicAddressWithoutMail)
	sc.Given(`^a running Gophenberg that mails its invitations$`, aGophenbergThatMails)
	sc.Given(`^a running Gophenberg that sends no mail$`, aGophenbergWithoutMail)
	sc.Given(`^the mail relay rejects every message$`, theRelayRejectsEveryMessage)
	sc.Given(`^the plugin "([^"]*)" declaring the role "([^"]*)" with the capability "([^"]*)"$`,
		thePluginDeclaringTheRole)
	sc.Given(`^a signed in "([^"]*)"$`, aSignedInHolder)
	sc.Given(`^the administrator invited "([^"]*)" as "([^"]*)" with the role "([^"]*)"$`, theAccountInvited)
	sc.Given(`^the invited person opened the latest link and chose the password "([^"]*)"$`,
		theInvitedPersonOpenedTheLatestLink)
	sc.Given(`^a visitor opened (\d+) dead links$`, aVisitorOpenedDeadLinks)
	sc.When(`^the invited person opens the latest link and chooses the password "([^"]*)"$`,
		theInvitedPersonOpensTheLatestLink)
	sc.When(`^the invited person opens the first link and chooses the password "([^"]*)"$`,
		theInvitedPersonOpensTheFirstLink)
	sc.Then(`^the invitation is answered as sent$`, theInvitationIsAnsweredAsSent)
	sc.Then(`^the invitation answers its link$`, theInvitationAnswersItsLink)
	sc.Then(`^"([^"]*)" received an invitation to "([^"]*)" greeting "([^"]*)"$`, theAddressReceivedAnInvitation)
	sc.Then(`^"([^"]*)" received (\d+) invitations$`, theAddressReceivedInvitations)
	sc.Then(`^no invitation was mailed$`, noInvitationWasMailed)
	sc.Then(`^the latest link opens "([^"]*)"$`, theLatestLinkOpens)
	sc.Then(`^the invited person is signed in as "([^"]*)"$`, theInvitedPersonIsSignedInAs)
	sc.Then(`^"([^"]*)" signs in with the password "([^"]*)"$`, theAddressSignsInWith)
	sc.Then(`^the user list shows "([^"]*)" as a "([^"]*)" not confirmed yet$`, listedNotConfirmed)
	sc.Then(`^the user list shows "([^"]*)" as a confirmed "([^"]*)"$`, listedConfirmed)
	sc.Then(`^the request is refused with the code "([^"]*)"$`, theRequestIsRefusedWithTheCode)
	sc.Then(`^the server logged the undelivered invitation with the relay's reason$`,
		theServerLoggedTheUndeliveredInvitation)
}
