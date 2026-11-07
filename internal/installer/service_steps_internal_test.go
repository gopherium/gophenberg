// SPDX-License-Identifier: Apache-2.0

package installer

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"net/http"
	"net/netip"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"syscall"
	"testing"
	"testing/synctest"
	"time"

	"github.com/gopherium/framework/gonsole"

	"github.com/gopherium/gophenberg/internal/version"
)

// checkLine is the line the site-env step runs check with, as gophenberg in the default install root.
const checkLine = "runuser -u gophenberg -- /opt/gophenberg/current/gophenberg check"

// caddyValidateLine is the line the caddy step checks Caddy's files with, as the caddy user.
const caddyValidateLine = "runuser -u caddy -- caddy validate --config /etc/caddy/Caddyfile --adapter caddyfile"

// createAdminLine is the line the admin step creates Maria Perez as the first administrator with.
const createAdminLine = "runuser -u gophenberg -- /opt/gophenberg/current/gophenberg account:create-admin -email " +
	"admin@example.com -name Maria Perez -role admin"

// mariaPerez is the first administrator the tests answer for.
var mariaPerez = admin{email: "admin@example.com", name: "Maria Perez", password: goodPassword}

// servedOn returns the run that takes the steps of planned on server, its user and install root made, its answer blank.
func servedOn(t *testing.T, server *fakeServer, planned site) (*installing, *strings.Builder) {
	t.Helper()
	check := server.check()
	check.settings, check.record = planned.settings, planned.record
	answer := &strings.Builder{}
	running, err := installingOn(check, planned, gonsole.Call{Stdout: answer, Stderr: &server.notes})
	if err != nil {
		t.Fatalf("installingOn() = %v, want the run ready", err)
	}
	t.Cleanup(running.close)
	for _, id := range []string{"user", "install-root"} {
		if err := running.take(t.Context(), plannedStep(t, running, id)); err != nil {
			t.Fatalf("take(%s) = %v, want it taken", id, err)
		}
	}
	server.forgetRun()
	answer.Reset()
	return running, answer
}

// laterSite returns the site of a later run for example.com, whose record holds what an earlier run installed.
func laterSite(t *testing.T) site {
	t.Helper()
	planned := firstSite(t, "example.com", 255)
	planned.current = true
	planned.record = Record{
		Installer: "0.0.1", Domain: "example.com", Caddyfile: caddyfileSum(wantedCaddyfile),
		Installed: []string{piecePostgreSQL, pieceCaddy, pieceService, "/opt/gophenberg"},
		Steps:     map[string]string{stepCaddy: stepEnded, stepStart: stepEnded, stepAdmin: stepEnded},
	}
	return planned
}

// wantMark fails t unless the install record on server marks the step called id as mark.
func wantMark(t *testing.T, server *fakeServer, id, mark string) {
	t.Helper()
	if got := marks(t, server)[id]; got != mark {
		t.Errorf("the record marks the step %s %q, want %s", id, got, mark)
	}
}

func TestEveryStepOfEveryRunHasAnApply(t *testing.T) {
	t.Parallel()

	for _, planned := range []site{firstSite(t, "example.com", 255), ownSite(t, "example.com"), laterSite(t)} {
		for _, taken := range planned.steps() {
			if taken.apply == nil {
				t.Errorf("the step %s has no apply", taken.id)
			}
		}
	}
}

func TestTheSiteEnvStepWritesTheSitesSettingsAsGophenbergsThenRunsCheckInTheRoot(t *testing.T) {
	t.Parallel()

	server := newFakeServer(t)
	running, _ := servedOn(t, server, firstSite(t, "example.com", 255))

	err := running.take(t.Context(), plannedStep(t, running, "site-env"))

	if err != nil {
		t.Fatalf("take(site-env) = %v, want .env written and checked", err)
	}
	wantText(t, server.at(siteEnvAt), wantedSiteEnv)
	wantEntry(t, server.at(siteEnvAt), 0o600, &owner{uid: os.Getuid(), gid: siteGroup(t)})
	wantRan(t, server, checkLine)
	if dir := server.runs[0].Dir; dir != "/opt/gophenberg" {
		t.Errorf("check ran in %q, want /opt/gophenberg", dir)
	}
}

func TestTheSiteEnvStepStopsWithWhatCheckPrinted(t *testing.T) {
	t.Parallel()

	server := newFakeServer(t)
	server.answers["gophenberg check"] = failed("/opt/gophenberg/current/gophenberg", "GOPHENBERG_ADDR: taken\n")
	running, _ := servedOn(t, server, firstSite(t, "example.com", 255))

	err := running.take(t.Context(), plannedStep(t, running, "site-env"))

	wantRefused(t, err, "installer: run /opt/gophenberg/current/gophenberg: exit status 1, after it printed:\n"+
		"GOPHENBERG_ADDR: taken")
	wantMark(t, server, "site-env", stepStarted)
}

func TestTheUnitsStepOwnsTheServiceAndWritesTheUnitsBeforeItReloadsSystemd(t *testing.T) {
	t.Parallel()

	server := newFakeServer(t)
	running, _ := servedOn(t, server, firstSite(t, "example.com", 255))
	var owned, written bool
	server.effects["systemctl"] = func(program Program) answer {
		if program.Args[0] == "daemon-reload" {
			record, _ := readRecord(server.machine())
			owned, written = record.owns(pieceService), server.machine().there("/etc/systemd/system/gophenberg.service")
		}
		return server.systemctl(program)
	}

	err := running.take(t.Context(), plannedStep(t, running, "units"))

	if err != nil {
		t.Fatalf("take(units) = %v, want the units written", err)
	}
	wantRan(t, server, "systemctl daemon-reload", "systemctl enable gophenberg.service")
	if !owned || !written {
		t.Errorf("daemon-reload found the record owning the service %t and the unit written %t, want both", owned,
			written)
	}
	for _, unit := range unitsOf(wantedSiteUnit) {
		wantText(t, server.at(unit.path), unit.text)
		wantEntry(t, server.at(unit.path), 0o644, nil)
	}
}

func TestTheUnitsStepOwnsTheServiceInTheRecordBeforeItWritesTheUnits(t *testing.T) {
	t.Parallel()

	server := newFakeServer(t)
	server.folder("/etc/systemd/system/gophenberg-rollback.service")
	running, _ := servedOn(t, server, firstSite(t, "example.com", 255))

	err := running.take(t.Context(), plannedStep(t, running, "units"))

	want := "installer: write /etc/systemd/system/gophenberg-rollback.service: "
	if got := fmt.Sprint(err); !strings.HasPrefix(got, want) {
		t.Errorf("take(units) = %v, want the rollback unit refused", err)
	}
	if record, _ := readRecord(server.machine()); !record.owns(pieceService) {
		t.Errorf("the record owns %q, want %s among them", record.Installed, pieceService)
	}
	wantRan(t, server)
}

func TestTheUnitsStepStopsWhenSystemdCannotReload(t *testing.T) {
	t.Parallel()

	server := newFakeServer(t)
	running, _ := servedOn(t, server, firstSite(t, "example.com", 255))
	server.effects["systemctl"] = func(Program) answer { return failed("systemctl", "Access denied\n") }

	err := running.take(t.Context(), plannedStep(t, running, "units"))

	wantRefused(t, err, "installer: run systemctl: exit status 1, after it printed:\nAccess denied")
	wantRan(t, server, "systemctl daemon-reload")
}

func TestTheCommandStepWritesTheCommandOnThePath(t *testing.T) {
	t.Parallel()

	server := newFakeServer(t)
	running, _ := servedOn(t, server, firstSite(t, "example.com", 255))

	err := running.take(t.Context(), plannedStep(t, running, "command"))

	if err != nil {
		t.Fatalf("take(command) = %v, want the command written", err)
	}
	wantText(t, server.at(commandPath), wantedCommand)
	wantEntry(t, server.at(commandPath), 0o755, nil)
}

func TestTheCaddyStepOfAFirstRunKeepsThePackagesCaddyfileThenRestartsCaddy(t *testing.T) {
	t.Parallel()

	server := newFakeServer(t)
	server.installPackage(pieceCaddy)
	running, _ := servedOn(t, server, firstSite(t, "example.com", 255))

	err := running.take(t.Context(), plannedStep(t, running, stepCaddy))

	if err != nil {
		t.Fatalf("take(caddy) = %v, want Caddy set up", err)
	}
	wantText(t, server.at(keptCaddyfilePath), packageCaddyfile)
	for _, written := range caddyFilesOf(wantedCaddySite, wantedCaddyfile) {
		wantText(t, server.at(written.path), written.text)
		wantEntry(t, server.at(written.path), 0o644, nil)
	}
	wantRan(t, server, caddyValidateLine, "systemctl restart caddy.service")
	if env := server.runs[0].Env; !slices.Equal(env, []string{"HOME=/var/lib/caddy"}) {
		t.Errorf("caddy validate ran with the settings %q, want HOME=/var/lib/caddy alone", env)
	}
	record, err := readRecord(server.machine())
	if err != nil || record.Caddyfile != caddyfileSum(wantedCaddyfile) {
		t.Errorf("the record holds the Caddyfile sum %q, %v, want that of the Caddyfile written", record.Caddyfile, err)
	}
}

func TestTheCaddyStepOfAFirstRunSavesTheSumBeforeCaddyRestarts(t *testing.T) {
	t.Parallel()

	server := newFakeServer(t)
	server.installPackage(pieceCaddy)
	var saved string
	server.effects["systemctl"] = func(Program) answer {
		record, _ := readRecord(server.machine())
		saved = record.Caddyfile
		return failed("systemctl", "Job for caddy.service failed.\n")
	}
	running, _ := servedOn(t, server, firstSite(t, "example.com", 255))

	err := running.take(t.Context(), plannedStep(t, running, stepCaddy))

	wantRefused(t, err, "installer: run systemctl: exit status 1, after it printed:\nJob for caddy.service failed.")
	wantText(t, server.at(caddyfilePath), wantedCaddyfile)
	if saved != caddyfileSum(wantedCaddyfile) {
		t.Errorf("the restart found the Caddyfile sum %q in the record, want that of the Caddyfile written, so the "+
			"next run keeps the Caddyfile", saved)
	}
	wantMark(t, server, stepCaddy, stepStarted)
}

func TestTheCaddyStepOfARunAfterAFailedRestartKeepsThePersonsCaddyfileAndRestartsCaddy(t *testing.T) {
	t.Parallel()

	server := newFakeServer(t)
	server.installPackage(pieceCaddy)
	server.write(keptCaddyfilePath, packageCaddyfile)
	server.write(caddyfilePath, wantedCaddyfile+personSite)
	planned := laterSite(t)
	planned.record.Steps[stepCaddy] = stepStarted
	running, _ := servedOn(t, server, planned)

	err := running.take(t.Context(), plannedStep(t, running, stepCaddy))

	if err != nil {
		t.Fatalf("take(caddy) = %v, want Caddy restarted", err)
	}
	wantText(t, server.at(caddyfilePath), wantedCaddyfile+personSite)
	wantText(t, server.at(keptCaddyfilePath), packageCaddyfile)
	wantRan(t, server, caddyValidateLine, "systemctl restart caddy.service")
	wantMark(t, server, stepCaddy, stepEnded)
}

func TestTheCaddyStepKeepsTheCaddyfileACutOffRunAlreadyKept(t *testing.T) {
	t.Parallel()

	server := newFakeServer(t)
	server.installPackage(pieceCaddy)
	server.write(keptCaddyfilePath, packageCaddyfile)
	server.write(caddyfilePath, wantedCaddyfile)
	running, _ := servedOn(t, server, firstSite(t, "example.com", 255))

	err := running.take(t.Context(), plannedStep(t, running, stepCaddy))

	if err != nil {
		t.Fatalf("take(caddy) = %v, want Caddy set up", err)
	}
	wantText(t, server.at(keptCaddyfilePath), packageCaddyfile)
	wantText(t, server.at(caddyfilePath), wantedCaddyfile)
}

func TestTheCaddyStepNeverKeepsACaddyfileThatImportsGophenbergsSiteAsThePackages(t *testing.T) {
	t.Parallel()

	server := newFakeServer(t)
	server.installPackage(pieceCaddy)
	server.write(caddyfilePath, wantedLocalCaddyfile+personSite)
	running, _ := servedOn(t, server, firstSite(t, "example.com", 255))

	err := running.take(t.Context(), plannedStep(t, running, stepCaddy))

	if err != nil {
		t.Fatalf("take(caddy) = %v, want Caddy set up", err)
	}
	wantText(t, server.at(caddyfilePath), wantedCaddyfile)
	if server.machine().there(keptCaddyfilePath) {
		t.Errorf("the run kept a Caddyfile that imports %s as %s, want it never kept as the package's",
			caddySitePath, keptCaddyfilePath)
	}
}

func TestTheCaddyStepWritesTheCaddyfileWhereThePackageLeftNone(t *testing.T) {
	t.Parallel()

	server := newFakeServer(t)
	server.installPackage(pieceCaddy)
	server.remove(caddyfilePath)
	running, _ := servedOn(t, server, firstSite(t, "example.com", 255))

	err := running.take(t.Context(), plannedStep(t, running, stepCaddy))

	if err != nil {
		t.Fatalf("take(caddy) = %v, want Caddy set up", err)
	}
	wantText(t, server.at(caddyfilePath), wantedCaddyfile)
	if server.machine().there(keptCaddyfilePath) {
		t.Errorf("the run kept a %s where the package left no Caddyfile", keptCaddyfilePath)
	}
}

func TestTheCaddyStepOfALaterRunLeavesTheCaddyfileAloneAndReloadsCaddy(t *testing.T) {
	t.Parallel()

	server := newFakeServer(t)
	server.installPackage(pieceCaddy)
	server.running["caddy.service"] = true
	own := wantedCaddyfile + mistypedSite
	server.write(caddyfilePath, strings.Replace(own, "respnd", "respond", 1))
	running, _ := servedOn(t, server, laterSite(t))

	err := running.take(t.Context(), plannedStep(t, running, stepCaddy))

	if err != nil {
		t.Fatalf("take(caddy) = %v, want Caddy reloaded", err)
	}
	wantText(t, server.at(caddyfilePath), strings.Replace(own, "respnd", "respond", 1))
	wantText(t, server.at(caddySitePath), wantedCaddySite)
	wantRan(t, server, caddyValidateLine, "systemctl reload caddy.service")
	if server.machine().there(keptCaddyfilePath) {
		t.Errorf("a later run kept a %s, want the Caddyfile left alone", keptCaddyfilePath)
	}
}

func TestTheCaddyStepStopsBeforeCaddyRestartsWhenCaddyValidateRefusesTheFiles(t *testing.T) {
	t.Parallel()

	server := newFakeServer(t)
	server.installPackage(pieceCaddy)
	server.answers["caddy"] = failed("caddy", "Error: unrecognized directive: lb_try_duration\n")
	delete(server.effects, "caddy")
	running, _ := servedOn(t, server, firstSite(t, "example.com", 255))

	err := running.take(t.Context(), plannedStep(t, running, stepCaddy))

	wantRefused(t, err, "caddy validate refused Caddy's files, so Caddy runs on with the settings it has: installer: "+
		"run caddy: exit status 1, after it printed:\nError: unrecognized directive: lb_try_duration")
	wantRan(t, server, caddyValidateLine)
	if record, _ := readRecord(server.machine()); record.Caddyfile != "" {
		t.Errorf("the record holds the Caddyfile sum %q after caddy validate refused, want none", record.Caddyfile)
	}
}

func TestTheCaddyStepStopsBeforeCaddyValidateWhenItCannotWriteCaddysFiles(t *testing.T) {
	t.Parallel()

	server := newFakeServer(t)
	server.installPackage(pieceCaddy)
	server.write(maintenanceFolder, "a file where the folder goes\n")
	running, _ := servedOn(t, server, firstSite(t, "example.com", 255))

	err := running.take(t.Context(), plannedStep(t, running, stepCaddy))

	if got := fmt.Sprint(err); !strings.HasPrefix(got, "installer: write "+maintenanceFolder+"/index.html: ") {
		t.Errorf("take(caddy) = %v, want the maintenance page refused", err)
	}
	wantRan(t, server)
}

func TestTheCaddyStepRefusesAServerWithoutTheCaddyUser(t *testing.T) {
	t.Parallel()

	server := newFakeServer(t)
	server.installPackage(pieceCaddy)
	delete(server.users, pieceCaddy)
	running, _ := servedOn(t, server, firstSite(t, "example.com", 255))

	err := running.take(t.Context(), plannedStep(t, running, stepCaddy))

	wantRefused(t, err, "installer: user: unknown user caddy")
}

func TestTheCaddyStepRefusesAPackageCaddyfileItCannotKeep(t *testing.T) {
	t.Parallel()

	skipAsRoot(t)
	server := newFakeServer(t)
	server.installPackage(pieceCaddy)
	server.chmod("/etc/caddy", 0o555)
	t.Cleanup(func() { server.chmod("/etc/caddy", 0o755) })
	running, _ := servedOn(t, server, firstSite(t, "example.com", 255))

	err := running.take(t.Context(), plannedStep(t, running, stepCaddy))

	if got := fmt.Sprint(err); !strings.HasPrefix(got, "installer: keep the caddy package's Caddyfile as "+
		keptCaddyfilePath+": ") {
		t.Errorf("take(caddy) = %v, want the rename refused", err)
	}
}

func TestTheCaddyStepRefusesACaddyFolderItCannotDescribe(t *testing.T) {
	t.Parallel()

	server := newFakeServer(t)
	server.installPackage(pieceCaddy)
	server.remove("/etc/caddy")
	server.write("/etc/caddy", "a file where the folder goes\n")
	running, _ := servedOn(t, server, firstSite(t, "example.com", 255))

	err := running.take(t.Context(), plannedStep(t, running, stepCaddy))

	if got := fmt.Sprint(err); !strings.HasPrefix(got, "installer: keep the caddy package's Caddyfile as ") {
		t.Errorf("take(caddy) = %v, want the Lstat refused", err)
	}
}

func TestTheStartStepStartsTheSiteAndWaitsUntilItsPagesAnswer(t *testing.T) {
	t.Parallel()

	synctest.Test(t, func(t *testing.T) {
		server := newFakeServer(t)
		running, _ := servedOn(t, server, firstSite(t, "example.com", 255))
		started := time.Now()
		server.answerFetch = func(request *http.Request) (*http.Response, error) {
			if time.Since(started) < 10*time.Second {
				return nil, errors.New("dial tcp 127.0.0.1:8081: connect: connection refused")
			}
			return server.sitePage(request)
		}

		err := running.take(t.Context(), plannedStep(t, running, stepStart))

		if err != nil || time.Since(started) != 10*time.Second {
			t.Errorf("take(start) = %v after %s, want the pages answered after 10s", err, time.Since(started))
		}
		wantRan(t, server, "systemctl reset-failed gophenberg.service", "systemctl start gophenberg.service")
		want := []string{"127.0.0.1:8081 http://127.0.0.1:8081/admin/", "127.0.0.1:8081 http://127.0.0.1:8081/api/locale",
			"127.0.0.1:8081 http://127.0.0.1:8081/"}
		if got := server.fetched[len(server.fetched)-3:]; len(server.fetched) != 13 || !slices.Equal(got, want) {
			t.Errorf("the start step fetched %q, want ten tries of /admin/, then the three pages", server.fetched)
		}
	})
}

func TestTheStartStepRestartsASiteWhoseUnitsAnEarlierRunWrote(t *testing.T) {
	t.Parallel()

	server := newFakeServer(t)
	cutOff := firstSite(t, "example.com", 255)
	cutOff.record = Record{Installed: []string{pieceService}, Steps: map[string]string{stepStart: stepStarted}}
	running, _ := servedOn(t, server, cutOff)

	err := running.take(t.Context(), plannedStep(t, running, stepStart))

	if err != nil {
		t.Fatalf("take(start) = %v, want the site restarted", err)
	}
	wantRan(t, server, "systemctl reset-failed gophenberg.service", "systemctl restart gophenberg.service")
}

func TestTheStartStepStopsOnceTheSiteDoesNotAnswerWithinTheStartWait(t *testing.T) {
	t.Parallel()

	synctest.Test(t, func(t *testing.T) {
		server := newFakeServer(t)
		server.siteDown = true
		running, _ := servedOn(t, server, firstSite(t, "example.com", 255))
		started := time.Now()

		err := running.take(t.Context(), plannedStep(t, running, stepStart))

		wantRefused(t, err, "gophenberg.service did not pass its start check within 1m0s, the time "+
			"GOPHENBERG_UPDATE_VERIFY_TIMEOUT allows, as at the last try it could not ask for /admin/: dial tcp "+
			"127.0.0.1:8081: connect: connection refused, so read why with sudo journalctl -u gophenberg")
		if waited := time.Since(started); waited != time.Minute {
			t.Errorf("the start step waited %s, want 1m0s", waited)
		}
		wantMark(t, server, stepStart, stepStarted)
	})
}

func TestTheStartStepStopsWhenSystemdCannotStartTheSite(t *testing.T) {
	t.Parallel()

	server := newFakeServer(t)
	running, _ := servedOn(t, server, firstSite(t, "example.com", 255))
	server.effects["systemctl"] = func(Program) answer { return failed("systemctl", "Unit not found\n") }

	err := running.take(t.Context(), plannedStep(t, running, stepStart))

	wantRefused(t, err, "installer: run systemctl: exit status 1, after it printed:\nUnit not found")
	if len(server.fetched) != 0 {
		t.Errorf("the start step fetched %q once systemd refused, want nothing", server.fetched)
	}
}

func TestTheStartStepEndsAtOnceWhenTheRunEnds(t *testing.T) {
	t.Parallel()

	server := newFakeServer(t)
	server.siteDown = true
	running, _ := servedOn(t, server, firstSite(t, "example.com", 255))
	ctx, cancel := context.WithCancel(t.Context())
	cancel()

	err := running.start(ctx, plannedStep(t, running, stepStart))

	if !errors.Is(err, context.Canceled) {
		t.Errorf("start() = %v, want the run's own end", err)
	}
}

func TestTheAdminStepCreatesTheFirstAdministratorAsGophenbergWithThePasswordOnItsInput(t *testing.T) {
	t.Parallel()

	server := newFakeServer(t)
	running, _ := servedOn(t, server, firstSite(t, "example.com", 255))
	running.admin = mariaPerez

	err := running.take(t.Context(), plannedStep(t, running, stepAdmin))

	if err != nil || !running.created {
		t.Fatalf("take(admin) = %v, created %t, want Maria Perez created", err, running.created)
	}
	wantRan(t, server, createAdminLine)
	want := [2]string{"/opt/gophenberg", goodPassword + "\n"}
	if got := [2]string{server.runs[0].Dir, server.runs[0].Stdin}; got != want {
		t.Errorf("account:create-admin ran in %q with %q on its input, want /opt/gophenberg and the password", got[0],
			got[1])
	}
}

func TestTheAdminStepTakesAnEmailAlreadyTakenAsMade(t *testing.T) {
	t.Parallel()

	server := newFakeServer(t)
	server.accounts = []listedAccount{{Email: "admin@example.com", Role: "admin"}}
	running, _ := servedOn(t, server, firstSite(t, "example.com", 255))
	running.admin = mariaPerez

	err := running.take(t.Context(), plannedStep(t, running, stepAdmin))

	if err != nil || running.created {
		t.Errorf("take(admin) = %v, created %t, want the account taken as made by an earlier run", err, running.created)
	}
	wantMark(t, server, stepAdmin, stepEnded)
}

func TestTheAdminStepStopsWhenTheEmailBelongsToAnAccountUnderAnotherRole(t *testing.T) {
	t.Parallel()

	server := newFakeServer(t)
	server.accounts = []listedAccount{{Email: "admin@example.com", Role: "editor"}}
	running, _ := servedOn(t, server, firstSite(t, "example.com", 255))
	running.admin = mariaPerez

	err := running.take(t.Context(), plannedStep(t, running, stepAdmin))

	wantRefused(t, err, "admin@example.com already belongs to an account that is not an administrator, so run sudo "+
		"gophenberg server:install again and give the first administrator another email")
	if running.created {
		t.Error("the run says it created the administrator, want none created")
	}
	wantMark(t, server, stepAdmin, stepStarted)
}

func TestTheAdminStepStopsWhenTheEmailBelongsToAnEditorWhileAnotherEmailIsUnderAdmin(t *testing.T) {
	t.Parallel()

	server := newFakeServer(t)
	server.accounts = []listedAccount{
		{Email: "owner@example.com", Role: "admin"}, {Email: "admin@example.com", Role: "editor"},
	}
	running, _ := servedOn(t, server, firstSite(t, "example.com", 255))
	running.admin = mariaPerez

	err := running.take(t.Context(), plannedStep(t, running, stepAdmin))

	wantRefused(t, err, "admin@example.com already belongs to an account that is not an administrator, so run sudo "+
		"gophenberg server:install again and give the first administrator another email")
	wantMark(t, server, stepAdmin, stepStarted)
}

func TestTheAdminStepStopsWhenAccountListFailsOnAnEmailAlreadyTaken(t *testing.T) {
	t.Parallel()

	server := newFakeServer(t)
	server.accounts = []listedAccount{{Email: "admin@example.com", Role: "admin"}}
	server.effects["gophenberg account:list"] = func(program Program) answer {
		return failed(program.Name, "database: connection refused\n")
	}
	running, _ := servedOn(t, server, firstSite(t, "example.com", 255))
	running.admin = mariaPerez

	err := running.take(t.Context(), plannedStep(t, running, stepAdmin))

	wantRefused(t, err, "installer: run /opt/gophenberg/current/gophenberg: exit status 1, after it printed:\n"+
		"database: connection refused")
	wantMark(t, server, stepAdmin, stepStarted)
}

func TestTheAdminStepStopsWithWhatCreateAdminPrinted(t *testing.T) {
	t.Parallel()

	server := newFakeServer(t)
	server.effects["gophenberg account:create-admin"] = func(program Program) answer {
		return failed(program.Name, "Password: unknown role admin\n")
	}
	running, _ := servedOn(t, server, firstSite(t, "example.com", 255))
	running.admin = mariaPerez

	err := running.take(t.Context(), plannedStep(t, running, stepAdmin))

	wantRefused(t, err, "installer: run /opt/gophenberg/current/gophenberg: exit status 1, after it printed:\n"+
		"Password: unknown role admin")
	wantMark(t, server, stepAdmin, stepStarted)
}

func TestTheHTTPSStepTrustsTheSystemsRootsForARealDomain(t *testing.T) {
	t.Parallel()

	server := newFakeServer(t)
	server.running["caddy.service"] = true
	running, _ := servedOn(t, server, firstSite(t, "example.com", 255))

	err := running.take(t.Context(), plannedStep(t, running, "https"))

	if err != nil || !slices.Equal(server.fetchedHTTPS, []string{"https://example.com/admin/ with roots false"}) {
		t.Errorf("take(https) = %v, fetching %q, want https://example.com/admin/ once with the system's roots", err,
			server.fetchedHTTPS)
	}
	wantMark(t, server, "https", stepEnded)
}

func TestTheHTTPSStepReadsCaddysLocalRootAgainOnEachTryForLocalhost(t *testing.T) {
	t.Parallel()

	synctest.Test(t, func(t *testing.T) {
		server := newFakeServer(t)
		server.installPackage(pieceCaddy)
		server.running["caddy.service"] = true
		running, _ := servedOn(t, server, firstSite(t, "localhost", 255))
		time.AfterFunc(5*time.Second, func() { server.write(localRootAt, fakeCaddyRoot()) })

		err := running.take(t.Context(), plannedStep(t, running, "https"))

		if err != nil || len(server.fetchedHTTPS) != 1 {
			t.Fatalf("take(https) = %v, fetching %q, want one fetch once Caddy made its local root", err,
				server.fetchedHTTPS)
		}
		if !server.httpsRoots[0].Equal(fakeCaddyRoots()) {
			t.Error("the HTTPS check trusted other roots than Caddy's local root")
		}
	})
}

// httpsWarning is the warning of an HTTPS check whose every try ended as last says.
func httpsWarning(last string) string {
	return "warning: https://example.com/admin/ did not answer 200 within 2m0s, the time GOPHENBERG_INSTALL_HTTPS_WAIT " +
		"allows, as at the last try " + last + ". The site runs, so check that the hosting provider's firewall lets " +
		"TCP 80, TCP 443 and UDP 443 in, then read sudo journalctl -u caddy, and the next sudo gophenberg " +
		"server:install checks again\n"
}

func TestTheHTTPSStepWarnsAndLeavesTheStepOpenOnceHTTPSDoesNotAnswerWithinItsWait(t *testing.T) {
	t.Parallel()

	synctest.Test(t, func(t *testing.T) {
		server := newFakeServer(t)
		server.answerHTTPS = answered(http.StatusBadGateway, "")
		running, _ := servedOn(t, server, firstSite(t, "example.com", 255))
		started := time.Now()

		err := running.take(t.Context(), plannedStep(t, running, "https"))

		if err != nil || time.Since(started) != 2*time.Minute {
			t.Errorf("take(https) = %v after %s, want nil after 2m0s", err, time.Since(started))
		}
		if got, want := server.notes.String(), httpsWarning("it answered 502"); got != want {
			t.Errorf("the run noted %q, want %q", got, want)
		}
		wantMark(t, server, "https", stepStarted)
	})
}

func TestTheHTTPSStepTriesAgainPastACaddyThatCannotBeReached(t *testing.T) {
	t.Parallel()

	synctest.Test(t, func(t *testing.T) {
		server := newFakeServer(t)
		running, _ := servedOn(t, server, firstSite(t, "example.com", 255))

		err := running.take(t.Context(), plannedStep(t, running, "https"))

		want := httpsWarning("it could not be reached: dial tcp example.com:443: connect: connection refused")
		if got := server.notes.String(); err != nil || got != want {
			t.Errorf("take(https) = %v, noting %q, want nil and %q", err, got, want)
		}
	})
}

func TestTheHTTPSStepTriesAgainPastALocalRootThatIsMissingOrHoldsNoCertificate(t *testing.T) {
	t.Parallel()

	for held, last := range map[string]string{
		"":                  "the local root Caddy signs localhost with was not at " + localRootAt + " yet",
		"not a certificate": localRootAt + " holds no certificate",
	} {
		synctest.Test(t, func(t *testing.T) {
			server := newFakeServer(t)
			server.installPackage(pieceCaddy)
			if held != "" {
				server.write(localRootAt, held)
			}
			running, _ := servedOn(t, server, firstSite(t, "localhost", 255))

			err := running.https(t.Context(), plannedStep(t, running, "https"))

			want := strings.Replace(httpsWarning(last), "https://example.com/", "https://localhost/", 1)
			if got := server.notes.String(); !errors.Is(err, errLeftOpen) || got != want {
				t.Errorf("https() = %v, noting %q, want the step left open and %q", err, got, want)
			}
		})
	}
}

// localRootAt is where the fake Caddy keeps the local root it signs localhost with, named from the top of the server.
const localRootAt = "/var/lib/caddy/" + caddyLocalRoot

// localhostOn returns the run that takes the steps of a first run for localhost on server, which Caddy is on.
func localhostOn(t *testing.T, server *fakeServer) *installing {
	t.Helper()
	server.installPackage(pieceCaddy)
	running, _ := servedOn(t, server, firstSite(t, "localhost", 255))
	return running
}

func TestTheHTTPSCheckRefusesALocalRootThatIsALink(t *testing.T) {
	t.Parallel()

	server := newFakeServer(t)
	running := localhostOn(t, server)
	server.write("/var/lib/caddy/elsewhere.crt", fakeCaddyRoot())
	server.link(localRootAt, "/var/lib/caddy/elsewhere.crt")

	roots, err := running.httpsRoots()

	wantRefused(t, err, "installer: open "+server.at(localRootAt)+": too many levels of symbolic links")
	if roots != nil {
		t.Error("httpsRoots() trusted the root a link leads to, want no roots")
	}
}

func TestTheHTTPSCheckRefusesALocalRootThatIsAPipeWithoutWaitingOnIt(t *testing.T) {
	t.Parallel()

	server := newFakeServer(t)
	running := localhostOn(t, server)
	server.folder(filepath.Dir(localRootAt))
	if err := syscall.Mkfifo(server.at(localRootAt), 0o600); err != nil {
		t.Fatalf("planting the pipe: %v", err)
	}
	read := make(chan error, 1)

	go func() {
		_, err := running.httpsRoots()
		read <- err
	}()

	select {
	case err := <-read:
		wantRefused(t, err, "installer: read "+localRootAt+": it is not a regular file")
	case <-time.After(2 * time.Second):
		t.Error("httpsRoots() still waits on a pipe after 2s, want the pipe refused at once")
	}
}

func TestTheHTTPSCheckFindsNoCertificateInALocalRootPastItsBound(t *testing.T) {
	t.Parallel()

	server := newFakeServer(t)
	running := localhostOn(t, server)
	server.write(localRootAt, fakeCaddyRoot()+strings.Repeat("\n", maxLocalRootBytes))

	_, err := running.httpsRoots()

	wantRefused(t, err, localRootAt+" holds no certificate")
}

func TestTheHTTPSStepRefusesALocalhostWithoutTheCaddyUser(t *testing.T) {
	t.Parallel()

	synctest.Test(t, func(t *testing.T) {
		server := newFakeServer(t)
		running, _ := servedOn(t, server, firstSite(t, "localhost", 255))

		err := running.https(t.Context(), plannedStep(t, running, "https"))

		want := strings.Replace(httpsWarning("installer: user: unknown user caddy"), "https://example.com/",
			"https://localhost/", 1)
		if got := server.notes.String(); !errors.Is(err, errLeftOpen) || got != want {
			t.Errorf("https() = %v, noting %q, want the step left open and %q", err, got, want)
		}
	})
}

func TestTheHTTPSStepEndsAtOnceWhenTheRunEnds(t *testing.T) {
	t.Parallel()

	server := newFakeServer(t)
	running, _ := servedOn(t, server, firstSite(t, "example.com", 255))
	ctx, cancel := context.WithCancel(t.Context())
	cancel()

	err := running.https(ctx, plannedStep(t, running, "https"))

	if !errors.Is(err, context.Canceled) || server.notes.String() != "" {
		t.Errorf("https() = %v, noting %q, want the run's own end and no warning", err, server.notes.String())
	}
}

func TestTheHTTPSStepReturnsWhatStopsItsWarning(t *testing.T) {
	t.Parallel()

	synctest.Test(t, func(t *testing.T) {
		server := newFakeServer(t)
		running, _ := servedOn(t, server, firstSite(t, "example.com", 255))
		running.call.Stderr = refusingWriter{}

		err := running.https(t.Context(), plannedStep(t, running, "https"))

		wantRefused(t, err, "the terminal is gone")
	})
}

// wantedCommandsToKnow are the last lines of every summary.
const wantedCommandsToKnow = `Commands to know:
  sudo gophenberg update:status     shows the build that runs and the update on offer
  sudo gophenberg update:apply      installs the update on offer
  sudo journalctl -u gophenberg     shows what the site logged
  sudo gophenberg server:uninstall  removes Gophenberg and keeps its data
`

func TestTheSummaryNamesTheBuildTheAddressAndTheEmailOfTheAdministratorTheRunCreated(t *testing.T) {
	t.Parallel()

	server := newFakeServer(t)
	running, answer := servedOn(t, server, firstSite(t, "example.com", 255))
	server.pointCurrent()
	running.admin, running.created = mariaPerez, true

	err := running.summary(t.Context(), plannedStep(t, running, "summary"))

	want := "\nGophenberg 0.19.0 runs at https://example.com. Sign in at https://example.com/admin/ as " +
		"admin@example.com.\nThe hosting provider's firewall must let TCP 80, TCP 443 and UDP 443 in for visitors " +
		"to reach the site.\n" + wantedCommandsToKnow
	if err != nil || answer.String() != want {
		t.Errorf("summary() = %v, printing\n%s\nwant\n%s", err, answer.String(), want)
	}
}

func TestTheSummaryWithoutCaddyNamesTheSiteAddressAndSignsInOnceTheirOwnWebServerSendsTheDomainThere(t *testing.T) {
	t.Parallel()

	server := newFakeServer(t)
	moved := ownSite(t, "blog.example.com")
	moved.settings.Addr = netip.MustParseAddrPort("[::1]:8090")
	running, answer := servedOn(t, server, moved)
	server.pointCurrent()
	running.admin, running.created = mariaPerez, true

	err := running.summary(t.Context(), plannedStep(t, running, "summary"))

	want := "\nGophenberg 0.19.0 answers on [::1]:8090, the GOPHENBERG_ADDR the site listens on.\nOnce your own web " +
		"server sends blog.example.com there, sign in at https://blog.example.com/admin/ as admin@example.com.\n" +
		wantedCommandsToKnow
	if err != nil || answer.String() != want {
		t.Errorf("summary() = %v, printing\n%s\nwant\n%s", err, answer.String(), want)
	}
}

func TestTheSummaryOfARunThatCreatedNoAdministratorWithoutCaddyNamesNoEmailAndNoFirewall(t *testing.T) {
	t.Parallel()

	server := newFakeServer(t)
	running, answer := servedOn(t, server, ownSite(t, "example.com"))
	server.pointCurrent()

	err := running.summary(t.Context(), plannedStep(t, running, "summary"))

	want := "\nGophenberg 0.19.0 answers on 127.0.0.1:8081, the GOPHENBERG_ADDR the site listens on.\nOnce your own web " +
		"server sends example.com there, sign in at https://example.com/admin/.\n" + wantedCommandsToKnow
	if err != nil || answer.String() != want {
		t.Errorf("summary() = %v, printing\n%s\nwant\n%s", err, answer.String(), want)
	}
}

func TestTheSummaryStopsWhenCurrentNamesNoBuild(t *testing.T) {
	t.Parallel()

	server := newFakeServer(t)
	running, answer := servedOn(t, server, firstSite(t, "example.com", 255))

	err := running.summary(t.Context(), plannedStep(t, running, "summary"))

	if got := fmt.Sprint(err); !strings.HasPrefix(got, "updater: read the current link: ") || answer.Len() != 0 {
		t.Errorf("summary() = %v, printing %q, want the link refused and nothing printed", err, answer.String())
	}
}

func TestTheWebServerStepPrintsTheLinesItsDryRunPrints(t *testing.T) {
	t.Parallel()

	server := newFakeServer(t)
	running, answer := servedOn(t, server, ownSite(t, "example.com"))

	err := running.take(t.Context(), plannedStep(t, running, "web-server"))

	if err != nil || answer.String() != "\n"+wantedWebServerText {
		t.Errorf("take(web-server) = %v, printing\n%s\nwant\n%s", err, answer.String(), wantedWebServerText)
	}
}

// flaggedRunOn puts this installer's archive and the password file on server, and returns a run every flag answers.
func flaggedRunOn(t *testing.T, server *fakeServer) Request {
	t.Helper()
	server.write(archivePath, releaseArchiveOf(t, version.Version(), serverPlatform))
	server.write(passwordFile, goodPassword+"\n")
	return Request{
		Domain: "example.com", Archive: archivePath, AdminEmail: mariaPerez.email, AdminName: mariaPerez.name,
		AdminPasswordFile: passwordFile,
	}
}

func TestInstallWithoutDryRunTakesEveryStepAndEndsTheRecord(t *testing.T) {
	t.Parallel()

	server := newFakeServer(t)

	stdout, _, err := runInstall(server, flaggedRunOn(t, server))

	if err != nil {
		t.Fatalf("Install() = %v, want every step taken", err)
	}
	for n, does := range firstSteps {
		if line := fmt.Sprintf("Step %d: %s\n", n+1, does); !strings.Contains(stdout, line) {
			t.Errorf("Install() printed\n%s\nwant the line %q", stdout, line)
		}
	}
	for _, id := range stepIDs {
		wantMark(t, server, id, stepEnded)
	}
	if !slices.Equal(server.accounts, []listedAccount{{Email: "admin@example.com", Name: "Maria Perez", Role: "admin"}}) {
		t.Errorf("the run made the accounts %+v, want Maria Perez alone", server.accounts)
	}
}

// cutOffInAdmin returns a fake server an earlier run for example.com left cut off in the admin step, with accounts.
func cutOffInAdmin(t *testing.T, accounts ...listedAccount) *fakeServer {
	t.Helper()
	scenario := &installScenario{t: t, server: newFakeServer(t)}
	scenario.installEarlier()
	record, err := readRecord(scenario.server.machine())
	if err != nil {
		t.Fatalf("readRecord() = %v, want the earlier run's record", err)
	}
	record.Steps[stepAdmin] = stepStarted
	scenario.server.saveRecord(record)
	scenario.server.accounts = accounts
	return scenario.server
}

func TestInstallAsksNothingOnceAccountListShowsTheCutOffRunMadeAnAdministrator(t *testing.T) {
	t.Parallel()

	server := cutOffInAdmin(t, listedAccount{Email: "admin@example.com", Role: "admin"})
	answering(server, "admin@example.com")

	stdout, _, err := runInstall(server, Request{})

	if err != nil || strings.Contains(stdout, firstSteps[12]) {
		t.Errorf("Install() = %v, printing\n%s\nwant every step but the admin step", err, stdout)
	}
	wantNoTerminal(t, server)
	if server.createdAdmin() {
		t.Error("the run ran account:create-admin, want the administrator the cut-off run made kept")
	}
	wantMark(t, server, stepAdmin, stepEnded)
}

func TestInstallAsksForTheAdministratorWhenTheCutOffRunMadeNone(t *testing.T) {
	t.Parallel()

	server := cutOffInAdmin(t, listedAccount{Email: "editor@example.com", Role: "editor"})
	prompt := answering(server, "admin@example.com", "Maria Perez", goodPassword, goodPassword)

	_, _, err := runInstall(server, Request{})

	if err != nil || !server.createdAdmin() {
		t.Errorf("Install() = %v, creating an admin %t, want the questions asked and the admin made", err,
			server.createdAdmin())
	}
	wantAsked(t, prompt, allAsked)
}

func TestInstallReadsTheAccountsPastWhatTheSiteLogsBeforeThem(t *testing.T) {
	t.Parallel()

	server := cutOffInAdmin(t, listedAccount{Email: "admin@example.com", Role: "admin"})
	server.effects["gophenberg account:list"] = func(program Program) answer {
		listed := server.listAccounts(program)
		listed.printed = "time=2026-10-27T09:00:00Z level=INFO msg=\"opened {the database}\"\n" + listed.printed
		return listed
	}

	_, _, err := runInstall(server, Request{})

	if err != nil || server.createdAdmin() {
		t.Errorf("Install() = %v, creating an admin %t, want the listed admin found", err, server.createdAdmin())
	}
}

func TestInstallStopsWhenItCannotTellWhetherTheCutOffRunMadeTheAdministrator(t *testing.T) {
	t.Parallel()

	for printed, want := range map[string]string{
		"listing\n": "cannot tell whether the run cut off in the admin step made the first administrator: " +
			"account:list -json printed no accounts, so nothing was changed",
		"{\n  \"accounts\": [\n": "cannot tell whether the run cut off in the admin step made the first " +
			"administrator: account:list -json printed accounts that cannot be read: unexpected EOF, so nothing was " +
			"changed",
	} {
		server := cutOffInAdmin(t)
		server.effects["gophenberg account:list"] = func(Program) answer { return answer{printed: printed} }

		_, _, err := runInstall(server, Request{})

		wantRefused(t, err, want)
	}
}

func TestInstallStopsWhenAccountListFails(t *testing.T) {
	t.Parallel()

	server := cutOffInAdmin(t)
	server.effects["gophenberg account:list"] = func(program Program) answer {
		return failed(program.Name, "database: connection refused\n")
	}

	_, _, err := runInstall(server, Request{})

	wantRefused(t, err, "cannot tell whether the run cut off in the admin step made the first administrator: "+
		"installer: run /opt/gophenberg/current/gophenberg: exit status 1, after it printed:\ndatabase: connection "+
		"refused, so nothing was changed")
	wantRan(t, server, "systemctl --version", sandboxLine, clockLine,
		"runuser -u gophenberg -- /opt/gophenberg/current/gophenberg account:list -json")
	wantMark(t, server, stepAdmin, stepStarted)
}

func TestInstallStopsAtAStepThatFailsAndClosesWhatItOpened(t *testing.T) {
	t.Parallel()

	server := newFakeServer(t)
	server.effects["useradd"] = func(Program) answer { return failed("useradd", "useradd: cannot lock /etc/passwd\n") }

	_, _, err := runInstall(server, flaggedRunOn(t, server))

	wantRefused(t, err, "installer: run useradd: exit status 1, after it printed:\nuseradd: cannot lock /etc/passwd")
	wantMark(t, server, "user", stepStarted)
}

func TestInstallRefusesASiteEnvServeCannotReadBeforeAnyStep(t *testing.T) {
	t.Parallel()

	server := installedServer(t, "GOPHENBERG_SHUTDOWN_GRACE=soon\n")

	_, _, err := runInstall(server, flaggedRunOn(t, server))

	wantRefused(t, err, `/opt/gophenberg/.env: GOPHENBERG_SHUTDOWN_GRACE: must be a duration like 30s, got "soon"`)
	wantRan(t, server, "systemctl --version", sandboxLine, clockLine)
}

func TestInstallRefusesAServerItCannotOpenAsATree(t *testing.T) {
	t.Parallel()

	skipAsRoot(t)
	server := newFakeServer(t)
	request := flaggedRunOn(t, server)
	if err := os.Chmod(server.root, 0o311); err != nil {
		t.Fatalf("taking the right to list the top of the server: %v", err)
	}
	t.Cleanup(func() { _ = os.Chmod(server.root, 0o755) })

	_, _, err := runInstall(server, request)

	if !errors.Is(err, fs.ErrPermission) || server.machine().there(recordPath) {
		t.Errorf("Install() = %v, want the server it cannot open refused before any change", err)
	}
}
