// SPDX-License-Identifier: Apache-2.0

import { setLocaleData } from '@wordpress/i18n'
import { expect, test } from 'vitest'

import { MEDIA_CATEGORIES } from '../media/inserterCategories'

setLocaleData(
	{
		Images: ['Imagenes'],
		'Search images': ['Buscar imagenes'],
		Videos: ['Videos'],
		'Search videos': ['Buscar videos'],
		Audio: ['Audio'],
		'Search audio': ['Buscar audio'],
	},
	'gophenberg',
)

test('names every inserter category in the language the reader loaded', () => {
	const named = MEDIA_CATEGORIES.map((category) => [
		category.labels.name,
		category.labels.search_items,
	])

	expect(named).toEqual([
		['Imagenes', 'Buscar imagenes'],
		['Videos', 'Buscar videos'],
		['Audio', 'Buscar audio'],
	])
})
