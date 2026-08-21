// SPDX-License-Identifier: Apache-2.0

import { mkdirSync, readFileSync, readdirSync, writeFileSync } from 'node:fs'
import { join } from 'node:path'

import { compileCatalog, serializeCatalog } from '@gopherium/gottext/build'

import { catalogTargets, repositoryRoot } from './config.ts'

const root = repositoryRoot()
const sources = join(root, 'languages')
const targets = catalogTargets(root)

for (const target of targets) {
	mkdirSync(target, { recursive: true })
}
for (const file of readdirSync(sources)) {
	if (!file.endsWith('.po')) {
		continue
	}
	const locale = file.slice(0, -'.po'.length)
	const compiled = compileCatalog(readFileSync(join(sources, file), 'utf8'))
	const text = serializeCatalog(compiled)
	for (const target of targets) {
		writeFileSync(join(target, `${locale}.json`), text)
	}
}
