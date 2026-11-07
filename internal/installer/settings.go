// SPDX-License-Identifier: Apache-2.0

package installer

import (
	"bytes"
	"errors"
	"fmt"
	"maps"
	"math"
	"net/mail"
	"net/netip"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"time"

	"github.com/joho/godotenv"

	"github.com/gopherium/framework/gonsole"
)

// serverEnvPath is where server:install keeps its settings, from the top of the server.
const serverEnvPath = "/etc/gophenberg/server.env"

// serverEnvHeader is the first line of server.env, naming what writes it.
const serverEnvHeader = "# Written by gophenberg server:install."

// maxMegabytes is the largest floor in megabytes, a pebibyte, so that a floor counted in bytes never overflows.
const maxMegabytes = 1 << 30

// hostRule says what a domain must be.
const hostRule = "localhost or a host name such as example.com: labels of 1 to 63 lower-case letters, digits and " +
	"hyphens joined by dots, no hyphen at either end of a label, a letter in the last label, 253 characters at most"

// maxHostLength is the most characters a host name holds.
const maxHostLength = 253

// webServerCaddy names the web server server:install sets up in front of the site itself.
const webServerCaddy = "caddy"

// webServerNone names a server whose own web server, set up by the person, answers in front of the site.
const webServerNone = "none"

// webServers are the values GOPHENBERG_INSTALL_WEB_SERVER takes.
var webServers = []string{webServerCaddy, webServerNone}

// hostLabels matches two or more dot-joined labels of 1 to 63 lower-case letters, digits and inner hyphens.
var hostLabels = regexp.MustCompile(`^([a-z0-9]([a-z0-9-]{0,61}[a-z0-9])?\.)+[a-z0-9]([a-z0-9-]{0,61}[a-z0-9])?$`)

// lastLabelLetter matches a name whose last label holds a letter, as every top-level domain does.
var lastLabelLetter = regexp.MustCompile(`[a-z][a-z0-9-]*$`)

// plainPath matches a path of letters, digits, dots, hyphens, underscores and slashes alone.
var plainPath = regexp.MustCompile(`^[A-Za-z0-9._/-]+$`)

// plainValue matches a value server.env and the site's .env hold with no quotes around it.
var plainValue = regexp.MustCompile(`^[A-Za-z0-9_./:@,+=?&\[\]-]*$`)

// serverLines are the lines server.env holds, in its order, each with its default, empty where the run supplies one.
var serverLines = []struct{ name, fallback string }{
	{"INSTALL_ROOT", "/opt/gophenberg"},
	{"PUBLIC_URL", ""},
	{"ADDR", "127.0.0.1:8081"},
	{"INSTALL_WEB_SERVER", webServerCaddy},
	{"INSTALL_CERTIFICATE_EMAIL", ""},
	{"SERVICE_RESTART_DELAY", "5s"},
	{"SERVICE_START_LIMIT_INTERVAL", "300s"},
	{"SERVICE_START_LIMIT_BURST", "10"},
	{"SERVICE_STOP_MARGIN", "5s"},
	{"MAINTENANCE_RETRY_AFTER", "600s"},
	{"PROXY_RETRY_DURATION", "10s"},
}

// Settings is what one run of server:install acts on, as GOPHENBERG_ names it, with the text server.env gets.
type Settings struct {
	Root             string
	Domain           string
	Addr             netip.AddrPort
	WebServer        string
	CertificateEmail string
	Restarts         Restarts
	Checks           Checks
	serverEnv        string
}

// Restarts is how the units restart the site and how Caddy holds its visitors meanwhile.
type Restarts struct {
	Delay         time.Duration
	LimitInterval time.Duration
	LimitBurst    int
	StopMargin    time.Duration
	RetryAfter    time.Duration
	ProxyRetry    time.Duration
}

// Checks is how server:install checks the machine, how long each wait lasts and how often it prints a progress line.
type Checks struct {
	MemoryFloorMB      int
	DiskFloorMB        int
	AptLockWait        time.Duration
	DomainCheckTimeout time.Duration
	HTTPSWait          time.Duration
	PollInterval       time.Duration
	StartWait          time.Duration
	ProgressInterval   time.Duration
}

// withCaddy reports whether the run sets up Caddy in front of the site, not the person's own web server.
func (s Settings) withCaddy() bool {
	return s.WebServer == webServerCaddy
}

// settingsOn reads one run's settings from the flags, env, server.env then the defaults, refusing a bad flag first.
func settingsOn(machine Machine, env gonsole.Env, request Request) (Settings, error) {
	flagged := errors.Join(domainFlag(request.Domain), archiveFlag(request.Archive), webServerFlag(request.WebServer),
		passwordFileFlag(request.AdminPasswordFile))
	if flagged != nil {
		return Settings{}, flagged
	}
	file, err := readServerEnv(machine)
	if err != nil {
		return Settings{}, err
	}
	asked := layered(lookup(flagLines(env, request)), env.Getenv)
	if err := errors.Join(keptWebServer(env, asked, file), keptInstallRoot(env, asked, file)); err != nil {
		return Settings{}, err
	}
	given := layered(asked, lookup(file))
	read := gonsole.Env{Prefix: env.Prefix, Getenv: layered(given, lookup(defaultLines(env, request)))}
	settings, err := parsed(read)
	if err != nil {
		return Settings{}, err
	}
	settings.serverEnv = serverEnvText(read, keptLines(env, file, given))
	return settings, nil
}

// domainFlag refuses a -domain that is given and is not a host name.
func domainFlag(domain string) error {
	if domain == "" || isHostName(domain) {
		return nil
	}
	return fmt.Errorf("-domain: must be %s, got %q", hostRule, domain)
}

// archiveFlag refuses an -archive that is given and is not a file inside installerFolder.
func archiveFlag(archive string) error {
	if archive == "" || strings.HasPrefix(filepath.Clean(archive), installerFolder+"/") {
		return nil
	}
	return fmt.Errorf("-archive: must be a file inside %s, the folder install.sh downloads it into, got %q",
		installerFolder, archive)
}

// passwordFileFlag refuses an -admin-password-file that is given and is not an absolute path.
func passwordFileFlag(path string) error {
	if path == "" || filepath.IsAbs(path) {
		return nil
	}
	return fmt.Errorf("-admin-password-file: must be an absolute path, got %q", path)
}

// webServerFlag refuses a -web-server that is given and is neither caddy nor none.
func webServerFlag(webServer string) error {
	if webServer == "" {
		return nil
	}
	if _, err := webServerName(webServer); err != nil {
		return fmt.Errorf("-web-server: %w", err)
	}
	return nil
}

// webServerName reads the web server in front of the site, refusing a name that is neither caddy nor none.
func webServerName(name string) (string, error) {
	if slices.Contains(webServers, name) {
		return name, nil
	}
	return "", fmt.Errorf("must be caddy or none, got %q", name)
}

// keptWebServer refuses a later run that asks for the other web server than the one server.env file keeps.
func keptWebServer(env gonsole.Env, asked func(string) string, file map[string]string) error {
	key := env.Key("INSTALL_WEB_SERVER")
	kept, wanted := layered(lookup(file))(key), asked(key)
	if !slices.Contains(webServers, kept) || !slices.Contains(webServers, wanted) {
		return nil
	}
	if wanted == kept {
		return nil
	}
	return settingMoved(env, "INSTALL_WEB_SERVER", kept, wanted)
}

// keptInstallRoot refuses a later run that asks for another install root than the one server.env file keeps.
func keptInstallRoot(env gonsole.Env, asked func(string) string, file map[string]string) error {
	key := env.Key("INSTALL_ROOT")
	kept, keptErr := installRoot(layered(lookup(file))(key))
	wanted, wantedErr := installRoot(asked(key))
	if keptErr != nil || wantedErr != nil {
		return nil
	}
	if wanted == kept {
		return nil
	}
	return settingMoved(env, "INSTALL_ROOT", kept, wanted)
}

// settingMoved refuses a later run that asks for wanted as the setting called name on a server set up with kept.
func settingMoved(env gonsole.Env, name, kept, wanted string) error {
	return fmt.Errorf("%s: this server was set up with %s, and a later run cannot move it to %s, so run sudo "+
		"gophenberg server:uninstall first, then change the line in %s", env.Key(name), kept, wanted, serverEnvPath)
}

// readServerEnv returns the settings server.env on machine holds, none before a first run wrote it.
func readServerEnv(machine Machine) (map[string]string, error) {
	body, err := machine.read(serverEnvPath)
	if err != nil {
		return nil, err
	}
	file, err := godotenv.Parse(bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("installer: %s: %w", serverEnvPath, err)
	}
	return file, nil
}

// flagLines returns the server.env lines the flags of request give: the web server and the domain's public address.
func flagLines(env gonsole.Env, request Request) map[string]string {
	lines := map[string]string{env.Key("INSTALL_WEB_SERVER"): request.WebServer}
	if request.Domain != "" {
		lines[env.Key("PUBLIC_URL")] = "https://" + request.Domain
	}
	return lines
}

// defaultLines returns the value of each server.env line when nothing else gives one, for the request given.
func defaultLines(env gonsole.Env, request Request) map[string]string {
	defaults := make(map[string]string, len(serverLines))
	for _, line := range serverLines {
		defaults[env.Key(line.name)] = line.fallback
	}
	defaults[env.Key("INSTALL_CERTIFICATE_EMAIL")] = request.AdminEmail
	return defaults
}

// keptLines returns every setting of file that is not one of serverLines, each with the value given reads for it.
func keptLines(env gonsole.Env, file map[string]string, given func(string) string) map[string]string {
	kept := make(map[string]string, len(file))
	for key := range file {
		kept[key] = given(key)
	}
	for _, line := range serverLines {
		delete(kept, env.Key(line.name))
	}
	return kept
}

// lookup returns a reader of the settings values holds.
func lookup(values map[string]string) func(string) string {
	return func(key string) string { return values[key] }
}

// layered returns a reader that takes each setting from the first of readers that gives it a value.
func layered(readers ...func(string) string) func(string) string {
	return func(key string) string {
		for _, read := range readers {
			if value := strings.TrimSpace(read(key)); value != "" {
				return value
			}
		}
		return ""
	}
}

// parsed reads every setting from env, where each server.env line already holds a value, refusing every bad one.
func parsed(env gonsole.Env) (Settings, error) {
	root, rootErr := gonsole.Parse(env, "INSTALL_ROOT", "", installRoot)
	domain, domainErr := publicDomain(env)
	addr, addrErr := gonsole.Parse(env, "ADDR", netip.AddrPort{}, loopback)
	webServer, webServerErr := gonsole.Parse(env, "INSTALL_WEB_SERVER", "", webServerName)
	email, emailErr := gonsole.Parse(env, "INSTALL_CERTIFICATE_EMAIL", "", certificateEmail)
	restarts, restartsErr := restartsFrom(env)
	checks, checksErr := checksFrom(env)
	settings := Settings{
		Root: root, Domain: domain, Addr: addr, WebServer: webServer, CertificateEmail: email, Restarts: restarts,
		Checks: checks,
	}
	return settings, errors.Join(rootErr, domainErr, addrErr, webServerErr, emailErr, restartsErr, checksErr)
}

// restartsFrom reads how the units restart the site and how Caddy holds its visitors, each line holding a value.
func restartsFrom(env gonsole.Env) (Restarts, error) {
	delay, delayErr := seconds(env, "SERVICE_RESTART_DELAY", 0)
	interval, intervalErr := seconds(env, "SERVICE_START_LIMIT_INTERVAL", 0)
	burst, burstErr := env.Count("SERVICE_START_LIMIT_BURST", 0, gonsole.AtMost(math.MaxUint32))
	margin, marginErr := env.Duration("SERVICE_STOP_MARGIN", 0)
	retryAfter, retryAfterErr := seconds(env, "MAINTENANCE_RETRY_AFTER", 0)
	proxyRetry, proxyRetryErr := env.Duration("PROXY_RETRY_DURATION", 0)
	restarts := Restarts{
		Delay: delay, LimitInterval: interval, LimitBurst: burst, StopMargin: margin, RetryAfter: retryAfter,
		ProxyRetry: proxyRetry,
	}
	return restarts, errors.Join(delayErr, intervalErr, burstErr, marginErr, retryAfterErr, proxyRetryErr)
}

// checksFrom reads the checks, the waits and the progress interval of server:install, each with its default.
func checksFrom(env gonsole.Env) (Checks, error) {
	memory, memoryErr := env.Count("INSTALL_MEMORY_FLOOR_MB", 900, gonsole.AtMost(maxMegabytes))
	disk, diskErr := env.Count("INSTALL_DISK_FLOOR_MB", 1024, gonsole.AtMost(maxMegabytes))
	apt, aptErr := seconds(env, "INSTALL_APT_LOCK_WAIT", 5*time.Minute, gonsole.AllowZero())
	domain, domainErr := env.Duration("INSTALL_DOMAIN_CHECK_TIMEOUT", 10*time.Second)
	https, httpsErr := env.Duration("INSTALL_HTTPS_WAIT", 2*time.Minute)
	poll, pollErr := env.Duration("INSTALL_POLL_INTERVAL", time.Second)
	start, startErr := env.Duration("UPDATE_VERIFY_TIMEOUT", time.Minute)
	progress, progressErr := seconds(env, "INSTALL_PROGRESS_INTERVAL", 30*time.Second, gonsole.AllowZero())
	checks := Checks{
		MemoryFloorMB: memory, DiskFloorMB: disk, AptLockWait: apt, DomainCheckTimeout: domain, HTTPSWait: https,
		PollInterval: poll, StartWait: start, ProgressInterval: progress,
	}
	return checks, errors.Join(memoryErr, diskErr, aptErr, domainErr, httpsErr, pollErr, startErr, progressErr)
}

// seconds returns the duration setting called name, refusing one that is not a whole number of seconds.
func seconds(env gonsole.Env, name string, fallback time.Duration, bounds ...gonsole.Bound) (time.Duration, error) {
	read, err := env.Duration(name, fallback, bounds...)
	if err == nil && read%time.Second != 0 {
		return 0, fmt.Errorf("%s: must be a whole number of seconds, got %q", env.Key(name), env.Value(name))
	}
	return read, err
}

// installRoot reads the install root, refusing a relative path or a character the units would have to quote.
func installRoot(path string) (string, error) {
	if !filepath.IsAbs(path) {
		return "", fmt.Errorf("must be an absolute path, got %q", path)
	}
	if !plainPath.MatchString(path) {
		return "", fmt.Errorf("must hold only letters, digits, dots, hyphens, underscores and slashes, got %q", path)
	}
	return filepath.Clean(path), nil
}

// publicDomain returns the domain of the site's public address, refusing a run that names none.
func publicDomain(env gonsole.Env) (string, error) {
	if env.Value("PUBLIC_URL") == "" {
		return "", fmt.Errorf("-domain is required while %s names no %s, such as -domain example.com", serverEnvPath,
			env.Key("PUBLIC_URL"))
	}
	return gonsole.Parse(env, "PUBLIC_URL", "", httpsHost)
}

// httpsHost reads a public address, refusing one that is not https:// followed by a host name alone.
func httpsHost(address string) (string, error) {
	host, found := strings.CutPrefix(address, "https://")
	if found && isHostName(host) {
		return host, nil
	}
	return "", fmt.Errorf("must be https:// followed by %s, got %q", hostRule, address)
}

// isHostName reports whether host is localhost or a host name the rule allows.
func isHostName(host string) bool {
	if host == "localhost" {
		return true
	}
	if len(host) > maxHostLength {
		return false
	}
	return hostLabels.MatchString(host) && lastLabelLetter.MatchString(host)
}

// loopback reads the site's address, refusing one that is not a loopback address with a port.
func loopback(address string) (netip.AddrPort, error) {
	read, err := netip.ParseAddrPort(address)
	if err == nil && loopbackWithPort(read) {
		return read, nil
	}
	return netip.AddrPort{}, fmt.Errorf("must be a loopback address with a port, such as 127.0.0.1:8081, got %q", address)
}

// loopbackWithPort reports whether address is a loopback address with a port above zero.
func loopbackWithPort(address netip.AddrPort) bool {
	return address.Addr().IsLoopback() && address.Port() != 0
}

// certificateEmail reads the email of the certificate account, refusing all but one plain address without braces.
func certificateEmail(email string) (string, error) {
	if plainEmail(email) && !strings.ContainsAny(email, "{}") {
		return email, nil
	}
	return "", fmt.Errorf("must be one plain email address such as admin@example.com, got %q", email)
}

// plainEmail reports whether email is one address with no display name.
func plainEmail(email string) bool {
	parsed, err := mail.ParseAddress(email)
	return err == nil && parsed.Address == email
}

// serverEnvText returns the text server.env gets: its header, each of serverLines as env reads it, then kept in order.
func serverEnvText(env gonsole.Env, kept map[string]string) string {
	lines := []string{serverEnvHeader}
	for _, line := range serverLines {
		lines = append(lines, envLine(env.Key(line.name), env.Value(line.name)))
	}
	for _, key := range slices.Sorted(maps.Keys(kept)) {
		lines = append(lines, envLine(key, kept[key]))
	}
	return strings.Join(lines, "\n") + "\n"
}

// envLine returns the line that sets key to value, the value in double quotes unless it needs none.
func envLine(key, value string) string {
	if plainValue.MatchString(value) {
		return key + "=" + value
	}
	quoted, _ := godotenv.Marshal(map[string]string{key: value})
	return quoted
}
