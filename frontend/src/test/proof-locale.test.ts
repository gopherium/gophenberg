// SPDX-License-Identifier: Apache-2.0

import { readFileSync } from 'node:fs'
import { join } from 'node:path'
import { expect, test } from 'vitest'

import { repositoryRoot } from '../../scripts/pot.ts'
import { untranslated } from '../../scripts/completeness.ts'

/** The language one hundred percent of the catalogue is proven against. */
const PROOF_LOCALE = 'es-ES'

/**
 * Returns the catalogue source of a language.
 * @param locale - The language to read.
 * @returns The catalogue as PO text.
 */
function catalogOf(locale: string): string {
	return readFileSync(join(repositoryRoot(), 'languages', `${locale}.po`), 'utf8')
}

/** How many messages of the template the proof locale has yet to answer. */
const PENDING = 0

test('leaves no more of the template unanswered than the proof locale admits', () => {
	const template = readFileSync(join(repositoryRoot(), 'languages', 'gophenberg.pot'), 'utf8')

	expect(untranslated(catalogOf(PROOF_LOCALE), template).length).toBeLessThanOrEqual(PENDING)
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
