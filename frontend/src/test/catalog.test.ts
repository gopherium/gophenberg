// SPDX-License-Identifier: AGPL-3.0-or-later

import { expect, test } from 'vitest'

import { catalogAt, catalogFor, DEFAULT_LOCALE, editorCatalogFor } from '../i18n/catalog'

test('loads the catalogue a supported locale ships', async () => {
	const held = await catalogFor('es-ES')

	expect(held).toHaveProperty('')
})

test('loads nothing for the language the sources are already written in', async () => {
	expect(await catalogFor(DEFAULT_LOCALE)).toBeUndefined()
})

test('loads nothing for a locale that ships no catalogue', async () => {
	expect(await catalogFor('fr-FR')).toBeUndefined()
})

test('loads no editor catalogue for the language the editor already speaks', async () => {
	expect(await editorCatalogFor(DEFAULT_LOCALE)).toBeUndefined()
})

test('loads no editor catalogue for a locale that ships none', async () => {
	expect(await editorCatalogFor('fr-FR')).toBeUndefined()
})

test('loads a catalogue a build placed at the path', async () => {
	const held = { '': { 'plural-forms': 'nplurals=2; plural=(n != 1);' } }
	const chunks = { './es-ES.json': async () => ({ default: held }) }

	expect(await catalogAt(chunks, './es-ES.json', 'es-ES')).toBe(held)
})
