// SPDX-License-Identifier: Apache-2.0

import { setLocaleData } from '@wordpress/i18n'
import { expect, test } from 'vitest'

import { groupsMenuItem, typesMenuItem } from '../../content/nav'
import { languageMenuItem } from '../../i18n/nav'
import { mediaMenuItem } from '../../media/nav'
import { pluginsMenuItem } from '../../pluginsscreen/nav'
import { settingsMenuItem } from '../../settings/nav'
import { themesMenuItem } from '../../themes/nav'

setLocaleData(
	{
		'admin section\u0004Media': ['Medios'],
		'admin section\u0004Themes': ['Temas'],
		'admin section\u0004Plugins': ['Plugins instalados'],
		'admin section\u0004Content Types': ['Tipos de contenido'],
		'admin section\u0004Field Groups': ['Grupos de campos'],
		'admin section\u0004Language': ['Idioma'],
		'admin section\u0004Settings': ['Ajustes'],
	},
	'gophenberg',
)

test('every core menu label follows the catalogue the reader loaded, under the admin section context', () => {
	expect(mediaMenuItem.label).toBe('Medios')
	expect(themesMenuItem.label).toBe('Temas')
	expect(pluginsMenuItem.label).toBe('Plugins instalados')
	expect(typesMenuItem.label).toBe('Tipos de contenido')
	expect(groupsMenuItem.label).toBe('Grupos de campos')
	expect(languageMenuItem.label).toBe('Idioma')
	expect(settingsMenuItem.label).toBe('Ajustes')
})
