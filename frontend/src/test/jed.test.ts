// SPDX-License-Identifier: Apache-2.0

import { createI18n } from '@wordpress/i18n'
import { readFileSync, readdirSync } from 'node:fs'
import { join } from 'node:path'
import { expect, test } from 'vitest'

import { compileCatalog, serializeCatalog } from '@gopherium/gottext/build'

import { catalogTargets, repositoryRoot } from '../../scripts/config.ts'

const CATALOG = `msgid ""
msgstr ""
"Language: es_ES\\n"
"MIME-Version: 1.0\\n"
"Content-Type: text/plain; charset=UTF-8\\n"
"Plural-Forms: nplurals=2; plural=(n != 1);\\n"

msgid "Add new"
msgstr "Anadir nuevo"

msgctxt "noun"
msgid "Post"
msgstr "Entrada"

msgid "%d item"
msgid_plural "%d items"
msgstr[0] "%d elemento"
msgstr[1] "%d elementos"
`

const DOMAIN = 'gophenberg'

/**
 * Returns a runtime carrying the compiled catalogue under the domain.
 * @param source - The catalogue as PO text.
 * @returns The runtime to read translations from.
 */
function loaded(source: string) {
	const i18n = createI18n()
	i18n.setLocaleData(compileCatalog(source), DOMAIN)
	return i18n
}

test('translates a plain message', () => {
	expect(loaded(CATALOG).__('Add new', DOMAIN)).toBe('Anadir nuevo')
})

test('tells two senses of one word apart by context', () => {
	expect(loaded(CATALOG)._x('Post', 'noun', DOMAIN)).toBe('Entrada')
})

test('picks the plural form the count asks for', () => {
	const i18n = loaded(CATALOG)

	expect(i18n._n('%d item', '%d items', 1, DOMAIN)).toBe('%d elemento')
	expect(i18n._n('%d item', '%d items', 2, DOMAIN)).toBe('%d elementos')
})

test('falls back to the source string when the catalogue misses one', () => {
	expect(loaded(CATALOG).__('Never translated', DOMAIN)).toBe('Never translated')
})

test('keeps translated long-jump messages scoped to failure after database access', () => {
	const messages = [
		{
			source:
				'This update needs a backup of the database. ' +
				'The site stays down if the new version fails after it reaches the database.',
			translations: {
				'es-ES':
					'Esta actualización necesita una copia de seguridad de la base de datos. ' +
					'El sitio permanecerá inactivo si la nueva versión falla después de acceder a la base de datos.',
				'fr-FR':
					'Cette mise à jour nécessite une sauvegarde de la base de données. ' +
					'Le site restera indisponible si la nouvelle version échoue après avoir accédé à la base de données.',
			},
		},
		{
			source:
				'A backup of the database is taken first. If the new version fails after reaching the database, ' +
				'the site stays down until you restore it.',
			translations: {
				'es-ES':
					'Primero se hace una copia de seguridad de la base de datos. ' +
					'Si la nueva versión falla después de acceder a la base de datos, el sitio permanecerá ' +
					'inactivo hasta que restaures la base de datos.',
				'fr-FR':
					"Une sauvegarde de la base de données est effectuée d'abord. " +
					"Si la nouvelle version échoue après avoir accédé à la base de données, le site restera " +
					"indisponible jusqu'à ce que vous restauriez la base de données.",
			},
		},
	]

	for (const locale of ['es-ES', 'fr-FR'] as const) {
		const catalogue = readFileSync(join(repositoryRoot(), 'languages', `${locale}.po`), 'utf8')
		const i18n = loaded(catalogue)
		for (const { source, translations } of messages) {
			expect(i18n.__(source, DOMAIN)).toBe(translations[locale])
		}
	}
})

test('carries the plural rule the runtime reads', () => {
	expect(compileCatalog(CATALOG)['']).toHaveProperty('plural-forms')
})

test('serializes the metadata entry first, whatever the message identifiers are', () => {
	const numbered = CATALOG + '\nmsgid "7"\nmsgstr "siete"\n'

	const written = serializeCatalog(compileCatalog(numbered))

	expect(written.indexOf('""')).toBeLessThan(written.indexOf('"7"'))
})

test('translates a message whose identifier reads as a number', () => {
	const numbered = CATALOG + '\nmsgid "7"\nmsgstr "siete"\n'
	const i18n = createI18n()
	i18n.setLocaleData(compileCatalog(numbered), DOMAIN)

	expect(i18n.__('7', DOMAIN)).toBe('siete')
})

test('serializes the same bytes on every run', () => {
	expect(serializeCatalog(compileCatalog(CATALOG))).toBe(serializeCatalog(compileCatalog(CATALOG)))
})

test('compiles a catalogue that names no language or plural rule', () => {
	const bare = 'msgid ""\nmsgstr ""\n\nmsgid "Add new"\nmsgstr "Anadir nuevo"\n'

	expect(compileCatalog(bare)['']).toEqual({ lang: '', 'plural-forms': '' })
})

test('rebuilds every committed catalogue byte for byte, everywhere one is shipped', () => {
	const sources = join(repositoryRoot(), 'languages')
	const targets = catalogTargets(repositoryRoot())

	expect(targets.length).toBeGreaterThan(1)
	for (const file of readdirSync(sources).filter((held) => held.endsWith('.po'))) {
		const locale = file.slice(0, -'.po'.length)
		const rebuilt = serializeCatalog(compileCatalog(readFileSync(join(sources, file), 'utf8')))

		for (const target of targets) {
			expect(readFileSync(join(target, `${locale}.json`), 'utf8')).toBe(rebuilt)
		}
	}
})
