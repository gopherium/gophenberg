// SPDX-License-Identifier: Apache-2.0

package installer

import (
	"fmt"
	"net/netip"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/gopherium/framework/gonsole"
	"github.com/gopherium/framework/gonsole/testkit"

	"github.com/gopherium/gophenberg/internal/updater"
)

// firstServerEnv is the server.env a first run on example.com writes with every default.
const firstServerEnv = `# Written by gophenberg server:install.
GOPHENBERG_INSTALL_ROOT=/opt/gophenberg
GOPHENBERG_PUBLIC_URL=https://example.com
GOPHENBERG_ADDR=127.0.0.1:8081
GOPHENBERG_INSTALL_WEB_SERVER=caddy
GOPHENBERG_INSTALL_CERTIFICATE_EMAIL=admin@example.com
GOPHENBERG_SERVICE_RESTART_DELAY=5s
GOPHENBERG_SERVICE_START_LIMIT_INTERVAL=300s
GOPHENBERG_SERVICE_START_LIMIT_BURST=10
GOPHENBERG_SERVICE_STOP_MARGIN=5s
GOPHENBERG_MAINTENANCE_RETRY_AFTER=600s
GOPHENBERG_PROXY_RETRY_DURATION=10s
`

// webServerMove is how a later run that asks for the other web server is refused, after the two names.
const webServerMove = "GOPHENBERG_INSTALL_WEB_SERVER: this server was set up with %s, and a later run cannot move it " +
	"to %s, so run sudo gophenberg server:uninstall first, then change the line in /etc/gophenberg/server.env"

// wantedHost is how a refused domain says what a host name must be.
const wantedHost = "localhost or a host name such as example.com: labels of 1 to 63 lower-case letters, digits and " +
	"hyphens joined by dots, no hyphen at either end of a label, a letter in the last label, 253 characters at most"

// envOf returns the settings values holds under the GOPHENBERG_ prefix.
func envOf(values map[string]string) gonsole.Env {
	return gonsole.Env{Prefix: "GOPHENBERG_", Getenv: testkit.Getenv(values)}
}

// serverWith returns a machine whose root holds files, each named by its path from the top of the server.
func serverWith(t *testing.T, files map[string]string) Machine {
	t.Helper()
	root := t.TempDir()
	for path, body := range files {
		full := filepath.Join(root, path)
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatalf("making the folder of %s: %v", path, err)
		}
		if err := os.WriteFile(full, []byte(body), 0o600); err != nil {
			t.Fatalf("writing %s: %v", path, err)
		}
	}
	return Machine{Root: root}
}

// firstRun is the request of a first run on example.com with the first administrator's email.
var firstRun = Request{Domain: "example.com", AdminEmail: "admin@example.com", DryRun: true}

// wantRefused fails t unless err reads want.
func wantRefused(t *testing.T, err error, want string) {
	t.Helper()
	if err == nil || err.Error() != want {
		t.Errorf("error = %v, want %q", err, want)
	}
}

func TestSettingsOnAFirstRunTakeEveryDefaultAndWriteItIntoServerEnv(t *testing.T) {
	t.Parallel()

	settings, err := settingsOn(serverWith(t, nil), envOf(nil), firstRun)

	if err != nil {
		t.Fatalf("settingsOn() = %v, want the defaults", err)
	}
	if settings.serverEnv != firstServerEnv {
		t.Errorf("server.env would get\n%s\nwant\n%s", settings.serverEnv, firstServerEnv)
	}
	settings.serverEnv = ""
	want := Settings{
		Root: "/opt/gophenberg", Domain: "example.com", Addr: netip.MustParseAddrPort("127.0.0.1:8081"),
		WebServer: "caddy", CertificateEmail: "admin@example.com",
		Restarts: Restarts{
			Delay: 5 * time.Second, LimitInterval: 300 * time.Second, LimitBurst: 10, StopMargin: 5 * time.Second,
			RetryAfter: 600 * time.Second, ProxyRetry: 10 * time.Second,
		},
		Checks: Checks{
			MemoryFloorMB: 900, DiskFloorMB: 1024, AptLockWait: 5 * time.Minute, DomainCheckTimeout: 10 * time.Second,
			HTTPSWait: 2 * time.Minute, PollInterval: time.Second, StartWait: time.Minute,
			ProgressInterval: 30 * time.Second,
		},
	}
	if settings != want {
		t.Errorf("settingsOn() = %+v, want %+v", settings, want)
	}
}

func TestSettingsOnWaitForTheFirstStartAsLongAsServeChecksANewBuild(t *testing.T) {
	t.Parallel()

	for _, values := range []map[string]string{
		{"GOPHENBERG_INSTALL_ROOT": "/opt/gophenberg"},
		{"GOPHENBERG_INSTALL_ROOT": "/opt/gophenberg", "GOPHENBERG_UPDATE_VERIFY_TIMEOUT": "90s"},
	} {
		served, err := updater.SettingsFrom(envOf(values))
		if err != nil {
			t.Fatalf("updater.SettingsFrom() = %v, want the update settings", err)
		}

		settings, err := settingsOn(serverWith(t, nil), envOf(values), firstRun)

		if err != nil || settings.Checks.StartWait != served.VerifyTimeout {
			t.Errorf("settingsOn() waits %s for the first start with %v, want the %s serve checks a build within",
				settings.Checks.StartWait, err, served.VerifyTimeout)
		}
	}
}

func TestSettingsOnRefuseAStartWaitServeCannotRead(t *testing.T) {
	t.Parallel()

	env := envOf(map[string]string{"GOPHENBERG_UPDATE_VERIFY_TIMEOUT": "soon"})

	_, err := settingsOn(serverWith(t, nil), env, firstRun)

	wantRefused(t, err, `GOPHENBERG_UPDATE_VERIFY_TIMEOUT: must be a duration like 30s, got "soon"`)
}

func TestSettingsOnRefuseAPasswordFileThatIsNotAnAbsolutePath(t *testing.T) {
	t.Parallel()

	request := firstRun
	request.AdminPasswordFile = "root/admin-password"

	_, err := settingsOn(Machine{}, envOf(nil), request)

	wantRefused(t, err, `-admin-password-file: must be an absolute path, got "root/admin-password"`)
}

func TestSettingsOnAFirstRunTakeTheFlagThenTheEnvironmentThenTheDefault(t *testing.T) {
	t.Parallel()

	env := envOf(map[string]string{
		"GOPHENBERG_PUBLIC_URL": "https://elsewhere.example.com", "GOPHENBERG_SERVICE_RESTART_DELAY": " 3s ",
		"GOPHENBERG_INSTALL_CERTIFICATE_EMAIL": "maria@example.com", "GOPHENBERG_INSTALL_MEMORY_FLOOR_MB": "800",
		"GOPHENBERG_INSTALL_ROOT": "",
	})

	settings, err := settingsOn(serverWith(t, nil), env, firstRun)

	if err != nil {
		t.Fatalf("settingsOn() = %v, want the settings taken", err)
	}
	want := strings.NewReplacer(
		"RESTART_DELAY=5s", "RESTART_DELAY=3s", "EMAIL=admin@example.com", "EMAIL=maria@example.com",
	).Replace(firstServerEnv)
	if settings.serverEnv != want {
		t.Errorf("server.env would get\n%s\nwant\n%s", settings.serverEnv, want)
	}
	if got := [3]any{settings.Domain, settings.Restarts.Delay, settings.Checks.MemoryFloorMB}; got != [3]any{
		"example.com", 3 * time.Second, 800,
	} {
		t.Errorf("the domain, the restart delay and the memory floor are %v, want the flag's and the environment's", got)
	}
}

func TestSettingsOnALaterRunKeepThePersonsLinesAndReplaceTheOnesAFlagOrTheEnvironmentGives(t *testing.T) {
	t.Parallel()

	machine := serverWith(t, map[string]string{serverEnvPath: `# Written by gophenberg server:install.
# Maria Perez's note, which a later run drops.
GOPHENBERG_INSTALL_ROOT=/srv/gophenberg/
GOPHENBERG_PUBLIC_URL=https://example.com
GOPHENBERG_ADDR=[::1]:8090
GOPHENBERG_INSTALL_CERTIFICATE_EMAIL=maria@example.com
GOPHENBERG_SERVICE_RESTART_DELAY=2s
GOPHENBERG_SERVICE_START_LIMIT_BURST=10
GOPHENBERG_INSTALL_MEMORY_FLOOR_MB=800
GOPHENBERG_INSTALL_DISK_FLOOR_MB=2048
GOPHENBERG_SITE_TITLE="Maria Perez's $site"
`})
	env := envOf(map[string]string{
		"GOPHENBERG_SERVICE_START_LIMIT_BURST": "20", "GOPHENBERG_INSTALL_DISK_FLOOR_MB": "4096",
	})

	settings, err := settingsOn(machine, env, Request{Domain: "blog.example.com", AdminEmail: "admin@example.com"})

	if err != nil {
		t.Fatalf("settingsOn() = %v, want the settings taken", err)
	}
	want := `# Written by gophenberg server:install.
GOPHENBERG_INSTALL_ROOT=/srv/gophenberg/
GOPHENBERG_PUBLIC_URL=https://blog.example.com
GOPHENBERG_ADDR=[::1]:8090
GOPHENBERG_INSTALL_WEB_SERVER=caddy
GOPHENBERG_INSTALL_CERTIFICATE_EMAIL=maria@example.com
GOPHENBERG_SERVICE_RESTART_DELAY=2s
GOPHENBERG_SERVICE_START_LIMIT_INTERVAL=300s
GOPHENBERG_SERVICE_START_LIMIT_BURST=20
GOPHENBERG_SERVICE_STOP_MARGIN=5s
GOPHENBERG_MAINTENANCE_RETRY_AFTER=600s
GOPHENBERG_PROXY_RETRY_DURATION=10s
GOPHENBERG_INSTALL_DISK_FLOOR_MB=4096
GOPHENBERG_INSTALL_MEMORY_FLOOR_MB=800
GOPHENBERG_SITE_TITLE="Maria Perez's \$site"
`
	if settings.serverEnv != want {
		t.Errorf("server.env would get\n%s\nwant\n%s", settings.serverEnv, want)
	}
	if got := [4]any{settings.Root, settings.Addr, settings.Domain, settings.Checks.DiskFloorMB}; got != [4]any{
		"/srv/gophenberg", netip.MustParseAddrPort("[::1]:8090"), "blog.example.com", 4096,
	} {
		t.Errorf("the root, the address, the domain and the disk floor are %v, want those of the file, the flag "+
			"and the environment", got)
	}
}

func TestSettingsOnALaterRunTakeTheDomainServerEnvNames(t *testing.T) {
	t.Parallel()

	machine := serverWith(t, map[string]string{serverEnvPath: "GOPHENBERG_PUBLIC_URL=https://localhost\n"})

	settings, err := settingsOn(machine, envOf(nil), Request{})

	if err != nil || settings.Domain != "localhost" {
		t.Errorf("settingsOn() = %q, %v, want the domain localhost of server.env", settings.Domain, err)
	}
}

func TestSettingsOnAFirstRunWithWebServerNoneKeepItInServerEnv(t *testing.T) {
	t.Parallel()

	request := firstRun
	request.WebServer = "none"

	settings, err := settingsOn(serverWith(t, nil), envOf(nil), request)

	want := strings.Replace(firstServerEnv, "WEB_SERVER=caddy", "WEB_SERVER=none", 1)
	if err != nil || settings.serverEnv != want {
		t.Errorf("settingsOn() = %v, server.env would get\n%s\nwant\n%s", err, settings.serverEnv, want)
	}
	if settings.WebServer != "none" {
		t.Errorf("settingsOn() takes the web server %q, want none", settings.WebServer)
	}
}

func TestSettingsOnAFirstRunTakeTheWebServerTheEnvironmentNames(t *testing.T) {
	t.Parallel()

	env := envOf(map[string]string{"GOPHENBERG_INSTALL_WEB_SERVER": " none "})

	settings, err := settingsOn(serverWith(t, nil), env, firstRun)

	if err != nil || settings.WebServer != "none" {
		t.Errorf("settingsOn() = %q, %v, want the web server none of the environment", settings.WebServer, err)
	}
}

func TestSettingsOnALaterRunKeepTheWebServerServerEnvNames(t *testing.T) {
	t.Parallel()

	for _, kept := range []string{"caddy", "none"} {
		machine := serverWith(t, map[string]string{serverEnvPath: "GOPHENBERG_PUBLIC_URL=https://example.com\n" +
			"GOPHENBERG_INSTALL_WEB_SERVER=" + kept + "\n"})

		for _, request := range []Request{{}, {WebServer: kept}} {
			settings, err := settingsOn(machine, envOf(nil), request)

			if err != nil || settings.WebServer != kept {
				t.Errorf("settingsOn(%+v) = %q, %v, want the web server %s of server.env", request, settings.WebServer,
					err, kept)
			}
		}
	}
}

func TestSettingsOnRefuseALaterRunThatAsksForTheOtherWebServer(t *testing.T) {
	t.Parallel()

	for kept, asked := range map[string]string{"caddy": "none", "none": "caddy"} {
		machine := serverWith(t, map[string]string{serverEnvPath: "GOPHENBERG_PUBLIC_URL=https://example.com\n" +
			"GOPHENBERG_INSTALL_WEB_SERVER=" + kept + "\n"})
		byEnvironment := envOf(map[string]string{"GOPHENBERG_INSTALL_WEB_SERVER": asked})

		_, byFlag := settingsOn(machine, envOf(nil), Request{WebServer: asked})
		_, byEnv := settingsOn(machine, byEnvironment, Request{})

		wantRefused(t, byFlag, fmt.Sprintf(webServerMove, kept, asked))
		wantRefused(t, byEnv, fmt.Sprintf(webServerMove, kept, asked))
	}
}

func TestSettingsOnRefuseALaterRunThatAsksForAnotherInstallRoot(t *testing.T) {
	t.Parallel()

	machine := serverWith(t, map[string]string{serverEnvPath: firstServerEnv})
	env := envOf(map[string]string{"GOPHENBERG_INSTALL_ROOT": "/srv/gophenberg/"})

	_, err := settingsOn(machine, env, Request{})

	wantRefused(t, err, "GOPHENBERG_INSTALL_ROOT: this server was set up with /opt/gophenberg, and a later run cannot "+
		"move it to /srv/gophenberg, so run sudo gophenberg server:uninstall first, then change the line in "+
		"/etc/gophenberg/server.env")
}

func TestSettingsOnALaterRunTakeTheInstallRootServerEnvKeepsWrittenWithATrailingSlash(t *testing.T) {
	t.Parallel()

	machine := serverWith(t, map[string]string{serverEnvPath: firstServerEnv})
	env := envOf(map[string]string{"GOPHENBERG_INSTALL_ROOT": "/opt/gophenberg/"})

	settings, err := settingsOn(machine, env, Request{})

	if err != nil || settings.Root != "/opt/gophenberg" {
		t.Errorf("settingsOn() = %q, %v, want the install root /opt/gophenberg of server.env", settings.Root, err)
	}
}

func TestSettingsOnALaterRunRefuseAnInstallRootTheEnvironmentNamesByItsRuleBeforeAnyMove(t *testing.T) {
	t.Parallel()

	machine := serverWith(t, map[string]string{serverEnvPath: firstServerEnv})
	env := envOf(map[string]string{"GOPHENBERG_INSTALL_ROOT": "srv/gophenberg"})

	_, err := settingsOn(machine, env, Request{})

	wantRefused(t, err, `GOPHENBERG_INSTALL_ROOT: must be an absolute path, got "srv/gophenberg"`)
}

func TestSettingsOnALaterRunRefuseAWebServerTheEnvironmentNamesThatIsNeitherCaddyNorNone(t *testing.T) {
	t.Parallel()

	machine := serverWith(t, map[string]string{serverEnvPath: firstServerEnv})
	env := envOf(map[string]string{"GOPHENBERG_INSTALL_WEB_SERVER": "nginx"})

	_, err := settingsOn(machine, env, Request{})

	wantRefused(t, err, `GOPHENBERG_INSTALL_WEB_SERVER: must be caddy or none, got "nginx"`)
}

func TestSettingsOnALaterRunTakeTheWebServerTheFlagNamesOverALineOfServerEnvThatNamesNeither(t *testing.T) {
	t.Parallel()

	machine := serverWith(t, map[string]string{
		serverEnvPath: strings.Replace(firstServerEnv, "WEB_SERVER=caddy", "WEB_SERVER=Caddy", 1),
	})

	settings, err := settingsOn(machine, envOf(nil), Request{WebServer: "caddy"})

	if err != nil || settings.WebServer != "caddy" || settings.serverEnv != firstServerEnv {
		t.Errorf("settingsOn() = %q, %v, server.env would get\n%s\nwant the web server caddy of the flag and\n%s",
			settings.WebServer, err, settings.serverEnv, firstServerEnv)
	}
}

func TestSettingsOnALaterRunWhoseServerEnvNamesNoWebServerTakeCaddy(t *testing.T) {
	t.Parallel()

	machine := serverWith(t, map[string]string{serverEnvPath: "GOPHENBERG_PUBLIC_URL=https://example.com\n"})

	settings, err := settingsOn(machine, envOf(nil), Request{WebServer: "none"})

	if err != nil || settings.WebServer != "none" {
		t.Errorf("settingsOn() = %q, %v, want the web server none the flag names", settings.WebServer, err)
	}
	if settings, err := settingsOn(machine, envOf(nil), Request{}); err != nil || settings.WebServer != "caddy" {
		t.Errorf("settingsOn() = %q, %v, want the default web server caddy", settings.WebServer, err)
	}
}

func TestSettingsOnRefuseAWebServerFlagTheyDoNotKnowBeforeTheyReadServerEnv(t *testing.T) {
	t.Parallel()

	unreadable := serverWith(t, nil)
	if err := os.MkdirAll(filepath.Join(unreadable.Root, serverEnvPath), 0o755); err != nil {
		t.Fatalf("making a folder where server.env goes: %v", err)
	}
	for _, webServer := range []string{"nginx", "Caddy", "caddy ", "apache"} {
		_, err := settingsOn(unreadable, envOf(nil), Request{Domain: "example.com", WebServer: webServer})

		wantRefused(t, err, `-web-server: must be caddy or none, got "`+webServer+`"`)
	}
}

func TestSettingsOnTakeEveryHostNameTheRuleAllows(t *testing.T) {
	t.Parallel()

	label := strings.Repeat("a", 63)
	longest := strings.Join([]string{label, label, label, strings.Repeat("b", 61)}, ".")
	for _, domain := range []string{
		"localhost", "example.com", "a.b", "x-1.example.com", "0.example.com", "203.0.113.7.example.com", "a.b1",
		longest,
	} {
		settings, err := settingsOn(serverWith(t, nil), envOf(nil), Request{Domain: domain})

		if err != nil || settings.Domain != domain {
			t.Errorf("settingsOn() of the domain %q = %q, %v, want it taken", domain, settings.Domain, err)
		}
	}
}

func TestSettingsOnRefuseADomainThatIsNotAHostNameBeforeTheyReadServerEnv(t *testing.T) {
	t.Parallel()

	unreadable := serverWith(t, nil)
	if err := os.MkdirAll(filepath.Join(unreadable.Root, serverEnvPath), 0o755); err != nil {
		t.Fatalf("making a folder where server.env goes: %v", err)
	}
	label := strings.Repeat("a", 63)
	for _, domain := range []string{
		"not a host name", "Example.com", "gophenberg", "-blog.example.com", "blog-.example.com", "blog..example.com",
		"example.com.", ".example.com", "blog_site.example.com", "https://example.com", label + "a.example.com",
		strings.Join([]string{label, label, label, strings.Repeat("b", 62)}, "."), "203.0.113.7", "1.2.3",
		"example.123", "example.1-2",
	} {
		_, err := settingsOn(unreadable, envOf(nil), Request{Domain: domain})

		wantRefused(t, err, "-domain: must be "+wantedHost+`, got "`+domain+`"`)
	}
}

func TestSettingsOnRefuseAnArchiveOutsideTheInstallersFolderBeforeTheyReadServerEnv(t *testing.T) {
	t.Parallel()

	unreadable := serverWith(t, nil)
	if err := os.MkdirAll(filepath.Join(unreadable.Root, serverEnvPath), 0o755); err != nil {
		t.Fatalf("making a folder where server.env goes: %v", err)
	}
	for _, archive := range []string{
		"gophenberg_0.20.0_linux_amd64.tar.gz", "/tmp/gophenberg_0.20.0_linux_amd64.tar.gz",
		"/var/lib/gophenberg-installer", "/var/lib/gophenberg-installer/",
		"/var/lib/gophenberg-installer/../gophenberg_0.20.0_linux_amd64.tar.gz",
		"/var/lib/gophenberg-installerx/gophenberg_0.20.0_linux_amd64.tar.gz",
		"var/lib/gophenberg-installer/gophenberg_0.20.0_linux_amd64.tar.gz",
	} {
		_, err := settingsOn(unreadable, envOf(nil), Request{Domain: "example.com", Archive: archive})

		wantRefused(t, err, "-archive: must be a file inside /var/lib/gophenberg-installer, the folder install.sh "+
			`downloads it into, got "`+archive+`"`)
	}
}

func TestSettingsOnTakeAnArchiveInsideTheInstallersFolder(t *testing.T) {
	t.Parallel()

	for _, archive := range []string{
		"/var/lib/gophenberg-installer/tmp.Q2xk8/gophenberg_0.20.0_linux_amd64.tar.gz",
		"/var/lib/gophenberg-installer//gophenberg_0.20.0_linux_amd64.tar.gz",
	} {
		_, err := settingsOn(serverWith(t, nil), envOf(nil), Request{Domain: "example.com", Archive: archive})

		if err != nil {
			t.Errorf("settingsOn() of the archive %q = %v, want it taken", archive, err)
		}
	}
}

func TestSettingsOnRefuseEveryBadFlagAtOnce(t *testing.T) {
	t.Parallel()

	_, err := settingsOn(serverWith(t, nil), envOf(nil), Request{Domain: "Example.com", Archive: "/tmp/a.tar.gz"})

	wantRefused(t, err, `-domain: must be `+wantedHost+`, got "Example.com"`+"\n"+`-archive: must be a file inside `+
		`/var/lib/gophenberg-installer, the folder install.sh downloads it into, got "/tmp/a.tar.gz"`)
}

func TestSettingsOnRefuseARunWithNoDomainWhileServerEnvNamesNone(t *testing.T) {
	t.Parallel()

	machine := serverWith(t, map[string]string{serverEnvPath: "GOPHENBERG_SERVICE_RESTART_DELAY=2s\n"})

	_, err := settingsOn(machine, envOf(nil), Request{})

	wantRefused(t, err, "-domain is required while /etc/gophenberg/server.env names no GOPHENBERG_PUBLIC_URL, "+
		"such as -domain example.com")
}

func TestSettingsOnRefuseEachBadValue(t *testing.T) {
	t.Parallel()

	for name, cases := range map[string]map[string]string{
		"INSTALL_ROOT": {
			"planted/root": `must be an absolute path, got "planted/root"`,
			"/srv/my site": `must hold only letters, digits, dots, hyphens, underscores and slashes, got "/srv/my site"`,
		},
		"PUBLIC_URL": {
			"http://example.com":  `must be https:// followed by ` + wantedHost + `, got "http://example.com"`,
			"https://Example.com": `must be https:// followed by ` + wantedHost + `, got "https://Example.com"`,
		},
		"ADDR": {
			"0.0.0.0:8081":   `must be a loopback address with a port, such as 127.0.0.1:8081, got "0.0.0.0:8081"`,
			"127.0.0.1":      `must be a loopback address with a port, such as 127.0.0.1:8081, got "127.0.0.1"`,
			"localhost:8081": `must be a loopback address with a port, such as 127.0.0.1:8081, got "localhost:8081"`,
			"127.0.0.1:0":    `must be a loopback address with a port, such as 127.0.0.1:8081, got "127.0.0.1:0"`,
		},
		"INSTALL_WEB_SERVER": {"nginx": `must be caddy or none, got "nginx"`},
		"INSTALL_CERTIFICATE_EMAIL": {
			"Maria Perez <admin@example.com>": `must be one plain email address such as admin@example.com, ` +
				`got "Maria Perez <admin@example.com>"`,
			"{admin}@example.com": `must be one plain email address such as admin@example.com, ` +
				`got "{admin}@example.com"`,
		},
		"SERVICE_RESTART_DELAY":        {"2500ms": `must be a whole number of seconds, got "2500ms"`},
		"SERVICE_START_LIMIT_INTERVAL": {"0s": `must stand above zero, got "0s"`},
		"SERVICE_START_LIMIT_BURST": {
			"0": `must stand above zero, got "0"`, "4294967296": `must stand at or below 4294967295, got "4294967296"`,
		},
		"SERVICE_STOP_MARGIN":          {"soon": `must be a duration like 30s, got "soon"`},
		"MAINTENANCE_RETRY_AFTER":      {"1.5s": `must be a whole number of seconds, got "1.5s"`},
		"PROXY_RETRY_DURATION":         {"-1s": `must stand above zero, got "-1s"`},
		"INSTALL_MEMORY_FLOOR_MB":      {"0": `must stand above zero, got "0"`},
		"INSTALL_DISK_FLOOR_MB":        {"1073741825": `must stand at or below 1073741824, got "1073741825"`},
		"INSTALL_APT_LOCK_WAIT":        {"90500ms": `must be a whole number of seconds, got "90500ms"`},
		"INSTALL_DOMAIN_CHECK_TIMEOUT": {"0s": `must stand above zero, got "0s"`},
		"INSTALL_HTTPS_WAIT":           {"later": `must be a duration like 30s, got "later"`},
		"INSTALL_POLL_INTERVAL":        {"0s": `must stand above zero, got "0s"`},
		"INSTALL_PROGRESS_INTERVAL": {
			"-30s": `must not be negative, got "-30s"`, "2500ms": `must be a whole number of seconds, got "2500ms"`,
			"often": `must be a duration like 30s, got "often"`,
		},
	} {
		for value, complaint := range cases {
			env := envOf(map[string]string{"GOPHENBERG_PUBLIC_URL": "https://example.com", "GOPHENBERG_" + name: value})

			_, err := settingsOn(serverWith(t, nil), env, Request{})

			wantRefused(t, err, "GOPHENBERG_"+name+": "+complaint)
		}
	}
}

func TestSettingsOnTakeAnAptLockWaitOfZero(t *testing.T) {
	t.Parallel()

	env := envOf(map[string]string{"GOPHENBERG_INSTALL_APT_LOCK_WAIT": "0s"})

	settings, err := settingsOn(serverWith(t, nil), env, firstRun)

	if err != nil || settings.Checks.AptLockWait != 0 {
		t.Errorf("settingsOn() = %v, %v, want an apt lock wait of 0, which does not wait", settings.Checks.AptLockWait,
			err)
	}
}

func TestSettingsOnTakeAProgressIntervalOfZero(t *testing.T) {
	t.Parallel()

	env := envOf(map[string]string{"GOPHENBERG_INSTALL_PROGRESS_INTERVAL": "0"})

	settings, err := settingsOn(serverWith(t, nil), env, firstRun)

	if err != nil || settings.Checks.ProgressInterval != 0 {
		t.Errorf("settingsOn() = %v, %v, want a progress interval of 0, which prints no still-running line",
			settings.Checks.ProgressInterval, err)
	}
}

func TestSettingsOnRefuseEveryBadValueAtOnce(t *testing.T) {
	t.Parallel()

	machine := serverWith(t, map[string]string{serverEnvPath: "GOPHENBERG_ADDR=0.0.0.0:80\n"})
	env := envOf(map[string]string{"GOPHENBERG_INSTALL_POLL_INTERVAL": "often"})

	_, err := settingsOn(machine, env, firstRun)

	wantRefused(t, err, `GOPHENBERG_ADDR: must be a loopback address with a port, such as 127.0.0.1:8081, `+
		`got "0.0.0.0:80"`+"\n"+`GOPHENBERG_INSTALL_POLL_INTERVAL: must be a duration like 30s, got "often"`)
}

func TestSettingsOnRefuseAServerEnvTheyCannotRead(t *testing.T) {
	t.Parallel()

	machine := serverWith(t, nil)
	if err := os.MkdirAll(filepath.Join(machine.Root, serverEnvPath), 0o755); err != nil {
		t.Fatalf("making a folder where server.env goes: %v", err)
	}

	_, err := settingsOn(machine, envOf(nil), firstRun)

	if got := fmt.Sprint(err); !strings.HasPrefix(got, "installer: read ") || !strings.HasSuffix(got, serverEnvPath+
		": is a directory") {
		t.Errorf("settingsOn() = %v, want server.env refused as a folder", err)
	}
}

func TestSettingsOnRefuseAServerEnvTheyCannotParse(t *testing.T) {
	t.Parallel()

	machine := serverWith(t, map[string]string{serverEnvPath: "not a setting line\n"})

	_, err := settingsOn(machine, envOf(nil), firstRun)

	wantRefused(t, err, `installer: /etc/gophenberg/server.env: unexpected character "\n" in variable name near `+
		`"not a setting line\n"`)
}
