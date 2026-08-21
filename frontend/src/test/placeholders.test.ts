// SPDX-License-Identifier: Apache-2.0

import { readFileSync, readdirSync } from 'node:fs'
import { join } from 'node:path'
import { describe, expect, test } from 'vitest'

import { mismatched } from '@gopherium/gottext/build'

import { repositoryRoot } from '../../scripts/config.ts'

test('passes a translation keeping the bare placeholder its message names', () => {
	const naming = 'msgid "Disable %s"\nmsgstr ""\n'
	const answered = 'msgid "Disable %s"\nmsgstr "Desactivar a %s"\n'

	expect(mismatched(answered, naming)).toEqual([])
})

test('names a translation dropping the bare placeholder its message names', () => {
	const naming = 'msgid "Disable %s"\nmsgstr ""\n'
	const dropped = 'msgid "Disable %s"\nmsgstr "Desactivar"\n'

	expect(mismatched(dropped, naming)).toEqual(['Disable %s'])
})

test('passes a translation reordering the positional placeholders its message names', () => {
	const naming = 'msgid "%1$s of %2$s"\nmsgstr ""\n'
	const answered = 'msgid "%1$s of %2$s"\nmsgstr "%2$s de %1$s"\n'

	expect(mismatched(answered, naming)).toEqual([])
})

test('names a translation carrying a bare placeholder its message does not name', () => {
	const naming = 'msgid "Saved"\nmsgstr ""\n'
	const bare = 'msgid "Saved"\nmsgstr "Guardado %s"\n'

	expect(mismatched(bare, naming)).toEqual(['Saved'])
})

/** The separator gettext writes between a context and the message it qualifies. */
const CONTEXT = String.fromCharCode(4)

/**
 * Returns every catalogue the repository ships, named by its locale.
 * @returns The locale of each catalogue paired with its PO source.
 */
function catalogues(): [string, string][] {
	const where = join(repositoryRoot(), 'languages')
	return readdirSync(where)
		.filter((named) => named.endsWith('.po'))
		.map((named) => [named.replace(/\.po$/, ''), readFileSync(join(where, named), 'utf8')])
}

describe('every shipped catalogue', () => {
	const template = readFileSync(join(repositoryRoot(), 'languages', 'gophenberg.pot'), 'utf8')

	test.each(catalogues())('answers %s with the placeholders its messages name', (_locale, source) => {
		expect(mismatched(source, template)).toEqual([])
	})
})

test('names a translation carrying a placeholder the message does not', () => {
	const template = `msgid ""\nmsgstr ""\n\nmsgid "Activate %(theme)s"\nmsgstr ""\n`
	const held = `msgid ""\nmsgstr ""\n"Language: es-ES\\n"\n\nmsgid "Activate %(theme)s"\nmsgstr "Activar %(name)s"\n`

	expect(mismatched(held, template)).toEqual(['Activate %(theme)s'])
})

test('names a translation that kept a bare placeholder', () => {
	const template = `msgid ""\nmsgstr ""\n\nmsgid "Activate %(theme)s"\nmsgstr ""\n`
	const held = `msgid ""\nmsgstr ""\n"Language: es-ES\\n"\n\nmsgid "Activate %(theme)s"\nmsgstr "Activar %s"\n`

	expect(mismatched(held, template)).toEqual(['Activate %(theme)s'])
})

test('names a translation that dropped a placeholder the message names', () => {
	const template = `msgid ""\nmsgstr ""\n\nmsgid "Media %(id)d"\nmsgstr ""\n`
	const held = `msgid ""\nmsgstr ""\n"Language: es-ES\\n"\n\nmsgid "Media %(id)d"\nmsgstr "Medio"\n`

	expect(mismatched(held, template)).toEqual(['Media %(id)d'])
})

test('passes a translation naming exactly the placeholders its message names', () => {
	const template = `msgid ""\nmsgstr ""\n\nmsgid "Activate %(theme)s"\nmsgstr ""\n`
	const held = `msgid ""\nmsgstr ""\n"Language: es-ES\\n"\n\nmsgid "Activate %(theme)s"\nmsgstr "Activar %(theme)s"\n`

	expect(mismatched(held, template)).toEqual([])
})

test('reads each plural form against the message it answers', () => {
	const template = `msgid ""\nmsgstr ""\n\nmsgid "%(count)d post"\n` +
		`msgid_plural "%(count)d posts"\nmsgstr[0] ""\nmsgstr[1] ""\n`
	const held = `msgid ""\nmsgstr ""\n"Language: es-ES\\n"\n\nmsgid "%(count)d post"\n` +
		`msgid_plural "%(count)d posts"\nmsgstr[0] "%(count)d entrada"\nmsgstr[1] "%d entradas"\n`

	expect(mismatched(held, template)).toEqual(['%(count)d post'])
})

test('passes over a message the catalogue has not translated yet', () => {
	const template = `msgid ""\nmsgstr ""\n\nmsgid "Activate %(theme)s"\nmsgstr ""\n`
	const held = `msgid ""\nmsgstr ""\n"Language: es-ES\\n"\n\nmsgid "Activate %(theme)s"\nmsgstr ""\n`

	expect(mismatched(held, template)).toEqual([])
})

test('passes over a message the template no longer names', () => {
	const template = `msgid ""\nmsgstr ""\n\nmsgid "Kept"\nmsgstr ""\n`
	const held = `msgid ""\nmsgstr ""\n"Language: es-ES\\n"\n\nmsgid "Retired %(type)s"\nmsgstr "Retirado %s"\n`

	expect(mismatched(held, template)).toEqual([])
})

test('names a message under its context, so one word may fail twice', () => {
	const template = `msgid ""\nmsgstr ""\n\nmsgctxt "column"\nmsgid "Name %(type)s"\nmsgstr ""\n`
	const held = `msgid ""\nmsgstr ""\n"Language: es-ES\\n"\n\nmsgctxt "column"\n` +
		`msgid "Name %(type)s"\nmsgstr "Nombre %s"\n`

	expect(mismatched(held, template)).toEqual([`column${CONTEXT}Name %(type)s`])
})
