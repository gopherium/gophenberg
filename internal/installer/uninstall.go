// SPDX-License-Identifier: Apache-2.0

package installer

import (
	"bytes"
	"cmp"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"maps"
	"os"
	"os/user"
	"path/filepath"
	"slices"
	"strings"
	"syscall"

	"github.com/gopherium/framework/gonsole"

	"github.com/gopherium/gophenberg/internal/updater"
)

// uninstallCommand is the name of the command that removes Gophenberg from a server.
const uninstallCommand = "server:uninstall"

// caddyDropIn is the drop-in the units step writes that gives Caddy the folder of its admin socket.
const caddyDropIn = "/etc/systemd/system/caddy.service.d/gophenberg.conf"

// caddyImport is the line of the Caddyfile a first run writes that has Caddy serve Gophenberg's site.
const caddyImport = "import " + caddySitePath

// databaseLine names the database and the role server:install makes, kept or dropped.
const databaseLine = "the database gophenberg and its role gophenberg"

// siteUserLine names the OS user server:install makes, kept or removed.
const siteUserLine = "the system user gophenberg"

// removedLine is the last line server:uninstall prints once it made every change it lists.
const removedLine = "server:uninstall removed what it lists and kept the rest.\n"

// keptSteps are the steps a run without -delete-data leaves as they were, since what they made stays on the server.
var keptSteps = []string{"server-env", "packages", "user", "database", stepAdmin}

// unitPaths are the site's unit and its rollback unit, as the units step writes them for every web server.
var unitPaths = writtenPaths(site{}.unitFiles())

// repositoryPaths are the files that bring Caddy from its own apt repository, its unattended-upgrades line included.
var repositoryPaths = writtenPaths(site{unattended: true}.aptFiles())

// builtNames are the builds, downloads, links and update state server:uninstall removes, by their names in the root.
var builtNames = []string{
	updater.Layout{}.Build(""), filepath.Dir(updater.Layout{}.Download("")), updater.Current, updater.Previous,
	updater.Layout{}.StateFile(),
}

// caddyLeftovers are the files of Caddy's a run keeps while it keeps the person's Caddyfile, each with why.
var caddyLeftovers = []struct{ path, why string }{
	{caddyDropIn, ", which gives Caddy the folder of the admin socket its Caddyfile names"},
	{keptCaddyfilePath, ", the Caddyfile the caddy package wrote"},
}

// unitNotLoaded ends each line systemctl reset-failed prints for a unit systemd no longer holds.
const unitNotLoaded = " not loaded."

// daemonReload has systemd read its units again once a unit or a drop-in is gone.
var daemonReload = Program{Name: "systemctl", Args: []string{"daemon-reload"}}

// dropPrograms drop the database gophenberg, then its role, as the postgres user.
var dropPrograms = []Program{
	{Name: "dropdb", User: "postgres", Args: []string{siteUser}},
	{Name: "dropuser", User: "postgres", Args: []string{siteUser}},
}

// removing is one run of server:uninstall: what it acts on, what it found and saved, its trees, lists and changes.
type removing struct {
	machine    Machine
	call       gonsole.Call
	settings   Settings
	found      Record
	saved      Record
	deleteData bool
	system     *os.Root
	tree       *os.Root
	held       []byte
	caddyfile  string
	removed    []string
	kept       []string
	changes    []func(context.Context) error
}

// datum is a piece of the site's data, by the line that names it kept and the one that names it removed, and how.
type datum struct {
	kept, removed string
	remove        func(context.Context) error
}

// spot is one file or folder the run removes: the tree it sits in, its name there and its path from the top.
type spot struct {
	tree       *os.Root
	name, path string
}

// listing is one list server:uninstall prints, by its header and its lines.
type listing struct {
	header string
	lines  []string
}

// writtenPaths returns the path of each of files.
func writtenPaths(files []file) []string {
	paths := make([]string, 0, len(files))
	for _, written := range files {
		paths = append(paths, written.path)
	}
	return paths
}

// Uninstall removes what server:install set up on machine and keeps the data, or with deleteData removes that too.
func Uninstall(ctx context.Context, machine Machine, call gonsole.Call, deleteData bool) error {
	unlock, err := lockedFor(machine, uninstallCommand, call.Apply)
	if err != nil {
		return err
	}
	defer unlock()
	u, err := removingOn(machine, call)
	if err != nil {
		return err
	}
	defer u.close()
	u.deleteData = deleteData
	if err := u.plan(); err != nil {
		return err
	}
	if err := u.print(); err != nil || !call.Apply {
		return err
	}
	return u.apply(ctx)
}

// removingOn returns the run of server:uninstall on machine, refusing one that finds no record.
func removingOn(machine Machine, call gonsole.Call) (*removing, error) {
	settings, err := removalSettings(machine, call.Env)
	if err != nil {
		return nil, err
	}
	found, err := readRecord(machine)
	if err != nil {
		return nil, err
	}
	if found.Installer == "" {
		return nil, fmt.Errorf("this server has no install record at %s, so %s has nothing to remove", recordPath,
			uninstallCommand)
	}
	system, err := machine.tree("/")
	if err != nil {
		return nil, err
	}
	return &removing{machine: machine, call: call, settings: settings, found: found, saved: found, system: system}, nil
}

// removalSettings reads what server:uninstall acts on from server.env alone and the defaults, never the environment.
func removalSettings(machine Machine, env gonsole.Env) (Settings, error) {
	file, err := readServerEnv(machine)
	if err != nil {
		return Settings{}, err
	}
	read := gonsole.Env{Prefix: env.Prefix, Getenv: layered(lookup(file), lookup(defaultLines(env, Request{})))}
	root, rootErr := gonsole.Parse(read, "INSTALL_ROOT", "", installRoot)
	domain, domainErr := gonsole.Parse(read, "PUBLIC_URL", "", httpsHost)
	webServer, webServerErr := gonsole.Parse(read, "INSTALL_WEB_SERVER", "", webServerName)
	email, emailErr := gonsole.Parse(read, "INSTALL_CERTIFICATE_EMAIL", "", certificateEmail)
	settings := Settings{Root: root, Domain: domain, WebServer: webServer, CertificateEmail: email}
	return settings, errors.Join(rootErr, domainErr, webServerErr, emailErr)
}

// close closes the system tree and, once the run opened it, the install tree.
func (u *removing) close() {
	_ = u.system.Close()
	if u.tree != nil {
		_ = u.tree.Close()
	}
}

// plan lists what the run removes and keeps, in the order it removes them.
func (u *removing) plan() error {
	u.planService()
	if err := u.planCaddy(); err != nil {
		return err
	}
	u.removesPaths(u.present([]string{caddySitePath})...)
	u.removesEmptied(maintenanceFolder)
	u.planCommand()
	if err := u.planRoot(); err != nil {
		return err
	}
	if err := u.planData(); err != nil {
		return err
	}
	u.planKept()
	return nil
}

// planService lists the site's units, the site stopped and disabled and both cleared before they go, then reloads.
func (u *removing) planService() {
	units := u.present(unitPaths)
	if len(units) == 0 {
		return
	}
	names := make([]string, 0, len(units))
	for _, unit := range units {
		names = append(names, filepath.Base(unit))
	}
	if slices.Contains(names, pieceService) {
		u.removed = append(u.removed, pieceService+", stopped and disabled")
		u.then(u.runs(Program{Name: "systemctl", Args: []string{"disable", "--now", pieceService}}))
	}
	u.then(u.resetFailed(names))
	u.removesPaths(units...)
	u.then(u.runs(daemonReload))
}

// resetFailed returns the change that clears the failed state of units, taking units systemd let go of as cleared.
func (u *removing) resetFailed(units []string) func(context.Context) error {
	program := Program{Name: "systemctl", Args: append([]string{"reset-failed"}, units...)}
	return func(ctx context.Context) error {
		printed, err := u.machine.Run(ctx, program)
		if notLoaded(printed) {
			return nil
		}
		return withPrinted(err, printed)
	}
}

// notLoaded reports whether printed holds a line and each line it holds says systemd holds no such unit.
func notLoaded(printed string) bool {
	lines := slices.DeleteFunc(trimmedLines(printed), func(line string) bool { return line == "" })
	return len(lines) > 0 && !slices.ContainsFunc(lines, func(line string) bool {
		return !strings.HasSuffix(line, unitNotLoaded)
	})
}

// planCaddy lists how the run takes Gophenberg's site out of Caddy and what of Caddy's it keeps, with why.
func (u *removing) planCaddy() error {
	u.caddyfile = u.found.Caddyfile
	if !u.withCaddy() {
		return nil
	}
	held, err := u.machine.read(caddyfilePath)
	if err != nil {
		return err
	}
	u.held = held
	switch {
	case u.putsBack():
		u.putBack()
	case u.held == nil && u.caddyOwed():
		return stop("%s needs %s to take Gophenberg's site out of Caddy, and it is not there", uninstallCommand,
			caddyfilePath)
	case u.restartOwed():
		u.finishPutBack()
	default:
		return u.keepCaddyfile()
	}
	return nil
}

// importsSite reports whether the Caddyfile on the server holds the line that has Caddy serve Gophenberg's site.
func (u *removing) importsSite() bool {
	return slices.Contains(trimmedLines(string(u.held)), caddyImport)
}

// withCaddy reports whether Caddy serves Gophenberg's site: server.env says so, or the site's Caddy file is there.
func (u *removing) withCaddy() bool {
	return u.settings.withCaddy() || u.machine.there(caddySitePath)
}

// putsBack reports whether the package's Caddyfile goes back: it is kept, and the Caddyfile is gone or the installer's.
func (u *removing) putsBack() bool {
	if !u.machine.there(keptCaddyfilePath) {
		return false
	}
	return u.held == nil || u.installersCaddyfile()
}

// installersCaddyfile reports whether the Caddyfile is the one server:install wrote, by the record's sum or its text.
func (u *removing) installersCaddyfile() bool {
	if u.found.Caddyfile == "" {
		return string(u.held) == u.renderedCaddyfile()
	}
	return caddyfileSum(string(u.held)) == u.found.Caddyfile
}

// renderedCaddyfile returns the Caddyfile a first run writes under the settings of the run.
func (u *removing) renderedCaddyfile() string {
	return render("Caddyfile", site{settings: u.settings}.data())
}

// caddyOwed reports whether Caddy still has to let go of Gophenberg's site, by a reload or a restart.
func (u *removing) caddyOwed() bool {
	return u.machine.there(caddySitePath) || u.restartOwed()
}

// restartOwed reports whether Caddy still owes the restart of a put back or a first run that a cut-off left undone.
func (u *removing) restartOwed() bool {
	if u.importsSite() || u.found.Caddyfile != "" {
		return false
	}
	if u.machine.there(keptCaddyfilePath) {
		return false
	}
	return u.found.Steps[stepCaddy] != "" || u.machine.there(caddyDropIn)
}

// putBack lists the Caddyfile going back to the package's once the record shows it, then the restart of Caddy.
func (u *removing) putBack() {
	u.removed = append(u.removed, caddyfilePath+", putting back "+keptCaddyfilePath+", the one the caddy package wrote")
	u.then(u.reopenCaddy)
	u.then(func(context.Context) error {
		if err := u.system.Rename(inSystemTree(keptCaddyfilePath), inSystemTree(caddyfilePath)); err != nil {
			return fmt.Errorf("installer: put %s back as %s: %w", keptCaddyfilePath, caddyfilePath, err)
		}
		return nil
	})
	u.restartCaddy()
}

// finishPutBack lists the restart a cut-off run owes Caddy on the package's Caddyfile, once the record shows it.
func (u *removing) finishPutBack() {
	u.removed = append(u.removed, "Gophenberg's settings in Caddy, restarting Caddy on "+caddyfilePath+", the one "+
		"the caddy package wrote")
	u.then(u.reopenCaddy)
	u.restartCaddy()
}

// restartCaddy lists Caddy's drop-in as removed, then reloads systemd and restarts Caddy on its own admin, no sum kept.
func (u *removing) restartCaddy() {
	u.removesPaths(u.present([]string{caddyDropIn})...)
	u.then(u.runs(daemonReload))
	u.then(u.runs(unitProgram("restart", caddyUnit)))
	u.caddyfile = ""
}

// reopenCaddy saves the record found with no Caddyfile sum and the caddy step started, before Caddy's files change.
func (u *removing) reopenCaddy(context.Context) error {
	reopened := u.found
	reopened.Caddyfile, reopened.Steps = "", map[string]string{}
	maps.Copy(reopened.Steps, u.found.Steps)
	reopened.Steps[stepCaddy] = stepStarted
	return u.leave(reopened)
}

// keepCaddyfile keeps a Caddyfile the person changed with its sum, reloads or restarts Caddy and lists what it keeps.
func (u *removing) keepCaddyfile() error {
	if err := u.keepHeld(); err != nil {
		return err
	}
	if u.importsSite() || u.machine.there(caddySitePath) {
		u.then(u.runs(unitProgram(site{record: u.found}.caddyVerb(), caddyUnit)))
	}
	for _, leftover := range caddyLeftovers {
		if u.machine.there(leftover.path) {
			u.kept = append(u.kept, leftover.path+leftover.why)
		}
	}
	return nil
}

// keepHeld lists a Caddyfile server:install wrote as kept and saves its sum, then lists its import line going out.
func (u *removing) keepHeld() error {
	if u.held == nil || !u.wroteCaddyfile() {
		return nil
	}
	if u.importsSite() && u.caddyfileLinked() {
		return stop("%s is a link, and the line %q must come out of the file it leads to before %s runs again",
			caddyfilePath, caddyImport, uninstallCommand)
	}
	u.caddyfile = cmp.Or(u.found.Caddyfile, caddyfileSum(u.renderedCaddyfile()))
	if u.caddyfile != u.found.Caddyfile {
		u.then(u.saveSum)
	}
	u.takeOutImport()
	return nil
}

// wroteCaddyfile reports whether server:install wrote the Caddyfile, as a sum, the import line or the kept one shows.
func (u *removing) wroteCaddyfile() bool {
	if u.found.Caddyfile != "" || u.importsSite() {
		return true
	}
	return u.machine.there(keptCaddyfilePath)
}

// caddyfileLinked reports whether the Caddyfile is a link.
func (u *removing) caddyfileLinked() bool {
	info, _ := u.machine.lstat(caddyfilePath)
	return info != nil && info.Mode()&fs.ModeSymlink != 0
}

// saveSum saves the record found with the sum of the Caddyfile the run keeps, its steps as found, before Caddy changes.
func (u *removing) saveSum(context.Context) error {
	kept := u.found
	kept.Caddyfile = u.caddyfile
	return u.leave(kept)
}

// takeOutImport lists the Caddyfile as kept and, where it holds the import line, that line going out.
func (u *removing) takeOutImport() {
	asks := "the line " + caddyImport + " in it"
	if u.importsSite() {
		asks = "that line back"
		u.removed = append(u.removed, "the line "+caddyImport+" of "+caddyfilePath)
		u.then(u.rewriteCaddyfile)
	}
	u.kept = append(u.kept, caddyfilePath+", which changed since server:install wrote it, so a reinstall asks for "+asks)
}

// rewriteCaddyfile writes the Caddyfile as it reads now without the import line, with the mode, owner and group it has.
func (u *removing) rewriteCaddyfile(context.Context) error {
	name := inSystemTree(caddyfilePath)
	held, readErr := u.system.ReadFile(name)
	info, statErr := u.system.Stat(name)
	if err := cmp.Or(readErr, statErr); err != nil {
		return fmt.Errorf("installer: %w", err)
	}
	var ids *owner
	if stat, described := info.Sys().(*syscall.Stat_t); described {
		ids = &owner{uid: int(stat.Uid), gid: int(stat.Gid)}
	}
	kept := file{path: caddyfilePath, owner: "root", mode: info.Mode().Perm(), text: withoutImport(string(held))}
	return writeFile(u.system, name, kept, ids)
}

// withoutImport returns text without each of its lines that reads the import line.
func withoutImport(text string) string {
	var kept strings.Builder
	for line := range strings.Lines(text) {
		if strings.TrimSpace(line) != caddyImport {
			kept.WriteString(line)
		}
	}
	return kept.String()
}

// planCommand lists the command on the path when server:install wrote it, else keeps it, then the installer's copy.
func (u *removing) planCommand() {
	info, _ := u.machine.lstat(commandPath)
	switch {
	case info == nil:
	case u.machine.wroteCommand(info):
		u.removesPaths(commandPath)
	default:
		u.kept = append(u.kept, commandPath+", which server:install did not write")
	}
	u.removesEmptied(installerCopy)
}

// wroteCommand reports whether info shows a regular file at commandPath that begins as server:install writes it.
func (m Machine) wroteCommand(info fs.FileInfo) bool {
	if !info.Mode().IsRegular() {
		return false
	}
	head, err := m.head(commandPath, len(commandHeader))
	return err == nil && head == commandHeader
}

// planRoot opens the install root the record owns as a tree and lists its builds, downloads, links and update state.
func (u *removing) planRoot() error {
	u.keepOtherRoots()
	root := u.settings.Root
	if !u.machine.there(root) {
		return nil
	}
	if !u.found.owns(root) {
		u.kept = append(u.kept, root+", since the install record does not show that server:install made it")
		return nil
	}
	tree, err := u.openRoot()
	if err != nil {
		return fmt.Errorf("installer: open the install root %s: %w", root, err)
	}
	u.tree = tree
	return u.planBuilt()
}

// keepOtherRoots lists as kept each install root the record owns that is on the server and server.env does not name.
func (u *removing) keepOtherRoots() {
	for _, piece := range u.found.Installed {
		if u.otherRoot(piece) {
			u.kept = append(u.kept, piece+", an install root server:install made, kept since "+serverEnvPath+
				" does not name it")
		}
	}
}

// otherRoot reports whether piece is an install root on the server that server.env does not name.
func (u *removing) otherRoot(piece string) bool {
	if !filepath.IsAbs(piece) || piece == u.settings.Root {
		return false
	}
	return u.machine.there(piece)
}

// openRoot opens the install root, a real folder, as a tree that no link inside it leads out of.
func (u *removing) openRoot() (*os.Root, error) {
	name := inSystemTree(u.settings.Root)
	if err := realFolder(u.system.Lstat(name)); err != nil {
		return nil, err
	}
	return u.system.OpenRoot(name)
}

// planBuilt lists each build, download, link and update state the install root holds, removed through its tree.
func (u *removing) planBuilt() error {
	for _, name := range builtNames {
		built := spot{tree: u.tree, name: name, path: filepath.Join(u.settings.Root, name)}
		_, err := u.tree.Lstat(name)
		if errors.Is(err, fs.ErrNotExist) {
			continue
		}
		if err != nil {
			return fmt.Errorf("installer: cannot tell whether %s is there: %w", built.path, err)
		}
		u.removed = append(u.removed, built.path)
		u.then(built.remove)
	}
	return nil
}

// planData lists the database, the install root, server.env and the site's user, removed with -delete-data, else kept.
func (u *removing) planData() error {
	held, err := u.data()
	if err != nil {
		return err
	}
	for _, piece := range held {
		if !u.deleteData {
			u.kept = append(u.kept, piece.kept)
			continue
		}
		u.removed = append(u.removed, piece.removed)
		u.then(piece.remove)
	}
	return nil
}

// data returns the pieces of the site's data on the server: its database, install root, server.env and OS user.
func (u *removing) data() ([]datum, error) {
	var held []datum
	if u.found.Steps["database"] != "" {
		held = append(held, datum{kept: databaseLine, removed: databaseLine, remove: u.dropDatabase})
	}
	if u.tree != nil {
		root := "the install root " + u.settings.Root
		held = append(held, datum{
			kept: root + ", with the site's .env, media, themes and backups", removed: root + " and everything in it",
			remove: u.removeRoot,
		})
	}
	held = append(held, u.settingsData()...)
	_, err := u.machine.LookupUser(siteUser)
	if errors.As(err, new(user.UnknownUserError)) {
		return held, nil
	}
	if err != nil {
		return nil, fmt.Errorf("installer: %w", err)
	}
	userdel := u.runs(Program{Name: "userdel", Args: []string{siteUser}})
	return append(held, datum{kept: siteUserLine, removed: siteUserLine, remove: userdel}), nil
}

// settingsData returns server.env as a piece of the data, or the folder a cut-off run emptied of it, when there.
func (u *removing) settingsData() []datum {
	removed, remove := u.emptying(u.systemSpot(serverEnvPath))
	kept := serverEnvPath
	if !u.machine.there(kept) {
		kept = removed
	}
	if !u.machine.there(kept) {
		return nil
	}
	return []datum{{kept: kept, removed: removed, remove: remove}}
}

// dropDatabase drops the database gophenberg and its role as postgres, taking one that does not exist as dropped.
func (u *removing) dropDatabase(ctx context.Context) error {
	for _, program := range dropPrograms {
		printed, err := u.machine.Run(ctx, program)
		if strings.Contains(printed, `"`+siteUser+`" does not exist`) {
			continue
		}
		if err != nil {
			return withPrinted(err, printed)
		}
	}
	return nil
}

// removeRoot removes every entry of the install root through its tree, following no link, then the root itself.
func (u *removing) removeRoot(context.Context) error {
	entries, err := fs.ReadDir(u.tree.FS(), ".")
	for _, entry := range entries {
		if err == nil {
			err = u.tree.RemoveAll(entry.Name())
		}
	}
	if err == nil {
		err = u.system.Remove(inSystemTree(u.settings.Root))
	}
	if err != nil {
		return fmt.Errorf("installer: remove the install root %s: %w", u.settings.Root, err)
	}
	return nil
}

// planKept lists what every run keeps: the install record, the packages and the files of Caddy's own repository.
func (u *removing) planKept() {
	reason := ", so a later install picks up what is kept"
	if u.deleteData {
		reason = ", holding the packages server:install installed, so a later install takes them as its own"
	}
	u.kept = append(u.kept, recordPath+reason)
	if packages := u.ownedPackages(); len(packages) > 0 {
		u.kept = append(u.kept, "the packages server:install installed, which sudo apt-get purge "+
			strings.Join(packages, " ")+" removes")
	}
	if files := u.present(repositoryPaths); len(files) > 0 {
		u.kept = append(u.kept, joined(files)+", which bring Caddy's updates from its own apt repository")
	}
}

// ownedPackages returns the packages the record says server:install installed, in the order it installed them.
func (u *removing) ownedPackages() []string {
	return slices.DeleteFunc(slices.Clone(u.found.Installed), func(piece string) bool {
		return piece != piecePostgreSQL && piece != pieceCaddy
	})
}

// print prints what the run removes, or that nothing is left to remove, then what it keeps.
func (u *removing) print() error {
	var text strings.Builder
	if len(u.removed) == 0 {
		text.WriteString(uninstallCommand + " finds nothing left to remove.\n")
	} else {
		listing{header: uninstallCommand + " removes:", lines: u.removed}.writeTo(&text)
	}
	listing{header: uninstallCommand + " keeps:", lines: u.kept}.writeTo(&text)
	_, err := io.WriteString(u.call.Stdout, text.String())
	return err
}

// writeTo writes the header of l to text, then each of its lines indented by two spaces.
func (l listing) writeTo(text *strings.Builder) {
	text.WriteString(l.header + "\n")
	for _, line := range l.lines {
		text.WriteString("  " + line + "\n")
	}
}

// apply makes each change in the order listed, then leaves the install record and says it is done.
func (u *removing) apply(ctx context.Context) error {
	for _, change := range u.changes {
		if err := change(ctx); err != nil {
			return err
		}
	}
	if err := u.leave(u.left()); err != nil {
		return err
	}
	_, err := io.WriteString(u.call.Stdout, removedLine)
	return err
}

// left returns the install record the run leaves: the packages, the domain, and the pieces a later install reuses.
func (u *removing) left() Record {
	left := Record{
		Installer: u.found.Installer, Domain: u.found.Domain, Steps: map[string]string{}, Installed: u.ownedPackages(),
		Caddyfile: u.caddyfile,
	}
	if !u.deleteData {
		left.Installed = slices.DeleteFunc(slices.Clone(u.found.Installed), func(piece string) bool {
			return piece == pieceService
		})
		for _, id := range keptSteps {
			if mark := u.found.Steps[id]; mark != "" {
				left.Steps[id] = mark
			}
		}
	}
	if left.Caddyfile != "" {
		left.Steps[stepCaddy] = stepEnded
	}
	return left
}

// leave saves record as the install record, unless it holds what the record the run found or last saved holds.
func (u *removing) leave(record Record) error {
	saved, _ := json.Marshal(u.saved)
	leaving, _ := json.Marshal(record)
	if bytes.Equal(saved, leaving) {
		return nil
	}
	saving := installing{system: u.system, record: record}
	if err := saving.save(); err != nil {
		return err
	}
	u.saved = record
	return nil
}

// present returns those of paths that something is at, a link that leads nowhere included.
func (u *removing) present(paths []string) []string {
	return slices.DeleteFunc(slices.Clone(paths), func(path string) bool { return !u.machine.there(path) })
}

// then adds change to what the run makes, in order, with -yes.
func (u *removing) then(change func(context.Context) error) {
	u.changes = append(u.changes, change)
}

// removesPaths lists each of paths as removed, removing it through the system tree with -yes.
func (u *removing) removesPaths(paths ...string) {
	for _, path := range paths {
		u.removed = append(u.removed, path)
		u.then(u.systemSpot(path).remove)
	}
}

// removesEmptied lists path, or its folder once that holds nothing else, as removed when there, removing it with -yes.
func (u *removing) removesEmptied(path string) {
	if listed, remove := u.emptying(u.systemSpot(path)); u.machine.there(listed) {
		u.removed = append(u.removed, listed)
		u.then(remove)
	}
}

// emptying returns what the run lists to remove entry, its folder once that holds nothing else, and the change.
func (u *removing) emptying(entry spot) (string, func(context.Context) error) {
	listed := u.machine.emptied(entry.path)
	if listed == entry.path {
		return listed, entry.remove
	}
	folder := u.systemSpot(listed)
	return listed, func(ctx context.Context) error {
		if err := entry.remove(ctx); err != nil {
			return err
		}
		return folder.removeEmptied()
	}
}

// emptied returns the folder of path when it is a real folder holding nothing but path, else path itself.
func (m Machine) emptied(path string) string {
	folder := filepath.Dir(path)
	if !m.isFolder(folder) {
		return path
	}
	other := func(entry fs.DirEntry) bool { return entry.Name() != filepath.Base(path) }
	entries, err := os.ReadDir(filepath.Join(m.Root, folder))
	if err != nil || slices.ContainsFunc(entries, other) {
		return path
	}
	return folder
}

// systemSpot returns the spot of path, named from the top of the server, in the system tree.
func (u *removing) systemSpot(path string) spot {
	return spot{tree: u.system, name: inSystemTree(path), path: path}
}

// runs returns the change that runs program, stopping with what it printed when it fails.
func (u *removing) runs(program Program) func(context.Context) error {
	return func(ctx context.Context) error {
		printed, err := u.machine.Run(ctx, program)
		return withPrinted(err, printed)
	}
}

// remove removes the spot and all it holds through its tree, following no link.
func (s spot) remove(context.Context) error {
	if err := s.tree.RemoveAll(s.name); err != nil {
		return fmt.Errorf("installer: remove %s: %w", s.path, err)
	}
	return nil
}

// removeEmptied removes the spot, a folder, unless it holds anything or is a link, taking one already gone as removed.
func (s spot) removeEmptied() error {
	info, err := s.tree.Lstat(s.name)
	if err == nil && info.IsDir() {
		err = s.tree.Remove(s.name)
	}
	switch {
	case errors.Is(err, syscall.ENOTEMPTY), errors.Is(err, fs.ErrNotExist):
		return nil
	case err != nil:
		return fmt.Errorf("installer: remove %s: %w", s.path, err)
	}
	return nil
}
