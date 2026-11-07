// SPDX-License-Identifier: Apache-2.0

package installer

import (
	"cmp"
	"context"
	"crypto/x509"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"syscall"
	"time"

	"github.com/gopherium/gophenberg/internal/updater"
)

// keptCaddyfilePath is where the first run keeps the Caddyfile the caddy package wrote, before it writes its own.
const keptCaddyfilePath = caddyfilePath + ".before-gophenberg"

// caddyLocalRoot is the root certificate Caddy signs localhost with, under the home folder of the caddy user.
const caddyLocalRoot = ".local/share/caddy/pki/authorities/local/root.crt"

// maxLocalRootBytes is the largest local root the HTTPS check reads, far above the one certificate Caddy writes there.
const maxLocalRootBytes = 1 << 16

// caddyUnit is the unit of Caddy, which its package installs.
const caddyUnit = "caddy.service"

// adminRole is the role the first administrator starts under.
const adminRole = "admin"

// emailTaken is what account:create-admin prints for an email an account already holds.
const emailTaken = "email already taken"

// firewallLine is the line of the summary that names the ports the hosting provider's firewall must let in.
const firewallLine = "The hosting provider's firewall must let TCP 80, TCP 443 and UDP 443 in for visitors to reach " +
	"the site.\n"

// commandsToKnow ends every summary with the commands a person runs next.
const commandsToKnow = `Commands to know:
  sudo gophenberg update:status     shows the build that runs and the update on offer
  sudo gophenberg update:apply      installs the update on offer
  sudo journalctl -u gophenberg     shows what the site logged
  sudo gophenberg server:uninstall  removes Gophenberg and keeps its data
`

// caddyValidateArgs have caddy validate check the Caddyfile and every file it imports.
var caddyValidateArgs = []string{"validate", "--config", caddyfilePath, "--adapter", "caddyfile"}

// unitPrograms are the programs the units step runs once it wrote the units, in order.
var unitPrograms = []Program{
	{Name: "systemctl", Args: []string{"daemon-reload"}}, unitProgram("enable", pieceService),
}

// unitProgram returns the systemctl line that does verb to unit.
func unitProgram(verb, unit string) Program {
	return Program{Name: "systemctl", Args: []string{verb, unit}}
}

// siteCommand returns the command args of the current build under root, run as gophenberg in root.
func siteCommand(root string, args ...string) Program {
	return Program{Name: filepath.Join(root, updater.Current, siteUser), Args: args, User: siteUser, Dir: root}
}

// placed returns the tree written goes into, its name there and its owner ids, the install tree for gophenberg's.
func (r *installing) placed(written file) (*os.Root, string, *owner) {
	if written.owner != siteUser {
		return r.system, inSystemTree(written.path), nil
	}
	return r.tree, strings.TrimPrefix(written.path, r.site.settings.Root+"/"), r.ids
}

// writeSiteEnv writes the site's .env, then has the site check its settings as gophenberg in the install root.
func (r *installing) writeSiteEnv(ctx context.Context, planned step) error {
	err := r.writeAll(planned.files)
	if err == nil {
		err = r.runStep(ctx, siteCommand(r.site.settings.Root, "check"))
	}
	return err
}

// units owns gophenberg.service in the record, writes the units, then reloads systemd and enables the site.
func (r *installing) units(ctx context.Context, planned step) error {
	r.record.own(pieceService)
	err := r.save()
	if err == nil {
		err = r.writeAll(planned.files)
	}
	for _, program := range unitPrograms {
		if err == nil {
			err = r.runStep(ctx, program)
		}
	}
	return err
}

// caddy writes Caddy's files and checks them, writing the Caddyfile only while the record holds no sum of it.
func (r *installing) caddy(ctx context.Context, planned step) error {
	if r.site.caddyfileWritten() {
		return r.laterCaddy(ctx, planned)
	}
	return r.firstCaddy(ctx, planned)
}

// firstCaddy keeps the package's Caddyfile, writes and checks Caddy's files, saves the sum, then restarts Caddy.
func (r *installing) firstCaddy(ctx context.Context, planned step) error {
	err := r.keepPackageCaddyfile()
	if err == nil {
		err = r.checkedCaddy(ctx, planned.files)
	}
	if err == nil {
		r.record.Caddyfile = caddyfileSum(render("Caddyfile", r.site.data()))
		err = r.save()
	}
	if err == nil {
		err = r.runStep(ctx, unitProgram("restart", caddyUnit))
	}
	return err
}

// laterCaddy writes and checks Caddy's files, the person's Caddyfile left alone, then restarts or reloads Caddy.
func (r *installing) laterCaddy(ctx context.Context, planned step) error {
	err := r.checkedCaddy(ctx, planned.files)
	if err == nil {
		err = r.runStep(ctx, unitProgram(r.site.caddyVerb(), caddyUnit))
	}
	return err
}

// keepPackageCaddyfile renames the caddy package's Caddyfile to Caddyfile.before-gophenberg, unless one is kept.
func (r *installing) keepPackageCaddyfile() error {
	kept := inSystemTree(keptCaddyfilePath)
	_, err := r.system.Lstat(kept)
	if errors.Is(err, fs.ErrNotExist) {
		err = r.renamePackageCaddyfile(kept)
	}
	if err == nil || errors.Is(err, fs.ErrNotExist) {
		return nil
	}
	return fmt.Errorf("installer: keep the caddy package's Caddyfile as %s: %w", keptCaddyfilePath, err)
}

// renamePackageCaddyfile renames the Caddyfile to kept in the system tree, unless it imports Gophenberg's site.
func (r *installing) renamePackageCaddyfile(kept string) error {
	held, err := r.system.ReadFile(inSystemTree(caddyfilePath))
	if err != nil || slices.Contains(trimmedLines(string(held)), caddyImport) {
		return err
	}
	return r.system.Rename(inSystemTree(caddyfilePath), kept)
}

// checkedCaddy writes files, then has caddy validate check them as the caddy user, at home where Caddy keeps its data.
func (r *installing) checkedCaddy(ctx context.Context, files []file) error {
	if err := r.writeAll(files); err != nil {
		return err
	}
	home, err := r.caddyHome()
	if err != nil {
		return err
	}
	validate := Program{
		Name: pieceCaddy, Args: caddyValidateArgs, User: pieceCaddy, Env: []string{"HOME=" + home}, Shown: "caddy validate",
	}
	if printed, err := r.runNamed(ctx, validate); err != nil {
		return fmt.Errorf("caddy validate refused Caddy's files, so Caddy runs on with the settings it has: %w",
			withPrinted(err, printed))
	}
	return nil
}

// caddyHome returns the home folder of the caddy user, where Caddy keeps its data and its local root.
func (r *installing) caddyHome() (string, error) {
	caddy, err := r.machine.LookupUser(pieceCaddy)
	if err != nil {
		return "", fmt.Errorf("installer: %w", err)
	}
	return caddy.HomeDir, nil
}

// start starts the site, or restarts it once the record owns its unit, then waits until its pages answer.
func (r *installing) start(ctx context.Context, _ step) error {
	err := r.runStep(ctx, unitProgram("reset-failed", pieceService))
	if err == nil {
		err = r.runStep(ctx, unitProgram(r.site.startVerb(), pieceService))
	}
	if err != nil {
		return err
	}
	return r.waitForSite(ctx)
}

// waitForSite waits within the start wait until the site's pages answer as the start check of an update wants.
func (r *installing) waitForSite(ctx context.Context) error {
	settings := r.site.settings
	err := r.within(ctx, settings.Checks.StartWait, "the site to start", func(ctx context.Context) error {
		return updater.CheckPages(ctx, r.machine.Fetch, settings.Addr.String())
	})
	if err == nil || ctx.Err() != nil {
		return err
	}
	return fmt.Errorf("%s did not pass its start check within %s, the time %s allows, as at the last try %w, so read "+
		"why with sudo journalctl -u gophenberg", pieceService, settings.Checks.StartWait,
		r.site.env.Key("UPDATE_VERIFY_TIMEOUT"), err)
}

// within tries try every poll interval until it passes or wait is over, printing a still-waiting line for what.
func (r *installing) within(
	ctx context.Context, wait time.Duration, what string, try func(context.Context) error,
) error {
	tried := &lastTry{}
	var err error
	r.watched("still waiting for "+what, wait, tried.last, func() { err = r.retried(ctx, wait, tried.kept(try)) })
	return err
}

// retried tries try every poll interval until it passes or wait is over, returning the last try's error or ctx's end.
func (r *installing) retried(ctx context.Context, wait time.Duration, try func(context.Context) error) error {
	waiting, cancel := context.WithTimeout(ctx, wait)
	defer cancel()
	for {
		err := try(waiting)
		if err == nil || ctx.Err() != nil {
			return cmp.Or(ctx.Err(), err)
		}
		select {
		case <-waiting.Done():
			return err
		case <-time.After(r.site.settings.Checks.PollInterval):
		}
	}
}

// createAdmin creates the first administrator with account:create-admin as gophenberg, the password on its input.
func (r *installing) createAdmin(ctx context.Context, _ step) error {
	program := siteCommand(r.site.settings.Root, "account:create-admin", "-email", r.admin.email, "-name",
		r.admin.name, "-role", adminRole)
	program.Stdin = r.admin.password + "\n"
	program.Shown = siteUser + " account:create-admin"
	printed, err := r.runNamed(ctx, program)
	if strings.Contains(printed, emailTaken) {
		return r.takenByAdmin(ctx)
	}
	r.created = err == nil
	return withPrinted(err, printed)
}

// takenByAdmin accepts the email account:create-admin found taken once account:list lists that email under admin.
func (r *installing) takenByAdmin(ctx context.Context) error {
	listed, err := listedAccounts(ctx, r.runNamed, r.site.settings.Root)
	if err != nil || slices.Contains(listed, account{Email: r.admin.email, Role: adminRole}) {
		return err
	}
	return fmt.Errorf("%s already belongs to an account that is not an administrator, so run sudo gophenberg "+
		"server:install again and give the first administrator another email", r.admin.email)
}

// https asks for the admin page over HTTPS within its wait, warning and leaving the step open when it never answers.
func (r *installing) https(ctx context.Context, _ step) error {
	settings := r.site.settings
	address := "https://" + settings.Domain + "/admin/"
	err := r.within(ctx, settings.Checks.HTTPSWait, address, func(ctx context.Context) error {
		return r.askHTTPS(ctx, address)
	})
	if err == nil || ctx.Err() != nil {
		return err
	}
	_, warnErr := fmt.Fprintf(r.call.Stderr, "warning: %s did not answer 200 within %s, the time %s allows, as at the "+
		"last try %v. The site runs, so check that the hosting provider's firewall lets TCP 80, TCP 443 and UDP 443 "+
		"in, then read sudo journalctl -u caddy, and the next sudo gophenberg server:install checks again\n", address,
		settings.Checks.HTTPSWait, r.site.env.Key("INSTALL_HTTPS_WAIT"), err)
	return cmp.Or(warnErr, errLeftOpen)
}

// askHTTPS asks for address over HTTPS once, refusing any answer but 200.
func (r *installing) askHTTPS(ctx context.Context, address string) error {
	roots, err := r.httpsRoots()
	if err != nil {
		return err
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, address, nil)
	var response *http.Response
	if err == nil {
		response, err = r.machine.FetchHTTPS(request, roots)
	}
	if err != nil {
		return fmt.Errorf("it could not be reached: %w", err)
	}
	_ = response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return fmt.Errorf("it answered %d", response.StatusCode)
	}
	return nil
}

// httpsRoots returns the roots the HTTPS check trusts: Caddy's local root for localhost, read again, else the system's.
func (r *installing) httpsRoots() (*x509.CertPool, error) {
	if r.site.settings.Domain != "localhost" {
		return nil, nil
	}
	home, err := r.caddyHome()
	if err != nil {
		return nil, err
	}
	path := filepath.Join(home, caddyLocalRoot)
	held, err := r.machine.localRoot(path)
	if err != nil || held == nil {
		return nil, cmp.Or(err, fmt.Errorf("the local root Caddy signs localhost with was not at %s yet", path))
	}
	roots := x509.NewCertPool()
	if len(held) > maxLocalRootBytes || !roots.AppendCertsFromPEM(held) {
		return nil, fmt.Errorf("%s holds no certificate", path)
	}
	return roots, nil
}

// localRoot reads up to one byte past maxLocalRootBytes at path, nil when absent, refusing a link or a pipe at once.
func (m Machine) localRoot(path string) ([]byte, error) {
	file, err := absentAsNil(os.OpenFile(filepath.Join(m.Root, path), os.O_RDONLY|syscall.O_NOFOLLOW|syscall.O_NONBLOCK,
		0))
	if err != nil || file == nil {
		return nil, err
	}
	defer func() { _ = file.Close() }()
	if err := regularFile(file.Stat()); err != nil {
		return nil, fmt.Errorf("installer: read %s: %w", path, err)
	}
	return io.ReadAll(io.LimitReader(file, maxLocalRootBytes+1))
}

// regularFile refuses what info, or the err of describing it, shows to be a pipe, a device or anything but a file.
func regularFile(info fs.FileInfo, err error) error {
	if err == nil && !info.Mode().IsRegular() {
		return errors.New("it is not a regular file")
	}
	return err
}

// summary prints where the site runs, how to sign in and the commands to know, the step that ends the install record.
func (r *installing) summary(_ context.Context, _ step) error {
	settings := r.site.settings
	current, err := updater.Layout{Root: filepath.Join(r.machine.Root, settings.Root)}.Named(updater.Current)
	if err != nil {
		return err
	}
	_, err = io.WriteString(r.call.Stdout, "\n"+r.signIn(current)+commandsToKnow)
	return err
}

// signIn returns the lines of the summary that say where the build current answers and where to sign in to it.
func (r *installing) signIn(current string) string {
	settings := r.site.settings
	admin := "https://" + settings.Domain + "/admin/"
	if r.created {
		admin += " as " + r.admin.email
	}
	if !settings.withCaddy() {
		return fmt.Sprintf("Gophenberg %s answers on %s, the %s the site listens on.\nOnce your own web server sends %s "+
			"there, sign in at %s.\n", current, settings.Addr, r.site.env.Key("ADDR"), settings.Domain, admin)
	}
	return fmt.Sprintf("Gophenberg %s runs at https://%s. Sign in at %s.\n", current, settings.Domain, admin) +
		firewallLine
}

// printWebServer prints the lines the person's web server needs, the same text the dry run prints.
func (r *installing) printWebServer(_ context.Context, planned step) error {
	_, err := io.WriteString(r.call.Stdout, "\n"+planned.prints)
	return err
}
