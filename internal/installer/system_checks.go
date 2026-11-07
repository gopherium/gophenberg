// SPDX-License-Identifier: Apache-2.0

package installer

import (
	"cmp"
	"context"
	"fmt"
	"regexp"
	"slices"
	"strconv"
	"strings"
)

// osReleasePath is where the system of the server names itself, from the top of the server.
const osReleasePath = "/etc/os-release"

// meminfoPath is where Linux reports the memory of the server, from the top of the server.
const meminfoPath = "/proc/meminfo"

// systemdRunning is the folder that exists while systemd runs the server, from the top of the server.
const systemdRunning = "/run/systemd/system"

// minSystemd is the oldest systemd that hands the rollback unit how the site's unit stopped.
const minSystemd = 251

// systemRelease is one release of a system, by its os-release ID, the name the installer gives it and its VERSION_ID.
type systemRelease struct {
	id, name, version string
}

// oldestSystems are the oldest release of each system server:install sets up, and it sets up every newer release too.
var oldestSystems = []systemRelease{
	{id: "ubuntu", name: "Ubuntu", version: "24.04"},
	{id: "debian", name: "Debian", version: "12"},
}

// testedSystems are the releases Gophenberg's CI tests server:install on.
var testedSystems = []systemRelease{
	{id: "ubuntu", name: "Ubuntu", version: "24.04"},
	{id: "ubuntu", name: "Ubuntu", version: "26.04"},
}

// systemdVersion matches the start of what systemctl --version prints, up to the major version, such as systemd 255.
var systemdVersion = regexp.MustCompile(`\Asystemd ([0-9]+)\b`)

// memTotal matches the MemTotal line of /proc/meminfo, which counts the memory in kB.
var memTotal = regexp.MustCompile(`(?m)^MemTotal:[ \t]+([0-9]+) kB$`)

// checkSystem stops on a system server:install does not set up, and notes one Gophenberg's CI does not test yet.
func checkSystem(_ context.Context, c *machineCheck) error {
	body, err := c.machine.read(osReleasePath)
	if err != nil {
		return err
	}
	release := osRelease(body)
	name := systemName(release)
	found := systemRelease{id: release["ID"], version: release["VERSION_ID"]}
	if !slices.ContainsFunc(oldestSystems, found.atLeast) {
		return stop("this server runs %s, and server:install sets up %s only, while Docker Compose runs Gophenberg on "+
			"other systems", name, releaseNames(oldestSystems, " or newer"))
	}
	if slices.ContainsFunc(testedSystems, found.is) {
		return nil
	}
	_, err = fmt.Fprintf(c.notes, "%s is not yet tested in Gophenberg's CI, which tests %s\n", name,
		releaseNames(testedSystems, ""))
	return err
}

// atLeast reports whether r is oldest or a newer release of the same system, by the numbers of their VERSION_IDs.
func (r systemRelease) atLeast(oldest systemRelease) bool {
	return r.id == oldest.id && slices.Compare(versionNumbers(r.version), versionNumbers(oldest.version)) >= 0
}

// is reports whether r and other are the same release, by os-release ID and VERSION_ID as written.
func (r systemRelease) is(other systemRelease) bool {
	return r.id == other.id && r.version == other.version
}

// releaseNames returns the names of releases joined by and, each followed by suffix, such as Ubuntu 24.04 or newer.
func releaseNames(releases []systemRelease, suffix string) string {
	names := make([]string, 0, len(releases))
	for _, r := range releases {
		names = append(names, r.name+" "+r.version+suffix)
	}
	return strings.Join(names, " and ")
}

// versionNumbers returns the numbers of a VERSION_ID such as 24.04, none when it is not numbers joined by dots.
func versionNumbers(versionID string) []uint64 {
	var numbers []uint64
	for part := range strings.SplitSeq(versionID, ".") {
		number, err := strconv.ParseUint(part, 10, 64)
		if err != nil {
			return nil
		}
		numbers = append(numbers, number)
	}
	return numbers
}

// osRelease returns the settings an os-release file holds, by name, each value without the quotes around it.
func osRelease(body []byte) map[string]string {
	release := map[string]string{}
	for line := range strings.Lines(string(body)) {
		name, value, found := strings.Cut(strings.TrimSpace(line), "=")
		if found {
			release[name] = unquoted(value)
		}
	}
	return release
}

// unquoted returns value without one pair of double or single quotes around it.
func unquoted(value string) string {
	for _, quote := range []string{`"`, `'`} {
		inner, opened := strings.CutPrefix(value, quote)
		if inside, closed := strings.CutSuffix(inner, quote); opened && closed {
			return inside
		}
	}
	return value
}

// systemName returns what release calls the system, its PRETTY_NAME first.
func systemName(release map[string]string) string {
	return cmp.Or(release["PRETTY_NAME"], strings.TrimSpace(release["ID"]+" "+release["VERSION_ID"]),
		"a system "+osReleasePath+" does not name")
}

// checkSystemd stops unless systemd minSystemd or newer runs the server, and keeps its major version for the units.
func checkSystemd(ctx context.Context, c *machineCheck) error {
	if !c.machine.isFolder(systemdRunning) {
		return stop("systemd does not run this server, since %s is missing, and the site runs as a systemd service",
			systemdRunning)
	}
	major, err := systemdOn(ctx, c.machine)
	if err != nil {
		return stop("cannot tell which systemd this server runs: %v", err)
	}
	if major < minSystemd {
		return stop("this server runs systemd %d, and the site's units need systemd %d or newer", major, minSystemd)
	}
	c.found.systemd = major
	return nil
}

// systemdOn returns the major version of the systemd that runs machine, from systemctl --version.
func systemdOn(ctx context.Context, machine Machine) (int, error) {
	printed, err := machine.Run(ctx, Program{Name: "systemctl", Args: []string{"--version"}})
	if err != nil {
		return 0, err
	}
	return systemdMajor(printed)
}

// systemdMajor returns the major version the first line of systemctl --version names, such as 255.
func systemdMajor(printed string) (int, error) {
	match := systemdVersion.FindStringSubmatch(printed)
	if match == nil {
		first, _, _ := strings.Cut(printed, "\n")
		return 0, fmt.Errorf("systemctl --version printed %q, which names no systemd version", first)
	}
	return strconv.Atoi(match[1])
}

// checkMemory stops on a server with less memory than GOPHENBERG_INSTALL_MEMORY_FLOOR_MB.
func checkMemory(_ context.Context, c *machineCheck) error {
	body, err := c.machine.read(meminfoPath)
	if err != nil {
		return err
	}
	total, err := memTotalMB(body)
	if err != nil {
		return stop("cannot tell how much memory this server has: %v", err)
	}
	if floor := c.settings.Checks.MemoryFloorMB; total < floor {
		return stop("this server has %d MB of memory, less than the %d MB %s asks", total, floor,
			c.env.Key("INSTALL_MEMORY_FLOOR_MB"))
	}
	return nil
}

// memTotalMB returns the memory the MemTotal line of /proc/meminfo names, in megabytes of 1024 kB.
func memTotalMB(meminfo []byte) (int, error) {
	match := memTotal.FindSubmatch(meminfo)
	if match == nil {
		return 0, fmt.Errorf("%s has no MemTotal line in kB", meminfoPath)
	}
	kB, err := strconv.Atoi(string(match[1]))
	if err != nil {
		return 0, err
	}
	return kB / 1024, nil
}
