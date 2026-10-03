// SPDX-License-Identifier: Apache-2.0

import { readFileSync } from 'node:fs'
import { join } from 'node:path'
import { po } from 'gettext-parser'
import { expect, test } from 'vitest'

import { repositoryRoot } from '../../scripts/config.ts'

/** The plural rule each language the site answers in is written under. */
const RULES: Record<string, string> = {
	'es-ES': 'nplurals=3; plural=((n == 1) ? 0 : ((n != 0 && n % 1000000 == 0) ? 1 : 2));',
	'fr-FR': 'nplurals=3; plural=(n == 0 || n == 1) ? 0 : n != 0 && n % 1000000 == 0 ? 1 : 2;',
}

/** Every language the site answers in. */
const LOCALES = Object.keys(RULES)

/**
 * Returns a catalogue as the repository ships it.
 * @param locale - The language to read.
 * @returns The catalogue as PO text.
 */
function catalogOf(locale: string): string {
	return readFileSync(join(repositoryRoot(), 'languages', `${locale}.po`), 'utf8')
}

/**
 * Returns how many forms a plural rule declares.
 * @param rule - The Plural-Forms header.
 * @returns The count declared, or zero when the rule names none.
 */
function formsOf(rule: string): number {
	return Number(/nplurals\s*=\s*(\d+)/.exec(rule)?.[1] ?? 0)
}

test.each(LOCALES)('the %s catalogue is written under the plural rule its language uses', (locale) => {
	expect(po.parse(catalogOf(locale)).headers['Plural-Forms']).toBe(RULES[locale])
})

test.each(LOCALES)('the %s catalogue answers every plural message in every form its rule declares', (locale) => {
	const wanted = formsOf(RULES[locale])
	const short: string[] = []
	for (const entries of Object.values(po.parse(catalogOf(locale)).translations)) {
		for (const [msgid, entry] of Object.entries(entries)) {
			if (entry.msgid_plural === undefined) {
				continue
			}
			if (entry.msgstr.length !== wanted || entry.msgstr.some((form) => form === '')) {
				short.push(msgid)
			}
		}
	}

	expect(short).toEqual([])
})

test('reads the number of forms a rule declares', () => {
	expect(formsOf('nplurals=3; plural=(n != 1);')).toBe(3)
})

test('reads no forms from a rule that declares none', () => {
	expect(formsOf('plural=(n != 1);')).toBe(0)
})
