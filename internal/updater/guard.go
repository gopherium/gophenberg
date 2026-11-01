// SPDX-License-Identifier: Apache-2.0

package updater

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"slices"

	"github.com/gopherium/gophenberg/internal/version"
)

// restoreSteps is the section of the manual that shows how a server install restores a backup of the database.
const restoreSteps = manualSteps + "#restoring-on-a-server-install"

// ExitFailedCheck is the exit code of a start whose build failed its start check after an update.
const ExitFailedCheck = 76

// checkedPage is one page a visitor reaches without signing in that the start check asks for over loopback.
type checkedPage struct {
	path    string
	want    string
	healthy func(status int) bool
}

// checkedPages are the pages the start check asks for, in order: the admin, the locale and the home page.
var checkedPages = []checkedPage{
	{path: "/admin/", want: "200", healthy: answeredOK},
	{path: "/api/locale", want: "200", healthy: answeredOK},
	{path: "/", want: "below 500", healthy: func(status int) bool { return status < http.StatusInternalServerError }},
}

// answeredOK reports whether status is 200.
func answeredOK(status int) bool {
	return status == http.StatusOK
}

// settledLogs are what the guard logs once a start passes its check, by the step that start writes.
var settledLogs = map[Step]string{
	StepCommitted:  "kept the update after its start check",
	StepRolledBack: "took back the update after the build before it passed its start check",
}

// Guard is the boot guard: the start check of a build the last update names, until that update is settled.
type Guard struct {
	settings Settings
	layout   Layout
	state    State
	logger   *slog.Logger
	running  string
	settles  Step
}

// CheckError is the error of a start whose build failed its start check after an update, saying what happened next.
type CheckError struct {
	err error
}

// Error says what failed and what the guard did about it.
func (e CheckError) Error() string {
	return e.err.Error()
}

// Unwrap returns what failed and what the guard did about it.
func (e CheckError) Unwrap() error {
	return e.err
}

// Start counts a start of running after a switch under the state lock, failing past the attempts or once refused.
func Start(settings Settings, running string, logger *slog.Logger) (*Guard, error) {
	g := &Guard{settings: settings, layout: settings.Layout(), logger: logger, running: running}
	if settings.Root == "" {
		return g, nil
	}
	state, unlock, err := g.layout.lockedState()
	if err != nil {
		return nil, err
	}
	defer unlock()
	g.state, g.settles = state, settledStep(state, running)
	if g.settles == "" || state.Step != StepSwitched {
		return g, nil
	}
	if slices.Contains(state.Refused, state.To) {
		return g, g.fail(errors.New("it failed the check at an earlier start, which stopped before it went back"))
	}
	return g, g.count()
}

// settledStep returns the step a start of running writes once it passes its check, empty for a start it does not check.
func settledStep(state State, running string) Step {
	switch {
	case running == state.To && slices.Contains(failedSteps, state.Step):
		return StepCommitted
	case running == state.From && state.Step == StepFailed:
		return StepRolledBack
	}
	return ""
}

// count writes this start as one more attempt, failing the check once the attempts pass the setting.
func (g *Guard) count() error {
	g.state.Attempts++
	if g.state.Attempts > g.settings.Attempts {
		return g.fail(fmt.Errorf("it tried to start %d times and %s allows %d", g.state.Attempts,
			g.settings.names.Key("UPDATE_ATTEMPTS"), g.settings.Attempts))
	}
	return g.layout.WriteState(g.state)
}

// ReachedDatabase writes under the state lock that the build of a start the guard checks reached the database.
func (g *Guard) ReachedDatabase() error {
	if g.settles == "" {
		return nil
	}
	state, unlock, err := g.layout.lockedState()
	if err != nil {
		return err
	}
	defer unlock()
	if !g.pending(state) {
		return fmt.Errorf("updater: another command took back the update to Gophenberg %s while this start waited "+
			"for the database, so it stops before it migrates", g.state.To)
	}
	state.DatabaseReached = true
	return g.layout.WriteState(state)
}

// MarkDatabaseReached writes under the state lock that running, the build an update switched to, reached the database.
func MarkDatabaseReached(settings Settings, running string) error {
	if settings.Root == "" {
		return nil
	}
	layout := settings.Layout()
	state, unlock, err := layout.lockedState()
	if err != nil {
		return err
	}
	defer unlock()
	if running != state.To || !slices.Contains(failedSteps, state.Step) {
		return nil
	}
	state.DatabaseReached = true
	return layout.WriteState(state)
}

// Serve runs serve with the start check once srv listens, ending it when a switched build fails it, or alone with none.
func (g *Guard) Serve(
	ctx context.Context, srv *http.Server, inside func(context.Context) error, serve func(context.Context) error,
) error {
	if g.settles == "" {
		return serve(ctx)
	}
	serving, stop := context.WithCancel(ctx)
	addresses := listening(serving, srv)
	checked := make(chan error, 1)
	go func() {
		err := g.check(serving, addresses, inside)
		if err != nil {
			stop()
		}
		checked <- err
	}()
	served := serve(serving)
	stop()
	return errors.Join(served, <-checked)
}

// listening sets srv to hand the address it listens on to the channel it returns, as it begins to serve under ctx.
func listening(ctx context.Context, srv *http.Server) <-chan string {
	addresses := make(chan string, 1)
	srv.BaseContext = func(listener net.Listener) context.Context {
		addresses <- listener.Addr().String()
		return ctx
	}
	return addresses
}

// check runs the start check and settles the update with what it found, unless the serve ended first.
func (g *Guard) check(ctx context.Context, addresses <-chan string, inside func(context.Context) error) error {
	err := g.verify(ctx, addresses, inside)
	if ctx.Err() != nil {
		return nil
	}
	return g.settle(err)
}

// settle settles the update with the state on disk under the state lock, leaving one another command took back.
func (g *Guard) settle(checked error) error {
	state, unlock, err := g.layout.lockedState()
	if err != nil {
		return err
	}
	defer unlock()
	if !g.pending(state) {
		g.leftTakenBack(state, checked)
		return nil
	}
	g.state = state
	switch {
	case checked == nil:
		return g.commit()
	case state.Step == StepFailed:
		g.logger.Warn("the start check failed and the update stays failed", "version", g.running, "reason", checked)
		return nil
	}
	return g.fail(checked)
}

// leftTakenBack logs that the guard left an update another command took back during its check, and why a check failed.
func (g *Guard) leftTakenBack(state State, checked error) {
	logged := []any{"version", g.running, "step", state.Step}
	if checked != nil {
		logged = append(logged, "reason", checked)
	}
	g.logger.Warn("another command took back the update while its start check ran, so the guard left it as it is",
		logged...)
}

// pending reports whether state still names the update the guard checks, at the same step and to the same version.
func (g *Guard) pending(state State) bool {
	return state.Step == g.state.Step && state.To == g.state.To
}

// verify checks the build inside the process, then asks for each checked page once it listens, within the timeout.
func (g *Guard) verify(ctx context.Context, addresses <-chan string, inside func(context.Context) error) error {
	checking, cancel := context.WithTimeout(ctx, g.settings.VerifyTimeout)
	defer cancel()
	if err := inside(checking); err != nil {
		return err
	}
	select {
	case address := <-addresses:
		return CheckPages(checking, pageClient().Do, loopback(address))
	case <-checking.Done():
		return fmt.Errorf("it did not listen within %v, the time %s allows", g.settings.VerifyTimeout,
			g.settings.names.Key("UPDATE_VERIFY_TIMEOUT"))
	}
}

// pageClient returns the client the start check asks with, on a new connection each time and following no redirect.
func pageClient() *http.Client {
	return &http.Client{
		Transport:     &http.Transport{DisableKeepAlives: true},
		CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse },
	}
}

// CheckPages asks the site at address for each checked page through fetch, refusing the first answer it does not take.
func CheckPages(ctx context.Context, fetch func(*http.Request) (*http.Response, error), address string) error {
	for _, page := range checkedPages {
		request, err := http.NewRequestWithContext(ctx, http.MethodGet, "http://"+address+page.path, nil)
		var response *http.Response
		if err == nil {
			response, err = fetch(request)
		}
		if err != nil {
			return fmt.Errorf("it could not ask for %s: %w", page.path, err)
		}
		_ = response.Body.Close()
		if !page.healthy(response.StatusCode) {
			return fmt.Errorf("%s answered %d, want %s", page.path, response.StatusCode, page.want)
		}
	}
	return nil
}

// loopback returns the address on this host that reaches a listener at address, which may listen on every interface.
func loopback(address string) string {
	host, port, _ := net.SplitHostPort(address)
	ip := net.ParseIP(host)
	switch {
	case !ip.IsUnspecified():
		return address
	case ip.To4() != nil:
		return net.JoinHostPort(net.IPv4(127, 0, 0, 1).String(), port)
	}
	return net.JoinHostPort(net.IPv6loopback.String(), port)
}

// commit writes the step the passed start check settles the update at, then deletes the builds past the kept number.
func (g *Guard) commit() error {
	g.state.Step, g.state.Refused = g.settles, lifting(g.state.Refused, g.running)
	if err := g.layout.WriteState(g.state); err != nil {
		return err
	}
	g.layout.prune(g.settings.VersionsKept)
	g.logger.Info(settledLogs[g.settles], "version", g.state.To, "from", g.state.From)
	return nil
}

// prune deletes every build past the newest keep in the versions folder, except one the current or previous link names.
func (l Layout) prune(keep int) {
	current, _ := l.Named(Current)
	previous, _ := l.Named(Previous)
	linked := []string{current, previous}
	for i, name := range l.builds() {
		if i >= keep && !slices.Contains(linked, name) {
			_ = os.RemoveAll(l.Build(name))
		}
	}
}

// builds returns the name of each folder of the versions folder that names a version, the newest first.
func (l Layout) builds() []string {
	entries, _ := os.ReadDir(filepath.Join(l.Root, versionsFolder))
	builds := make([]string, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() && version.Valid(entry.Name()) {
			builds = append(builds, entry.Name())
		}
	}
	slices.SortFunc(builds, func(a, b string) int { return version.Compare(b, a) })
	return builds
}

// fail refuses the build that failed its start check, then goes back or stays down, and returns what it did.
func (g *Guard) fail(reason error) error {
	failed := fmt.Errorf("updater: Gophenberg %s failed its start check after the update from %s: %w",
		g.state.To, g.state.From, reason)
	if mayHaveMigrated(g.state) {
		return CheckError{err: errors.Join(failed, g.layout.stayDown(g.state))}
	}
	return CheckError{err: errors.Join(failed, g.goBack(g.state))}
}

// refusing returns the versions refused with release among them, once, leaving refused as it is.
func refusing(refused []string, release string) []string {
	if slices.Contains(refused, release) {
		return refused
	}
	return append(slices.Clone(refused), release)
}

// lifting returns the versions refused without release, leaving refused as it is.
func lifting(refused []string, release string) []string {
	return slices.DeleteFunc(slices.Clone(refused), func(version string) bool { return version == release })
}

// goBack writes state refusing its version and points current at the build before the update, saying what starts next.
func (g *Guard) goBack(state State) error {
	state.Refused = refusing(state.Refused, state.To)
	err := g.layout.WriteState(state)
	if err == nil {
		err = g.layout.Point(Current, state.From)
	}
	if err != nil {
		return err
	}
	return fmt.Errorf("updater: current names Gophenberg %s again, which the next start runs, and update:apply never "+
		"takes %s again on its own", state.From, state.To)
}

// stayDown writes that the update failed, refusing its version and keeping current, and returns the restore steps.
func (l Layout) stayDown(state State) error {
	state.Step, state.Refused = StepFailed, refusing(state.Refused, state.To)
	return errors.Join(
		l.WriteState(state),
		fmt.Errorf("updater: %s and the site stays down", raisedPast(state)),
		errors.New("updater: "+l.restoreText(state)),
	)
}

// raisedPast says the build an update switched to raised the minimum version past the build before it.
func raisedPast(state State) string {
	return fmt.Sprintf("Gophenberg %s raised the minimum version past %s, so %s cannot open the database",
		state.To, state.From, state.From)
}

// restoreText says how to restore the backup taken before the update and start the build before it again.
func (l Layout) restoreText(state State) string {
	return fmt.Sprintf("restore the backup taken before the update, %s, into an empty database, point current at "+
		"%s and start Gophenberg again, as %s shows, or fix what failed and start Gophenberg %s again",
		filepath.Join(l.Kept(), state.Backup), filepath.Join(versionsFolder, state.From), restoreSteps, state.To)
}
