// SPDX-License-Identifier: AGPL-3.0-or-later

import { mkdirSync, readFileSync, readdirSync, writeFileSync } from 'node:fs'
import { join } from 'node:path'

import { compileCatalog, serializeCatalog } from './jed.ts'
import { repositoryRoot } from './pot.ts'

const root = repositoryRoot()
const sources = join(root, 'languages')
const built = join(root, 'frontend', 'src', 'languages')
const server = join(root, 'internal', 'i18n', 'catalogs')

mkdirSync(built, { recursive: true })
mkdirSync(server, { recursive: true })
for (const file of readdirSync(sources)) {
	if (!file.endsWith('.po')) {
		continue
	}
	const locale = file.slice(0, -'.po'.length)
	const compiled = compileCatalog(readFileSync(join(sources, file), 'utf8'))
	const text = serializeCatalog(compiled)
	writeFileSync(join(built, `${locale}.json`), text)
	writeFileSync(join(server, `${locale}.json`), text)
}
