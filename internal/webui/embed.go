// SPDX-License-Identifier: Apache-2.0

//go:build embedweb

package webui

import "embed"

// files holds the built admin, copied into dist by make build-release.
//
//go:embed all:dist
var files embed.FS

// root names the folder that holds the built admin inside files.
const root = "dist"
