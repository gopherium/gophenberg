// SPDX-License-Identifier: Apache-2.0

import { readFileSync } from 'node:fs'
import { join } from 'node:path'
import { expect, test } from 'vitest'

import { messages, pot } from '@gopherium/gottext/build'

import { potConfig, repositoryRoot } from '../../scripts/config.ts'

const ROOT = repositoryRoot()

const FIXTURE = ['frontend/testdata/potfixture/*.tsx']

test('writes the same template bytes on every run', () => {
	expect(pot(potConfig()).equals(pot(potConfig()))).toBe(true)
})

test('reads every gettext call a source may make', () => {
	const found = messages(potConfig(FIXTURE)).map((held) => held.text).sort()

	expect(found).toEqual(['%d draft waits', '%d item held', 'Alpha label', 'Zulu label'])
})

test('orders entries by context then message, never by the machine locale', () => {
	const written = pot(potConfig(FIXTURE)).toString('utf8')

	expect(written.indexOf('%d item held')).toBeLessThan(written.indexOf('Zulu label'))
	expect(written.indexOf('Zulu label')).toBeLessThan(written.indexOf('dialog title'))
	expect(written.indexOf('dialog title')).toBeLessThan(written.indexOf('sidebar summary'))
})

test('carries a plural pair with both its empty forms', () => {
	expect(pot(potConfig(FIXTURE)).toString('utf8')).toContain('msgid_plural "%d items held"')
})

test('regenerates the committed template byte for byte', () => {
	const committed = readFileSync(join(ROOT, 'languages', 'gophenberg.pot'), 'utf8')

	expect(pot(potConfig()).toString('utf8')).toBe(committed)
})
