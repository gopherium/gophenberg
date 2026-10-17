// SPDX-License-Identifier: Apache-2.0

// Package webui holds the admin app the server serves, built in by release builds.
package webui

import "io/fs"

// FS returns the admin app's files, rooted where index.html sits.
func FS() fs.FS {
	sub, _ := fs.Sub(files, root)
	return sub
}
