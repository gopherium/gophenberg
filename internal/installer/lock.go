// SPDX-License-Identifier: Apache-2.0

package installer

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"syscall"
)

// lockedFor checks root and the installer folder for a run of command, then locks the folder when the run applies.
func lockedFor(machine Machine, command string, applies bool) (func(), error) {
	if machine.Euid() != 0 {
		return nil, notRoot(command)
	}
	if err := checkInstallerFolder(machine); err != nil {
		return nil, err
	}
	if !applies {
		return func() {}, nil
	}
	return lockInstallerFolder(machine)
}

// lockInstallerFolder locks the installer folder for this run alone, none while it is missing, and returns the unlock.
func lockInstallerFolder(machine Machine) (func(), error) {
	folder, err := absentAsNil(os.OpenFile(filepath.Join(machine.Root, installerFolder), os.O_RDONLY|syscall.O_NONBLOCK,
		0))
	if err != nil {
		return nil, err
	}
	if folder == nil {
		return func() {}, nil
	}
	return heldAlone(folder, syscall.Flock)
}

// heldAlone locks the open folder unless another run holds it, and returns what lets go of the lock.
func heldAlone(folder *os.File, flock func(fd, how int) error) (func(), error) {
	err := flock(int(folder.Fd()), syscall.LOCK_EX|syscall.LOCK_NB)
	if err == nil {
		return func() {
			_ = flock(int(folder.Fd()), syscall.LOCK_UN)
			_ = folder.Close()
		}, nil
	}
	_ = folder.Close()
	if errors.Is(err, syscall.EWOULDBLOCK) {
		return nil, stop("another server:install or server:uninstall is running on this server")
	}
	return nil, fmt.Errorf("installer: lock %s: %w", installerFolder, err)
}
