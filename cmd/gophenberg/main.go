// SPDX-License-Identifier: Apache-2.0

// Command gophenberg runs the Gophenberg CMS server and the commands an operator runs beside it.
package main

import (
	"os"

	"github.com/joho/godotenv"

	"github.com/gopherium/gophenberg/internal/app"
)

// main runs the gophenberg command line over the compiled plugins, after .env unless a server: command runs.
func main() {
	if app.ReadsDotEnv(os.Args[1:]) {
		_ = godotenv.Load()
	}
	os.Exit(app.Main(os.Getenv, pluginRegistrations))
}
