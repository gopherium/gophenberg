// SPDX-License-Identifier: Apache-2.0

import { setLocaleData } from '@wordpress/i18n'
import { expect, test } from 'vitest'

import { typesNavItem } from '../../content/nav'
import { languageNavItem } from '../../i18n/nav'
import { mediaNavItem } from '../../media/nav'
import { themesNavItem } from '../../themes/nav'

setLocaleData(
	{
		'admin section\u0004Media': ['Medios'],
		Themes: ['Temas'],
		'Content Types': ['Tipos de contenido'],
		Language: ['Idioma'],
	},
	'gophenberg',
)

test('every core nav label follows the catalogue the reader loaded', () => {
	expect(mediaNavItem.label).toBe('Medios')
	expect(themesNavItem.label).toBe('Temas')
	expect(typesNavItem.label).toBe('Tipos de contenido')
	expect(languageNavItem.label).toBe('Idioma')
})
