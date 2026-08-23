// SPDX-License-Identifier: Apache-2.0

package features_test

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/gopherium/gophenberg/internal/themehost"
)

// part is one file an archive carries.
type part struct {
	path    string
	body    string
	symlink bool
}

// manifestOf returns the manifest a theme declares itself with.
func manifestOf(name, version string) part {
	return part{
		path: "theme.json",
		body: fmt.Sprintf(`{"name":%q,"version":%q,"kit":%q}`, name, version, themehost.NewestKit()),
	}
}

// manifestNaming returns the manifest a theme declares itself with, naming the kit value given.
func manifestNaming(name, kit string) part {
	return part{
		path: "theme.json",
		body: fmt.Sprintf(`{"name":%q,"version":"1.0.0","kit":%q}`, name, kit),
	}
}

// serverEntry is a module that parses as a theme server but never serves.
var serverEntry = part{path: "server/entry.mjs", body: "export default {}\n"}

// servedBy is what a running stub theme answers the public site with.
func servedBy(name string) string { return "served by " + name }

// runningEntry returns a theme server that waits for its gate, then names itself in every answer.
func runningEntry(name, gate string) part {
	return part{path: "server/entry.mjs", body: `import { createServer } from 'node:http'
import { existsSync } from 'node:fs'

const { HOST, PORT } = process.env

const serve = () =>
	createServer((request, response) => {
		if (request.url === '/_gophenberg/health') {
			response.setHeader('content-type', 'application/json')
			response.end(JSON.stringify({ gophenberg: '0.0', ready: true }))
			return
		}
		response.end(` + quoteJS(servedBy(name)) + `)
	}).listen(Number(PORT), HOST)

const gate = ` + quoteJS(gate) + `
const opened = setInterval(() => {
	if (!existsSync(gate)) return
	clearInterval(opened)
	serve()
}, 10)
`}
}

// unstartableEntry is a theme server that parses, starts, and dies without ever serving.
var unstartableEntry = part{path: "server/entry.mjs", body: "process.exit(1)\n"}

// quoteJS returns a value as a JavaScript string literal.
func quoteJS(value string) string {
	quoted, err := json.Marshal(value)
	if err != nil {
		return `""`
	}
	return string(quoted)
}

// runnableArchive returns an archive holding a theme that serves once its gate opens.
func runnableArchive(name, version, gate string) ([]byte, error) {
	return archiveOf(manifestOf(name, version), runningEntry(name, gate), clientAsset)
}

// unstartableArchive returns an archive holding a theme that validates but never serves.
func unstartableArchive(name string) ([]byte, error) {
	return archiveOf(manifestOf(name, "1.0.0"), unstartableEntry, clientAsset)
}

// clientAsset is a file a theme serves to the browser.
var clientAsset = part{path: "client/app.css", body: "body{}\n"}

// archiveOf returns a zip carrying the given parts.
func archiveOf(parts ...part) ([]byte, error) {
	var buffer bytes.Buffer
	writer := zip.NewWriter(&buffer)
	for _, p := range parts {
		header := &zip.FileHeader{Name: p.path, Method: zip.Deflate}
		if p.symlink {
			header.SetMode(os.ModeSymlink)
		}
		file, err := writer.CreateHeader(header)
		if err != nil {
			return nil, fmt.Errorf("writing %s: %w", p.path, err)
		}
		if _, err := file.Write([]byte(p.body)); err != nil {
			return nil, fmt.Errorf("writing the body of %s: %w", p.path, err)
		}
	}
	if err := writer.Close(); err != nil {
		return nil, fmt.Errorf("closing the archive: %w", err)
	}
	return buffer.Bytes(), nil
}

// validArchive returns an archive holding a theme that meets every rule.
func validArchive(name, version string) ([]byte, error) {
	return archiveOf(manifestOf(name, version), serverEntry, clientAsset)
}

// flawedArchive returns an archive carrying the flaw the scenario names.
func flawedArchive(name, flaw string) ([]byte, error) {
	manifest := manifestOf(name, "1.0.0")
	switch flaw {
	case "carries no theme.json":
		return archiveOf(serverEntry, clientAsset)
	case `declares the name "other" in theme.json`:
		return archiveOf(manifestOf("other", "1.0.0"), serverEntry, clientAsset)
	case "is built on a kit this release does not serve":
		return archiveOf(manifestNaming(name, "0.1.0"), serverEntry, clientAsset)
	case "names a kit range rather than a version":
		return archiveOf(manifestNaming(name, "^0.9.0"), serverEntry, clientAsset)
	case "holds no server entry":
		return archiveOf(manifest, clientAsset)
	case "holds no client directory":
		return archiveOf(manifest, serverEntry)
	case "contains a symbolic link":
		return archiveOf(manifest, serverEntry, clientAsset,
			part{path: "client/escape", body: "/etc/passwd", symlink: true})
	case "unpacks to more than the size cap":
		return archiveOf(manifest, serverEntry,
			part{path: "client/app.css", body: strings.Repeat("a", int(themehost.MaxSize)+1)})
	case "contains an entry escaping its directory":
		return archiveOf(manifest, serverEntry, clientAsset, part{path: "../escaped.txt", body: "owned"})
	case "nests directories past the size cap":
		deep := strings.Repeat("d/", 400)
		parts := []part{manifest, serverEntry, clientAsset}
		for i := range 50 {
			parts = append(parts, part{path: fmt.Sprintf("client/x%d/%s", i, deep)})
		}
		return archiveOf(parts...)
	case "carries more files than the cap":
		parts := []part{manifest, serverEntry, clientAsset}
		for i := range themehost.MaxEntries {
			parts = append(parts, part{path: fmt.Sprintf("client/%d", i)})
		}
		return archiveOf(parts...)
	}
	return nil, fmt.Errorf("no archive is built for the flaw %q", flaw)
}
