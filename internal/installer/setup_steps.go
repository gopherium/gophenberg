// SPDX-License-Identifier: Apache-2.0

package installer

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os/user"
	"slices"
	"strconv"
	"strings"
)

// siteUser is the OS user that runs the site, signs in to PostgreSQL and owns the install root.
const siteUser = "gophenberg"

// statusFormat has dpkg-query print the status of each package in three letters, "ii " or "hi " once it is installed.
const statusFormat = `${db:Status-Abbrev}\n`

// aptEnv are the settings every apt and dpkg program gets, so it asks nothing and restarts services on its own.
var aptEnv = []string{"DEBIAN_FRONTEND=noninteractive", "NEEDRESTART_MODE=a"}

// databasePrograms make the role gophenberg that signs in by peer, then its database, sorted by ICU's root collation.
var databasePrograms = []Program{
	{Name: "createuser", User: "postgres", Shown: "createuser " + siteUser, Args: []string{
		"--no-superuser", "--no-createdb", "--no-createrole", siteUser,
	}},
	{Name: "createdb", User: "postgres", Shown: "createdb " + siteUser, Args: []string{
		"--owner=" + siteUser, "--encoding=UTF8", "--template=template0", "--locale-provider=icu", "--icu-locale=und",
		siteUser,
	}},
}

// rootFolder is one folder of the install root, by its name in the root, with its mode.
type rootFolder struct {
	name string
	mode fs.FileMode
}

// rootFolders are the folders of the install root, each with its mode, all owned by gophenberg.
var rootFolders = []rootFolder{
	{"versions", 0o755}, {"downloads", 0o755}, {"state", 0o700}, {"backups", 0o700}, {"media", 0o750},
	{"themes", 0o750},
}

// installRootDoes says what the install-root step does, naming any missing folder above the root it makes as root's.
func (s site) installRootDoes() string {
	does := "make the install root " + s.settings.Root + " and its folders, owned by gophenberg"
	if len(s.missing) == 0 {
		return does
	}
	return does + ", after the missing " + joined(s.missing) + " above it, owned by root with mode 0755"
}

// writeOnly writes the files of planned, the only work of its step.
func (r *installing) writeOnly(_ context.Context, planned step) error {
	return r.writeAll(planned.files)
}

// writeAll writes each of files, root's own through the system tree and gophenberg's through the install tree.
func (r *installing) writeAll(files []file) error {
	for _, written := range files {
		tree, name, ids := r.placed(written)
		if err := writeFile(tree, name, written, ids); err != nil {
			return err
		}
	}
	return nil
}

// packages writes the apt files of planned, then installs the packages of the run unless dpkg has them installed.
func (r *installing) packages(ctx context.Context, planned step) error {
	if err := r.repairPackages(ctx, planned.id); err != nil {
		return err
	}
	if err := r.writeAll(planned.files); err != nil {
		return err
	}
	names := r.packageNames()
	if r.installed(ctx, names) {
		return nil
	}
	return r.aptInstall(ctx, names)
}

// repairPackages has dpkg set up what a run cut off in the step called id left half set up.
func (r *installing) repairPackages(ctx context.Context, id string) error {
	if !r.found.started(id) {
		return nil
	}
	return r.runStep(ctx, aptProgram("dpkg", "--configure", "-a"))
}

// packageNames returns the packages the run installs: PostgreSQL, and Caddy unless the person keeps a web server.
func (r *installing) packageNames() []string {
	if !r.site.settings.withCaddy() {
		return []string{piecePostgreSQL}
	}
	return []string{piecePostgreSQL, pieceCaddy}
}

// installed reports whether dpkg has every one of names installed, held or not, one it cannot find counted as missing.
func (r *installing) installed(ctx context.Context, names []string) bool {
	query := aptProgram("dpkg-query", append([]string{"-W", "-f", statusFormat}, names...)...)
	printed, err := r.runNamed(ctx, query.shownAs("dpkg-query -W "+strings.Join(names, " ")))
	if err != nil {
		return false
	}
	statuses := strings.Split(strings.TrimSuffix(printed, "\n"), "\n")
	return len(statuses) == len(names) && !slices.ContainsFunc(statuses, notInstalled)
}

// notInstalled reports whether status, as dpkg abbreviates it, shows a package missing or installed with an error.
func notInstalled(status string) bool {
	return len(status) != 3 || status[1:] != "i "
}

// aptInstall saves names as the record's own, then installs them with apt-get, waiting on the apt lock as set.
func (r *installing) aptInstall(ctx context.Context, names []string) error {
	for _, name := range names {
		r.record.own(name)
	}
	if err := r.save(); err != nil {
		return err
	}
	lock := "DPkg::Lock::Timeout=" + strconv.FormatInt(inSeconds(r.site.settings.Checks.AptLockWait), 10)
	if err := r.runStep(ctx, aptProgram("apt-get", "-o", lock, "update").shownAs("apt-get update")); err != nil {
		return err
	}
	install := aptProgram("apt-get", append([]string{"-o", lock, "install", "-y", "--no-install-recommends"}, names...)...)
	return r.runStep(ctx, install.shownAs("apt-get install "+strings.Join(names, " ")))
}

// aptProgram returns the apt or dpkg program called name with args and the settings every such program gets.
func aptProgram(name string, args ...string) Program {
	return Program{Name: name, Args: args, Env: aptEnv}
}

// runStep runs program and returns its error with what it printed, so the person sees why the step stopped.
func (r *installing) runStep(ctx context.Context, program Program) error {
	printed, err := r.runNamed(ctx, program)
	return withPrinted(err, printed)
}

// withPrinted returns err, the error of a program, followed by what the program printed when it printed anything.
func withPrinted(err error, printed string) error {
	if err == nil || strings.TrimSpace(printed) == "" {
		return err
	}
	return fmt.Errorf("%w, after it printed:\n%s", err, strings.TrimRight(printed, "\n"))
}

// user makes the OS user gophenberg unless it is there, then keeps its ids for everything it owns.
func (r *installing) user(ctx context.Context, _ step) error {
	found, err := r.lookupSiteUser()
	if errors.As(err, new(user.UnknownUserError)) {
		found, err = r.addSiteUser(ctx)
	}
	if err != nil {
		return err
	}
	r.ids, err = idsOf(found)
	return err
}

// addSiteUser makes the system user gophenberg, at home in the install root and unable to sign in, then looks it up.
func (r *installing) addSiteUser(ctx context.Context) (*user.User, error) {
	err := r.runStep(ctx, Program{Name: "useradd", Shown: "useradd " + siteUser, Args: []string{
		"--system", "--user-group", "--home-dir", r.site.settings.Root, "--no-create-home", "--shell", "/usr/sbin/nologin",
		siteUser,
	}})
	if err != nil {
		return nil, err
	}
	return r.lookupSiteUser()
}

// lookupSiteUser looks up the OS user gophenberg.
func (r *installing) lookupSiteUser() (*user.User, error) {
	found, err := r.machine.LookupUser(siteUser)
	if err != nil {
		return nil, fmt.Errorf("installer: %w", err)
	}
	return found, nil
}

// idsOf returns the ids of the OS user found, refusing ids that are not numbers.
func idsOf(found *user.User) (*owner, error) {
	uid, uidErr := strconv.Atoi(found.Uid)
	gid, gidErr := strconv.Atoi(found.Gid)
	if err := errors.Join(uidErr, gidErr); err != nil {
		return nil, fmt.Errorf("installer: the user %s has ids that are not numbers: %w", found.Username, err)
	}
	return &owner{uid: uid, gid: gid}, nil
}

// installRoot owns the install root in the record, then makes it and its folders and opens it as the install tree.
func (r *installing) installRoot(_ context.Context, _ step) error {
	root := r.site.settings.Root
	r.record.own(root)
	if err := r.save(); err != nil {
		return err
	}
	if err := r.makeRoot(inSystemTree(root)); err != nil {
		return fmt.Errorf("installer: make the install root %s: %w", root, err)
	}
	return nil
}

// makeRoot makes each missing folder above name as root's, then name and its folders as gophenberg's, as the tree.
func (r *installing) makeRoot(name string) error {
	err := makeParents(r.system, name, nil)
	if err == nil {
		err = makeFolder(r.system, name, 0o750, r.ids)
	}
	if err == nil {
		r.tree, err = r.system.OpenRoot(name)
	}
	for _, folder := range rootFolders {
		if err == nil {
			err = makeFolder(r.tree, folder.name, folder.mode, r.ids)
		}
	}
	return err
}

// database makes the role gophenberg and its database as postgres, unless the record says an earlier run made them.
func (r *installing) database(ctx context.Context, planned step) error {
	if r.found.ended(planned.id) {
		return nil
	}
	for _, program := range databasePrograms {
		if err := r.runMade(ctx, program); err != nil {
			return err
		}
	}
	return nil
}

// runMade runs program, taking a failure that says what it makes already exists as made.
func (r *installing) runMade(ctx context.Context, program Program) error {
	printed, err := r.runNamed(ctx, program)
	if strings.Contains(printed, "already exists") {
		return nil
	}
	return withPrinted(err, printed)
}
