// SPDX-License-Identifier: Apache-2.0

// Command gophenberg runs the Gophenberg CMS server and the commands an operator runs beside it.
package main

import (
	"os"

	"github.com/gopherium/framework/gonsole"
	"github.com/joho/godotenv"

	"github.com/gopherium/gophenberg/internal/app"
)

// main runs the gophenberg command line over the compiled plugins.
func main() {
	_ = godotenv.Load()
	os.Exit(gonsole.Main(app.Program(os.Getenv, registerPlugins)))
}
