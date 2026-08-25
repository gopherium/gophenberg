// SPDX-License-Identifier: Apache-2.0

import { readFileSync, readdirSync } from 'node:fs'
import { join } from 'node:path'
import { expect, test } from 'vitest'

import { orphaned, unreviewed, untranslated } from '@gopherium/gottext/build'

import { repositoryRoot } from '../../scripts/config.ts'

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


test.each(catalogues())('says how much of %s still waits for review', (locale, source) => {
	const waiting = unreviewed(source)

	console.log(`${locale}: ${waiting.length} answers awaiting review`)
	expect(Array.isArray(waiting)).toBe(true)
})

test('names an answer still carrying the fuzzy flag', () => {
	const held = `msgid ""\nmsgstr ""\n"Language: es-ES\\n"\n\n#, fuzzy\nmsgid "Machine"\nmsgstr "Maquina"\n`

	expect(unreviewed(held)).toEqual(['Machine'])
})

test('names no answer waiting for review in a settled catalogue', () => {
	const held = `msgid ""\nmsgstr ""\n"Language: es-ES\\n"\n\nmsgid "Settled"\nmsgstr "Asentado"\n`

	expect(unreviewed(held)).toEqual([])
})

test.each(catalogues())('carries nothing in %s that the template does not name', (_locale, source) => {
	const template = readFileSync(join(repositoryRoot(), 'languages', 'gophenberg.pot'), 'utf8')

	expect(orphaned(source, template)).toEqual([])
})

test('names a message the template no longer holds', () => {
	const template = `msgid ""\nmsgstr ""\n\nmsgid "Kept"\nmsgstr ""\n`
	const held = `msgid ""\nmsgstr ""\n"Language: es-ES\\n"\n\nmsgid "Kept"\nmsgstr "Conservado"\n\n` +
		`msgid "Retired"\nmsgstr "Retirado"\n`

	expect(orphaned(held, template)).toEqual(['Retired'])
})

test('names an orphan under its context, apart from the same word without one', () => {
	const template = `msgid ""\nmsgstr ""\n\nmsgid "Post"\nmsgstr ""\n`
	const held = `msgid ""\nmsgstr ""\n"Language: es-ES\\n"\n\nmsgctxt "noun"\nmsgid "Post"\nmsgstr "Entrada"\n`

	expect(orphaned(held, template)).toEqual([`noun${''}Post`])
})

test('names no orphan in a catalogue the template fully covers', () => {
	const template = `msgid ""\nmsgstr ""\n\nmsgid "Kept"\nmsgstr ""\n`
	const held = `msgid ""\nmsgstr ""\n"Language: es-ES\\n"\n\nmsgid "Kept"\nmsgstr "Conservado"\n`

	expect(orphaned(held, template)).toEqual([])
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
