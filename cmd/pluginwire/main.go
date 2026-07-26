// SPDX-License-Identifier: Apache-2.0

// Command pluginwire regenerates the plugin wiring files from the
// plugin.json manifest of every directory under plugins/.
package main

import (
	"fmt"
	"os"

	"github.com/gopherium/pluginkit/wire"
)

// config parameterizes the shared wiring generator for Gophenberg.
var config = wire.Config{
	SDKImport:    "github.com/gopherium/gophenberg/sdk",
	FrontendSDK:  "@gophenberg/frontend-sdk",
	GoWiringPath: "cmd/gophenberg/plugins_gen.go",
	TSWiringPath: "frontend/src/plugins/index.ts",
	License:      "Apache-2.0",
}

// main regenerates the plugin wiring files.
func main() {
	if err := wire.Run(".", config); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
