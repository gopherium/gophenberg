// SPDX-License-Identifier: Apache-2.0

package installer

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"strings"

	"github.com/gopherium/framework/gonsole"

	"github.com/gopherium/gophenberg/internal/version"
)

// Request is what one run of server:install asks for on its command line.
type Request struct {
	Domain            string
	Archive           string
	AdminEmail        string
	AdminName         string
	AdminPasswordFile string
	WebServer         string
	SkipDomainCheck   bool
	DryRun            bool
}

// Install checks machine for server:install as request asks, then prints the steps of a dry run or takes them.
func Install(ctx context.Context, machine Machine, call gonsole.Call, request Request) error {
	unlock, err := lockedFor(machine, installCommand, !request.DryRun)
	if err != nil {
		return err
	}
	defer unlock()
	check, err := checkFor(ctx, machine, call, request)
	if err != nil {
		return err
	}
	if !request.DryRun {
		return install(ctx, call, request, check)
	}
	planned, err := siteOn(machine, check)
	if err != nil {
		return err
	}
	return printSteps(call.Stdout, planned.steps())
}

// checkFor reads the settings and the install record of the run request asks for, then checks machine against them.
func checkFor(ctx context.Context, machine Machine, call gonsole.Call, request Request) (*machineCheck, error) {
	settings, err := settingsOn(machine, call.Env, request)
	if err != nil {
		return nil, err
	}
	record, err := readRecord(machine)
	if err != nil {
		return nil, err
	}
	if err := record.allows(version.Version()); err != nil {
		return nil, err
	}
	if err := allowsWebServer(machine, settings, call.Env); err != nil {
		return nil, err
	}
	check := &machineCheck{
		machine: machine, settings: settings, request: request, record: record, env: call.Env, notes: call.Stderr,
	}
	return check, check.run(ctx)
}

// install opens the release archive and settles the first administrator, then takes every step on the server.
func install(ctx context.Context, call gonsole.Call, request Request, check *machineCheck) error {
	archive, err := check.openArchive(request.Archive)
	if err != nil {
		return err
	}
	defer closeArchive(archive)
	answered, err := check.firstAdmin(ctx, call, request)
	if err != nil {
		return err
	}
	planned, err := siteOn(check.machine, check)
	if err != nil {
		return err
	}
	running, err := installingOn(check, planned, call)
	if err != nil {
		return err
	}
	defer running.close()
	running.admin = answered
	if archive != nil {
		running.archive = archive
	}
	return running.run(ctx, planned.steps())
}

// firstAdmin returns the first administrator the run creates, asked for while the admin step is open, none after.
func (c *machineCheck) firstAdmin(ctx context.Context, call gonsole.Call, request Request) (admin, error) {
	if err := c.settleAdmin(ctx); err != nil || c.record.ended(stepAdmin) {
		return admin{}, err
	}
	answered, err := askAdmin(ctx, c.machine, request, c.typedEmailRule())
	if err != nil {
		return admin{}, err
	}
	request.AdminEmail = answered.email
	c.settings, err = settingsOn(c.machine, call.Env, request)
	return answered, err
}

// settleAdmin ends the admin step a cut-off run started once account:list shows that run made an account under admin.
func (c *machineCheck) settleAdmin(ctx context.Context) error {
	if !c.record.started(stepAdmin) {
		return nil
	}
	listed, err := listedAccounts(ctx, c.machine.Run, c.settings.Root)
	if err != nil {
		return stop("cannot tell whether the run cut off in the admin step made the first administrator: %v", err)
	}
	if slices.ContainsFunc(listed, func(held account) bool { return held.Role == adminRole }) {
		c.record.Steps[stepAdmin] = stepEnded
	}
	return nil
}

// account is one account account:list -json lists, by its email and its role.
type account struct {
	Email string `json:"email"`
	Role  string `json:"role"`
}

// listedAccounts returns the accounts account:list -json lists, started by run as gophenberg in root.
func listedAccounts(
	ctx context.Context, run func(context.Context, Program) (string, error), root string,
) ([]account, error) {
	printed, err := run(ctx, siteCommand(root, "account:list", "-json"))
	if err != nil {
		return nil, withPrinted(err, printed)
	}
	return accountsIn(printed)
}

// accountsIn returns the accounts account:list -json printed, past any line the site logged before them.
func accountsIn(printed string) ([]account, error) {
	start := strings.Index("\n"+printed, "\n{\n")
	if start < 0 {
		return nil, errors.New("account:list -json printed no accounts")
	}
	var listing struct {
		Accounts []account `json:"accounts"`
	}
	if err := json.NewDecoder(strings.NewReader(printed[start:])).Decode(&listing); err != nil {
		return nil, fmt.Errorf("account:list -json printed accounts that cannot be read: %w", err)
	}
	return listing.Accounts, nil
}

// typedEmailRule returns the rule a typed email keeps, the certificate email's too while no setting gives one.
func (c *machineCheck) typedEmailRule() func(string) (string, error) {
	if c.settings.CertificateEmail != "" {
		return checkEmail
	}
	return func(typed string) (string, error) {
		email, err := checkEmail(typed)
		if err != nil {
			return "", err
		}
		if _, err := certificateEmail(email); err != nil {
			return "", fmt.Errorf("the email also becomes %s, which %w", c.env.Key("INSTALL_CERTIFICATE_EMAIL"), err)
		}
		return email, nil
	}
}
