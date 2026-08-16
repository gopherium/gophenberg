// SPDX-License-Identifier: AGPL-3.0-or-later

import { readFileSync } from 'node:fs'
import { join } from 'node:path'
import { expect, test } from 'vitest'

import { messages, pot, repositoryRoot } from '../../scripts/pot.ts'

const ROOT = repositoryRoot()

const FIXTURE = ['frontend/testdata/potfixture/*.tsx']

test('writes the same template bytes on every run', () => {
	expect(pot(ROOT).equals(pot(ROOT))).toBe(true)
})

test('reads every gettext call a source may make', () => {
	const found = messages(ROOT, FIXTURE).map((held) => held.text).sort()

	expect(found).toEqual(['%d draft waits', '%d item held', 'Alpha label', 'Zulu label'])
})

test('orders entries by context then message, never by the machine locale', () => {
	const written = pot(ROOT, FIXTURE).toString('utf8')

	expect(written.indexOf('%d item held')).toBeLessThan(written.indexOf('Zulu label'))
	expect(written.indexOf('Zulu label')).toBeLessThan(written.indexOf('dialog title'))
	expect(written.indexOf('dialog title')).toBeLessThan(written.indexOf('sidebar summary'))
})

test('carries a plural pair with both its empty forms', () => {
	expect(pot(ROOT, FIXTURE).toString('utf8')).toContain('msgid_plural "%d items held"')
})

test('regenerates the committed template byte for byte', () => {
	const committed = readFileSync(join(ROOT, 'languages', 'gophenberg.pot'), 'utf8')

	expect(pot(ROOT).toString('utf8')).toBe(committed)
})
