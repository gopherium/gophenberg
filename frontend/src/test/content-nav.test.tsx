// SPDX-License-Identifier: Apache-2.0

import { http, HttpResponse, server } from '@gophenberg/frontend-sdk/testing'
import { LIST_CHROME_DOMAIN, listChromeCatalogFor } from '@gopherium/godmin'
import { screen, within } from '@testing-library/react'
import { resetLocaleData, setLocaleData } from '@wordpress/i18n'
import { beforeEach, expect, onTestFinished, test } from 'vitest'

import { placeholderType } from '../content/useContentType'
import { catalogFor } from '../i18n/catalog'
import { renderAt } from './render'

const POST_TYPE = {
	key: 'post',
	singular_label: 'Post',
	plural_label: 'Posts',
	description: '',
	route_word: '',
	hierarchical: false,
	revisions: true,
	revision_cap: 100,
	page_kind: 'single',
	default: true,
	active: true,
	fields: [],
}

const PAGE_TYPE = {
	...POST_TYPE,
	key: 'page',
	singular_label: 'Page',
	plural_label: 'Pages',
	route_word: 'pages',
	hierarchical: true,
	default: false,
}

const CLOSED_TYPE = {
	...POST_TYPE,
	key: 'guide',
	singular_label: 'Guide',
	plural_label: 'Guides',
	route_word: 'guides',
	default: false,
	active: false,
}

beforeEach(() => {
	server.use(
		http.get('/api/types', () =>
			HttpResponse.json({ items: [POST_TYPE, PAGE_TYPE, CLOSED_TYPE] }),
		),
		http.get('/api/content', () => HttpResponse.json({ items: [], total: 0 })),
	)
})

test('lists a navigation row for every active type', async () => {
	renderAt('/')

	const menu = await screen.findByRole('region', { name: 'Navigation' })

	expect(await within(menu).findByRole('link', { name: 'Posts' })).toBeInTheDocument()
	expect(await within(menu).findByRole('link', { name: 'Pages' })).toBeInTheDocument()
})

/**
 * Loads the Spanish words of the admin and of godmin for one test, as the admin loads them at boot.
 */
async function readSpanish() {
	setLocaleData(await catalogFor('es-ES'), 'gophenberg')
	setLocaleData(await listChromeCatalogFor('es-ES'), LIST_CHROME_DOMAIN)
	onTestFinished(() => {
		resetLocaleData({}, 'gophenberg')
		resetLocaleData({}, LIST_CHROME_DOMAIN)
	})
}

test('names the built-in types in the words of WordPress es_ES for a Spanish reader', async () => {
	await readSpanish()
	renderAt('/')

	const menu = await screen.findByRole('region', { name: 'Navegación' })

	expect(await within(menu).findByRole('link', { name: 'Entradas' })).toBeInTheDocument()
	expect(within(menu).getByRole('link', { name: 'Páginas' })).toBeInTheDocument()
})

test('names a built-in type a person renamed as they typed it for a Spanish reader', async () => {
	await readSpanish()
	server.use(
		http.get('/api/types', () =>
			HttpResponse.json({ items: [{ ...POST_TYPE, plural_label: 'Articles' }, PAGE_TYPE] }),
		),
	)
	renderAt('/')

	const menu = await screen.findByRole('region', { name: 'Navegación' })

	expect(await within(menu).findByRole('link', { name: 'Articles' })).toBeInTheDocument()
	expect(within(menu).queryByRole('link', { name: 'Entradas' })).not.toBeInTheDocument()
})

test('puts the type that answers at the root first', async () => {
	server.use(
		http.get('/api/types', () => HttpResponse.json({ items: [PAGE_TYPE, POST_TYPE] })),
	)
	renderAt('/')

	const menu = await screen.findByRole('region', { name: 'Navigation' })
	await within(menu).findByRole('link', { name: 'Pages' })

	const rows = within(menu)
		.getAllByRole('link')
		.map((link) => link.textContent)
	expect(rows.indexOf('Posts')).toBeLessThan(rows.indexOf('Pages'))
})

test('leaves an inactive type out of the navigation', async () => {
	renderAt('/')

	const menu = await screen.findByRole('region', { name: 'Navigation' })
	await within(menu).findByRole('link', { name: 'Pages' })

	expect(within(menu).queryByRole('link', { name: 'Guides' })).not.toBeInTheDocument()
})

test('keeps the sections that do not come from the registry', async () => {
	renderAt('/')

	const menu = await screen.findByRole('region', { name: 'Navigation' })
	await within(menu).findByRole('link', { name: 'Pages' })

	expect(within(menu).getByRole('link', { name: 'Media' })).toBeInTheDocument()
	expect(within(menu).getByRole('link', { name: 'Themes' })).toBeInTheDocument()
	expect(within(menu).getByRole('link', { name: 'Content Types' })).toBeInTheDocument()
})

test('stands in with the key the route carries until the registry answers', () => {
	expect(placeholderType('guide').key).toBe('guide')
	expect(placeholderType('guide').pluralLabel).toBe('Content')
	expect(placeholderType(undefined).key).toBe('')
})
