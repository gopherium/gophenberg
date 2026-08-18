// SPDX-License-Identifier: AGPL-3.0-or-later

import { existsSync, readFileSync, writeFileSync } from 'node:fs'
import { join } from 'node:path'

import { meaningfulChange, poeditorAt } from './poeditor.ts'
import { repositoryRoot } from './pot.ts'

const token = process.env.POEDITOR_API_TOKEN
const project = process.env.POEDITOR_PROJECT_ID

if (token === undefined || project === undefined) {
	console.error('set POEDITOR_API_TOKEN and POEDITOR_PROJECT_ID to sync translations')
	process.exit(1)
}

const platform = poeditorAt(token, project)
const languages = join(repositoryRoot(), 'languages')
const moved: string[] = []

for (const locale of await platform.languages()) {
	const target = join(languages, `${locale}.po`)
	const incoming = await platform.exportPo(locale)
	const current = existsSync(target) ? readFileSync(target, 'utf8') : undefined
	if (!meaningfulChange(current, incoming)) {
		continue
	}
	writeFileSync(target, incoming)
	moved.push(locale)
}

console.log(moved.length === 0 ? 'no translation moved' : `translations moved: ${moved.join(', ')}`)
