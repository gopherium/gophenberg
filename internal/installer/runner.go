// SPDX-License-Identifier: Apache-2.0

package installer

import (
	"cmp"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
)

// searchPath is the PATH every program the runner starts gets, and the only folders it finds a program in.
const searchPath = "/usr/sbin:/usr/bin:/sbin:/bin"

// searchFolders are the folders of searchPath in the order the runner looks in them.
var searchFolders = filepath.SplitList(searchPath)

// run starts program in a session of its own with an environment built from nothing and returns what it printed.
func run(ctx context.Context, program Program) (string, error) {
	args, err := argsOf(program, searchFolders)
	if err != nil {
		return "", err
	}
	cmd := exec.CommandContext(ctx, args[0], args[1:]...)
	cmd.Env = append([]string{"PATH=" + searchPath, "LC_ALL=C"}, program.Env...)
	cmd.Dir = cmp.Or(program.Dir, "/")
	cmd.Stdin = strings.NewReader(program.Stdin)
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
	var printed strings.Builder
	cmd.Stdout = io.MultiWriter(&printed, cmp.Or(program.Output, io.Discard))
	cmd.Stderr = cmd.Stdout
	if err := cmd.Run(); err != nil {
		return printed.String(), fmt.Errorf("installer: run %s: %w", program.Name, err)
	}
	return printed.String(), nil
}

// argsOf returns the command line that starts program from folders, with runuser in front for another OS user.
func argsOf(program Program, folders []string) ([]string, error) {
	path, err := find(program.Name, folders)
	if err != nil {
		return nil, err
	}
	args := append([]string{path}, program.Args...)
	if program.User == "" {
		return args, nil
	}
	runuser, err := find("runuser", folders)
	if err != nil {
		return nil, err
	}
	return append([]string{runuser, "-u", program.User, "--"}, args...), nil
}

// find returns the program called name, as it is for an absolute path, else from the first of folders that holds it.
func find(name string, folders []string) (string, error) {
	if filepath.IsAbs(name) {
		return name, nil
	}
	for _, folder := range folders {
		path := filepath.Join(folder, name)
		if executable(path) {
			return path, nil
		}
	}
	return "", fmt.Errorf("installer: no program %s in %s", name, strings.Join(folders, string(filepath.ListSeparator)))
}

// executable reports whether path is a regular file somebody may run.
func executable(path string) bool {
	info, err := os.Stat(path)
	if err != nil {
		return false
	}
	return info.Mode().IsRegular() && info.Mode().Perm()&0o111 != 0
}
