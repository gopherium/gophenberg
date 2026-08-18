// SPDX-License-Identifier: Apache-2.0

import { existsSync, readFileSync, writeFileSync } from 'node:fs'
import { join } from 'node:path'

import {
	localeFor,
	meaningfulChange,
	poeditorAt,
	supportedLocales,
	translated,
	withPluralRuleOf,
} from './poeditor.ts'
import { repositoryRoot } from './pot.ts'

const token = process.env.POEDITOR_API_TOKEN
const project = process.env.POEDITOR_PROJECT_ID

if (token === undefined || project === undefined) {
	console.error('set POEDITOR_API_TOKEN and POEDITOR_PROJECT_ID to sync translations')
	process.exit(1)
}

const platform = poeditorAt(token, project)
const root = repositoryRoot()
const languages = join(root, 'languages')
const supported = supportedLocales(root)
const moved: string[] = []
const skipped: string[] = []

for (const named of await platform.languages()) {
	const locale = localeFor(named, supported)
	if (locale === undefined) {
		skipped.push(`${named}, which the site does not answer in`)
		continue
	}
	const target = join(languages, `${locale}.po`)
	const current = existsSync(target) ? readFileSync(target, 'utf8') : undefined
	const exported = await platform.exportPo(locale)
	if (translated(exported) === 0) {
		skipped.push(`${named}, which nobody has translated yet`)
		continue
	}
	const incoming = current === undefined ? exported : withPluralRuleOf(current, exported)
	if (!meaningfulChange(current, incoming)) {
		continue
	}
	writeFileSync(target, incoming)
	moved.push(locale)
}

console.log(moved.length === 0 ? 'no translation moved' : `translations moved: ${moved.join(', ')}`)
for (const held of skipped) {
	console.log(`skipped ${held}`)
}
