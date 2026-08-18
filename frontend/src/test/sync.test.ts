// SPDX-License-Identifier: Apache-2.0

import { expect, test } from 'vitest'

import { syncTranslations } from '../../scripts/sync.ts'
import type { Catalogues } from '../../scripts/sync.ts'
import type { Poeditor } from '../../scripts/poeditor.ts'

const HEADER = `msgid ""
msgstr ""
"Language: es-ES\\n"
"Plural-Forms: nplurals=2; plural=(n != 1);\\n"
`

/**
 * Returns a catalogue carrying one translation.
 * @param msgstr - The translation the catalogue holds.
 * @returns The catalogue as PO text.
 */
function catalogue(msgstr: string): string {
	return `${HEADER}\nmsgid "Older posts"\nmsgstr "${msgstr}"\n`
}

/**
 * Returns a platform answering with the given languages and exports.
 * @param languages - The languages the platform lists.
 * @param exports - The catalogue each language exports, keyed by platform name.
 * @param asked - The names the platform was asked for, appended to.
 * @returns The platform.
 */
function platformOf(
	languages: string[],
	exports: Record<string, string>,
	asked: string[] = [],
): Poeditor {
	return {
		languages: async () => languages,
		exportPo: async (named: string) => {
			asked.push(named)
			return exports[named] ?? ''
		},
		uploadTerms: async () => undefined,
	}
}

/**
 * Returns a catalogue store over the given committed sources.
 * @param committed - The catalogues already committed, keyed by locale.
 * @returns The store and the writes it received.
 */
function storeOf(committed: Record<string, string> = {}): {
	held: Catalogues
	written: Record<string, string>
} {
	const written: Record<string, string> = {}
	return {
		held: {
			read: (locale: string) => committed[locale],
			write: (locale: string, source: string) => {
				written[locale] = source
			},
		},
		written,
	}
}

test('asks the platform under its own name and writes under the site locale', async () => {
	const asked: string[] = []
	const platform = platformOf(['es'], { es: catalogue('Entradas anteriores') }, asked)
	const { held, written } = storeOf()

	const done = await syncTranslations(platform, ['en-US', 'es-ES'], held)

	expect(asked).toEqual(['es'])
	expect(Object.keys(written)).toEqual(['es-ES'])
	expect(done.moved).toEqual(['es-ES'])
})

test('skips a language the site does not answer in', async () => {
	const asked: string[] = []
	const platform = platformOf(['fr'], { fr: catalogue('Articles plus anciens') }, asked)
	const { held, written } = storeOf()

	const done = await syncTranslations(platform, ['en-US', 'es-ES'], held)

	expect(asked).toEqual([])
	expect(written).toEqual({})
	expect(done.skipped[0]).toContain('fr')
})

test('skips a region the site does not answer in, rather than writing over another', async () => {
	const platform = platformOf(['es-MX'], { 'es-MX': catalogue('Entradas antiguas') })
	const { held, written } = storeOf({ 'es-ES': catalogue('Entradas anteriores') })

	await syncTranslations(platform, ['en-US', 'es-ES'], held)

	expect(written).toEqual({})
})

test('skips a language nobody has translated yet', async () => {
	const bare = `${HEADER}\nmsgid "Older posts"\nmsgstr ""\n`
	const platform = platformOf(['es'], { es: bare })
	const { held, written } = storeOf()

	const done = await syncTranslations(platform, ['en-US', 'es-ES'], held)

	expect(written).toEqual({})
	expect(done.skipped[0]).toContain('nobody has translated')
})

test('writes nothing when the platform says what the catalogue already says', async () => {
	const same = catalogue('Entradas anteriores')
	const platform = platformOf(['es'], { es: same })
	const { held, written } = storeOf({ 'es-ES': same })

	const done = await syncTranslations(platform, ['en-US', 'es-ES'], held)

	expect(written).toEqual({})
	expect(done.moved).toEqual([])
})

test('keeps the plural rule the committed catalogue declares', async () => {
	const theirs = `msgid ""
msgstr ""
"Plural-Forms: nplurals=3; plural=(n==1 ? 0 : n==2 ? 1 : 2);\\n"

msgid "%d post"
msgid_plural "%d posts"
msgstr[0] "%d entrada"
msgstr[1] "%d entradas"
msgstr[2] ""
`
	const platform = platformOf(['es'], { es: theirs })
	const { held, written } = storeOf({ 'es-ES': HEADER })

	await syncTranslations(platform, ['en-US', 'es-ES'], held)

	expect(written['es-ES']).toContain('nplurals=2')
	expect(written['es-ES']).not.toMatch(/msgstr\[2\]/)
})
