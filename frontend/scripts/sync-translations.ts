// SPDX-License-Identifier: Apache-2.0

import { existsSync, readFileSync, writeFileSync } from 'node:fs'
import { join } from 'node:path'

import { poeditorAt, supportedLocales } from './poeditor.ts'
import { DOMAIN, repositoryRoot } from './pot.ts'
import { syncTranslations } from './sync.ts'

const token = process.env.POEDITOR_API_TOKEN
const project = process.env.POEDITOR_PROJECT_ID

if (token === undefined || project === undefined) {
	console.error('set POEDITOR_API_TOKEN and POEDITOR_PROJECT_ID to sync translations')
	process.exit(1)
}

const root = repositoryRoot()
const languages = join(root, 'languages')

const done = await syncTranslations(poeditorAt(token, project), supportedLocales(root), {
	read: (locale) => {
		const target = join(languages, `${locale}.po`)
		return existsSync(target) ? readFileSync(target, 'utf8') : undefined
	},
	write: (locale, source) => writeFileSync(join(languages, `${locale}.po`), source),
}, readFileSync(join(languages, `${DOMAIN}.pot`), 'utf8'))

console.log(done.moved.length === 0 ? 'no translation moved' : `translations moved: ${done.moved.join(', ')}`)
for (const held of done.skipped) {
	console.log(`skipped ${held}`)
}
for (const held of done.kept) {
	console.log(`kept ${held}`)
}
