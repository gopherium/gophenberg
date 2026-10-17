// SPDX-License-Identifier: Apache-2.0

//go:build !embedweb

package webui

import "embed"

// files holds the page a build without the admin app serves in its place.
//
//go:embed all:placeholder
var files embed.FS

// root names the folder that holds the placeholder page inside files.
const root = "placeholder"
