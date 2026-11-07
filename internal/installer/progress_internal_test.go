// SPDX-License-Identifier: Apache-2.0

package installer

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"testing"
	"testing/synctest"
	"time"

	"github.com/gopherium/framework/gonsole"
)

// aptGetInstall is apt-get install as the packages step runs it, shown by its short line.
var aptGetInstall = aptProgram("apt-get", "-o", "DPkg::Lock::Timeout=300", "install", "-y", "--no-install-recommends",
	"postgresql", "caddy").shownAs("apt-get install postgresql caddy")

// printing is a piece of text a slow program prints once it has run for at.
type printing struct {
	at   time.Duration
	text string
}

// slowRun returns a Run whose program prints each of printed at its time, copying it to its output, then ends at end.
func slowRun(end time.Duration, printed ...printing) func(context.Context, Program) (string, error) {
	return func(_ context.Context, program Program) (string, error) {
		started := time.Now()
		var all strings.Builder
		for _, piece := range printed {
			time.Sleep(piece.at - time.Since(started))
			all.WriteString(piece.text)
			copyPrinted(program, piece.text)
		}
		time.Sleep(end - time.Since(started))
		return all.String(), nil
	}
}

// watchedRun returns a run that prints a still-running line every interval to its answer, which it returns too.
func watchedRun(interval time.Duration) (*installing, *strings.Builder) {
	answer := &strings.Builder{}
	running := &installing{call: gonsole.Call{Stdout: answer}}
	running.site.settings.Checks.ProgressInterval = interval
	return running, answer
}

// stillRunning returns the line a run prints for apt-get install once it ran for after, last its last line printed.
func stillRunning(after, last string) string {
	return "  still running apt-get install postgresql caddy after " + after + ": " + last + "\n"
}

func TestARunNamesEachProgramUnderItsStepBeforeItRunsIt(t *testing.T) {
	t.Parallel()

	for _, named := range []struct {
		program Program
		want    string
	}{
		{aptGetInstall, "  apt-get install postgresql caddy\n"},
		{siteCommand("/opt/gophenberg", "check"), "  gophenberg check\n"},
		{unitProgram("restart", caddyUnit), "  systemctl restart caddy.service\n"},
		{aptProgram("dpkg", "--configure", "-a"), "  dpkg --configure -a\n"},
	} {
		running, answer := watchedRun(30 * time.Second)
		var before string
		running.machine.Run = func(context.Context, Program) (string, error) {
			before = answer.String()
			return "", nil
		}

		_, err := running.runNamed(t.Context(), named.program)

		if err != nil || before != named.want || answer.String() != named.want {
			t.Errorf("runNamed(%s) = %v, printing %q before the program and %q in all, want %q both times",
				named.program.Name, err, before, answer.String(), named.want)
		}
	}
}

func TestARunPrintsNoStillRunningLineForAProgramThatEndsWithinTheInterval(t *testing.T) {
	t.Parallel()

	synctest.Test(t, func(t *testing.T) {
		running, answer := watchedRun(30 * time.Second)
		running.machine.Run = slowRun(29*time.Second, printing{time.Second, "Reading package lists...\n"})

		printed, err := running.runNamed(t.Context(), aptGetInstall)

		if err != nil || printed != "Reading package lists...\n" {
			t.Errorf("runNamed() = %q, %v, want what the program printed", printed, err)
		}
		if want := "  apt-get install postgresql caddy\n"; answer.String() != want {
			t.Errorf("the run printed %q, want only %q", answer.String(), want)
		}
	})
}

func TestARunPrintsAStillRunningLineEveryIntervalWithTheLastLineThatHoldsText(t *testing.T) {
	t.Parallel()

	synctest.Test(t, func(t *testing.T) {
		setUp := "Setting up postgresql-16 (16.10-0ubuntu0.24.04.1) ..."
		running, answer := watchedRun(30 * time.Second)
		running.machine.Run = slowRun(95*time.Second, printing{10 * time.Second, "Reading package lists...\n"},
			printing{40 * time.Second, setUp + "\n\n   \n"})

		printed, err := running.runNamed(t.Context(), aptGetInstall)

		if want := "Reading package lists...\n" + setUp + "\n\n   \n"; err != nil || printed != want {
			t.Errorf("runNamed() = %q, %v, want everything the program printed, %q", printed, err, want)
		}
		want := "  apt-get install postgresql caddy\n" + stillRunning("30s", "Reading package lists...") +
			stillRunning("1m0s", setUp) + stillRunning("1m30s", setUp)
		if answer.String() != want {
			t.Errorf("the run printed\n%s\nwant\n%s", answer.String(), want)
		}
	})
}

func TestAStillRunningLineLeavesOutTheLastLineWhileTheProgramPrintedNoText(t *testing.T) {
	t.Parallel()

	for _, printed := range [][]printing{nil, {{time.Second, "\n  \n"}}} {
		synctest.Test(t, func(t *testing.T) {
			running, answer := watchedRun(30 * time.Second)
			running.machine.Run = slowRun(35*time.Second, printed...)

			_, err := running.runNamed(t.Context(), aptGetInstall)

			want := "  apt-get install postgresql caddy\n  still running apt-get install postgresql caddy after 30s\n"
			if err != nil || answer.String() != want {
				t.Errorf("runNamed() = %v, printing %q, want %q", err, answer.String(), want)
			}
		})
	}
}

func TestAStillRunningLineEndsALineAtACarriageReturnAndShowsEachControlCharacterAsASpace(t *testing.T) {
	t.Parallel()

	for printed, last := range map[string]string{
		"Waiting for cache lock: 1s\rWaiting for cache lock: 2s\r": "Waiting for cache lock: 2s",
		"Unpacking caddy\x1b[2K\t(2.10.2) ...\n":                   "Unpacking caddy [2K (2.10.2) ...",
	} {
		synctest.Test(t, func(t *testing.T) {
			running, answer := watchedRun(30 * time.Second)
			running.machine.Run = slowRun(35*time.Second, printing{time.Second, printed})

			_, err := running.runNamed(t.Context(), aptGetInstall)

			want := "  apt-get install postgresql caddy\n" + stillRunning("30s", last)
			if err != nil || answer.String() != want {
				t.Errorf("runNamed() = %v, printing %q, want %q", err, answer.String(), want)
			}
		})
	}
}

func TestAStillRunningLineCutsALastLineLongerThan160Characters(t *testing.T) {
	t.Parallel()

	for printed, last := range map[string]string{
		strings.Repeat("é", 300):                  strings.Repeat("é", 160) + "...",
		strings.Repeat("x", 160):                  strings.Repeat("x", 160),
		"Get:1 " + strings.Repeat("y", 154) + "z": "Get:1 " + strings.Repeat("y", 154) + "...",
	} {
		synctest.Test(t, func(t *testing.T) {
			running, answer := watchedRun(30 * time.Second)
			running.machine.Run = slowRun(35*time.Second, printing{time.Second, printed + "\n"})

			_, err := running.runNamed(t.Context(), aptGetInstall)

			want := "  apt-get install postgresql caddy\n" + stillRunning("30s", last)
			if err != nil || answer.String() != want {
				t.Errorf("runNamed() = %v, printing %q, want %q", err, answer.String(), want)
			}
		})
	}
}

func TestAProgressIntervalOfZeroPrintsNoStillRunningLine(t *testing.T) {
	t.Parallel()

	synctest.Test(t, func(t *testing.T) {
		running, answer := watchedRun(0)
		running.machine.Run = slowRun(95*time.Second, printing{time.Second, "Reading package lists...\n"})

		printed, err := running.runNamed(t.Context(), aptGetInstall)

		if err != nil || printed != "Reading package lists...\n" {
			t.Errorf("runNamed() = %q, %v, want what the program printed", printed, err)
		}
		if want := "  apt-get install postgresql caddy\n"; answer.String() != want {
			t.Errorf("the run printed %q, want only %q", answer.String(), want)
		}
	})
}

func TestARunStopsWithEverythingASlowProgramPrintedWhenItFails(t *testing.T) {
	t.Parallel()

	synctest.Test(t, func(t *testing.T) {
		missing := "E: Unable to locate package caddy"
		running, answer := watchedRun(30 * time.Second)
		slow := slowRun(35*time.Second, printing{time.Second, "Reading package lists...\n" + missing + "\n"})
		running.machine.Run = func(ctx context.Context, program Program) (string, error) {
			printed, _ := slow(ctx, program)
			return printed, errors.New("installer: run apt-get: exit status 100")
		}

		err := running.runStep(t.Context(), aptGetInstall)

		wantRefused(t, err, "installer: run apt-get: exit status 100, after it printed:\nReading package lists...\n"+
			missing)
		if want := "  apt-get install postgresql caddy\n" + stillRunning("30s", missing); answer.String() != want {
			t.Errorf("the run printed %q, want %q", answer.String(), want)
		}
	})
}

func TestARunPrintsAStillRunningLineEveryIntervalItIsGiven(t *testing.T) {
	t.Parallel()

	synctest.Test(t, func(t *testing.T) {
		running, answer := watchedRun(10 * time.Second)
		running.machine.Run = slowRun(25*time.Second, printing{time.Second, "Reading package lists...\n"})

		_, err := running.runNamed(t.Context(), aptGetInstall)

		want := "  apt-get install postgresql caddy\n" + stillRunning("10s", "Reading package lists...") +
			stillRunning("20s", "Reading package lists...")
		if err != nil || answer.String() != want {
			t.Errorf("runNamed() = %v, printing\n%s\nwant\n%s", err, answer.String(), want)
		}
	})
}

func TestARunStartsNoProgramItCannotName(t *testing.T) {
	t.Parallel()

	running, _ := watchedRun(30 * time.Second)
	running.call.Stdout = refusingWriter{}
	ran := false
	running.machine.Run = func(context.Context, Program) (string, error) {
		ran = true
		return "", nil
	}

	_, err := running.runNamed(t.Context(), aptGetInstall)

	if err == nil || err.Error() != "the terminal is gone" || ran {
		t.Errorf("runNamed() = %v, the program ran %t, want the terminal gone and no program started", err, ran)
	}
}

// startLines are the lines the start step prints for the programs it runs before it waits for the site.
const startLines = "  systemctl reset-failed gophenberg.service\n  systemctl start gophenberg.service\n"

// refusedSite is what the start step's last try got from a site that does not listen yet.
const refusedSite = "it could not ask for /admin/: dial tcp 127.0.0.1:8081: connect: connection refused"

// stillWaiting returns the line the HTTPS check of example.com prints once it waited for after, last its last try.
func stillWaiting(after, last string) string {
	return "  still waiting for https://example.com/admin/ after " + after + ": " + last + "\n"
}

// answerHTTPSAfter has every HTTPS fetch on server fail with refused until after has passed, then get Caddy's page.
func answerHTTPSAfter(server *fakeServer, after time.Duration, refused error) {
	started := time.Now()
	server.running[caddyUnit] = true
	server.answerHTTPS = func(request *http.Request) (*http.Response, error) {
		if time.Since(started) < after {
			return nil, refused
		}
		return server.caddyPage(request)
	}
}

func TestTheHTTPSStepPrintsAStillWaitingLineEveryIntervalWithWhatTheLastTryGot(t *testing.T) {
	t.Parallel()

	synctest.Test(t, func(t *testing.T) {
		server := newFakeServer(t)
		running, answer := servedOn(t, server, firstSite(t, "example.com", 255))
		server.running[caddyUnit] = true
		started := time.Now()
		server.answerHTTPS = func(request *http.Request) (*http.Response, error) {
			switch waited := time.Since(started); {
			case waited < 40*time.Second:
				return nil, errors.New(noCertificateYet)
			case waited < 75*time.Second:
				return answered(http.StatusBadGateway, "")(request)
			}
			return server.caddyPage(request)
		}

		err := running.take(t.Context(), plannedStep(t, running, "https"))

		want := stillWaiting("30s", "it could not be reached: "+noCertificateYet) +
			stillWaiting("1m0s", "it answered 502")
		if err != nil || answer.String() != want {
			t.Errorf("take(https) = %v, printing\n%s\nwant\n%s", err, answer.String(), want)
		}
		wantMark(t, server, "https", stepEnded)
	})
}

func TestTheStartStepPrintsAStillWaitingLineEveryIntervalWithWhatTheLastTryGot(t *testing.T) {
	t.Parallel()

	synctest.Test(t, func(t *testing.T) {
		server := newFakeServer(t)
		running, answer := servedOn(t, server, firstSite(t, "example.com", 255))
		started := time.Now()
		server.answerFetch = func(request *http.Request) (*http.Response, error) {
			if time.Since(started) < 45*time.Second {
				return nil, errors.New("dial tcp 127.0.0.1:8081: connect: connection refused")
			}
			return server.sitePage(request)
		}

		err := running.take(t.Context(), plannedStep(t, running, stepStart))

		want := startLines + "  still waiting for the site to start after 30s: " + refusedSite + "\n"
		if err != nil || answer.String() != want {
			t.Errorf("take(start) = %v, printing\n%s\nwant\n%s", err, answer.String(), want)
		}
	})
}

func TestAStillWaitingLineLeavesOutTheLastTryWhileNoTryEnded(t *testing.T) {
	t.Parallel()

	synctest.Test(t, func(t *testing.T) {
		server := newFakeServer(t)
		running, answer := servedOn(t, server, firstSite(t, "example.com", 255))
		slept := false
		server.answerFetch = func(request *http.Request) (*http.Response, error) {
			if !slept {
				slept = true
				time.Sleep(40 * time.Second)
			}
			return server.sitePage(request)
		}

		err := running.take(t.Context(), plannedStep(t, running, stepStart))

		if want := startLines + "  still waiting for the site to start after 30s\n"; err != nil || answer.String() != want {
			t.Errorf("take(start) = %v, printing\n%s\nwant\n%s", err, answer.String(), want)
		}
	})
}

func TestAStillWaitingLineShowsTheLastTryOnOneLineCutAfter160Characters(t *testing.T) {
	t.Parallel()

	synctest.Test(t, func(t *testing.T) {
		server := newFakeServer(t)
		running, answer := servedOn(t, server, firstSite(t, "example.com", 255))
		answerHTTPSAfter(server, 35*time.Second, errors.New("remote error:\ttls:\ninternal error "+strings.Repeat("y", 200)))

		err := running.take(t.Context(), plannedStep(t, running, "https"))

		shown := "it could not be reached: remote error: tls: internal error " + strings.Repeat("y", 200)
		if want := stillWaiting("30s", shown[:160]+"..."); err != nil || answer.String() != want {
			t.Errorf("take(https) = %v, printing\n%s\nwant\n%s", err, answer.String(), want)
		}
	})
}

func TestAWaitThatRunsOutPrintsNoStillWaitingLineAtTheMomentItGivesUp(t *testing.T) {
	t.Parallel()

	synctest.Test(t, func(t *testing.T) {
		server := newFakeServer(t)
		running, answer := servedOn(t, server, firstSite(t, "example.com", 255))
		running.site.settings.Checks.ProgressInterval = 30 * time.Second
		running.site.settings.Checks.HTTPSWait = 2 * time.Minute
		answerHTTPSAfter(server, time.Hour, errors.New(noCertificateYet))

		_ = running.take(t.Context(), plannedStep(t, running, "https"))

		last := "it could not be reached: " + noCertificateYet
		want := stillWaiting("30s", last) + stillWaiting("1m0s", last) + stillWaiting("1m30s", last)
		if answer.String() != want {
			t.Errorf("take(https) printed\n%s\nwant\n%s", answer.String(), want)
		}
	})
}

func TestAWaitPrintsAStillWaitingLineEveryIntervalItIsGiven(t *testing.T) {
	t.Parallel()

	synctest.Test(t, func(t *testing.T) {
		server := newFakeServer(t)
		running, answer := servedOn(t, server, firstSite(t, "example.com", 255))
		running.site.settings.Checks.ProgressInterval = 10 * time.Second
		answerHTTPSAfter(server, 25*time.Second, errors.New(noCertificateYet))

		err := running.take(t.Context(), plannedStep(t, running, "https"))

		last := "it could not be reached: " + noCertificateYet
		if want := stillWaiting("10s", last) + stillWaiting("20s", last); err != nil || answer.String() != want {
			t.Errorf("take(https) = %v, printing\n%s\nwant\n%s", err, answer.String(), want)
		}
	})
}

func TestAProgressIntervalOfZeroPrintsNoStillWaitingLine(t *testing.T) {
	t.Parallel()

	synctest.Test(t, func(t *testing.T) {
		server := newFakeServer(t)
		running, answer := servedOn(t, server, firstSite(t, "example.com", 255))
		running.site.settings.Checks.ProgressInterval = 0
		answerHTTPSAfter(server, 75*time.Second, errors.New(noCertificateYet))

		err := running.take(t.Context(), plannedStep(t, running, "https"))

		if err != nil || answer.String() != "" {
			t.Errorf("take(https) = %v, printing %q, want nothing printed", err, answer.String())
		}
		wantMark(t, server, "https", stepEnded)
	})
}
