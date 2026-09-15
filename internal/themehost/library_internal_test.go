// SPDX-License-Identifier: Apache-2.0

package themehost

import (
	"errors"
	"fmt"
	"io/fs"
	"syscall"
	"testing"
)

func TestUnwritableRefusesAThemesDirectoryOnAReadOnlyFilesystem(t *testing.T) {
	t.Parallel()

	readOnly := &fs.PathError{Op: "mkdir", Path: "/themes/.aurora-staging-1", Err: syscall.EROFS}

	err := unwritable("aurora", fmt.Errorf("themehost: staging aurora: %w", readOnly))

	var refused *Error
	if !errors.As(err, &refused) {
		t.Fatalf("unwritable() = %v, want a read-only filesystem refused in words the admin can read", err)
	}
	if refused.Code != "themes_directory_readonly" {
		t.Errorf("Code = %q, want themes_directory_readonly", refused.Code)
	}
	if !errors.Is(err, syscall.EROFS) {
		t.Errorf("unwritable() = %v, want the read-only filesystem error kept underneath", err)
	}
}
