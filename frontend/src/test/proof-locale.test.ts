// SPDX-License-Identifier: Apache-2.0

import { readFileSync, readdirSync } from 'node:fs'
import { join } from 'node:path'
import { expect, test } from 'vitest'

import { repositoryRoot } from '../../scripts/pot.ts'
import { untranslated } from '../../scripts/completeness.ts'

/**
 * Returns every language the repository ships a catalogue for.
 * @returns The locale of each catalogue paired with its PO source.
 */
function catalogues(): [string, string][] {
	const where = join(repositoryRoot(), 'languages')
	return readdirSync(where)
		.filter((named) => named.endsWith('.po'))
		.map((named) => [named.replace(/\.po$/, ''), readFileSync(join(where, named), 'utf8')])
}

/** How many messages of the template a catalogue may leave unanswered. */
const PENDING = 0

test.each(catalogues())('leaves no more of the template unanswered in %s than is admitted', (_locale, source) => {
	const template = readFileSync(join(repositoryRoot(), 'languages', 'gophenberg.pot'), 'utf8')

	expect(untranslated(source, template).length).toBeLessThanOrEqual(PENDING)
})


test('counts a message the catalogue omits entirely as waiting', () => {
	const template = `msgid ""
msgstr ""

msgid "Absent"
msgstr ""
`
	const held = `msgid ""
msgstr ""
"Language: es-ES\\n"
`

	expect(untranslated(held, template)).toEqual(['Absent'])
})

test('reports the messages a catalogue has not translated', () => {
	const held = `msgid ""
msgstr ""
"Language: es-ES\\n"

msgid "Translated"
msgstr "Traducido"

msgid "Waiting"
msgstr ""
`

	expect(untranslated(held)).toEqual(['Waiting'])
})

test('reports a plural entry whose forms are not all filled', () => {
	const held = `msgid ""
msgstr ""
"Language: es-ES\\n"

msgid "%d item"
msgid_plural "%d items"
msgstr[0] "%d elemento"
msgstr[1] ""
`

	expect(untranslated(held)).toEqual(['%d item'])
})

test('counts a message under its context, so one word may wait twice', () => {
	const held = `msgid ""
msgstr ""
"Language: es-ES\\n"

msgctxt "column"
msgid "Name"
msgstr ""
`

	expect(untranslated(held)).toEqual(['columnName'])
})
