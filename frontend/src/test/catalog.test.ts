// SPDX-License-Identifier: AGPL-3.0-or-later

import { expect, test } from 'vitest'

import { catalogFor, DEFAULT_LOCALE } from '../i18n/catalog'

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
