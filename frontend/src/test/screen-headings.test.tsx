// SPDX-License-Identifier: Apache-2.0

import { http, HttpResponse, server } from '@gophenberg/frontend-sdk/testing'
import { screen, within } from '@testing-library/react'
import { resetLocaleData, setLocaleData } from '@wordpress/i18n'
import { expect, onTestFinished, test, vi } from 'vitest'

import { catalogFor } from '../i18n/catalog'
import { gate } from './gate'
import { renderAt } from './render'
import { warmPostsScreen } from './warm'

warmPostsScreen()

/**
 * Serves an empty posts listing so the list screen settles.
 */
function servePosts() {
	server.use(http.get('/api/content', () => HttpResponse.json({ items: [], total: 0 })))
}

/** The post type as the registry answers it on a new site. */
const POST_ROW = {
	key: 'post',
	singular_label: 'Post',
	plural_label: 'Posts',
	description: 'Manage the posts on this site.',
	route_word: '',
	hierarchical: false,
	revisions: true,
	revision_cap: 100,
	page_kind: 'single',
	default: true,
	active: true,
	fields: [],
}

/**
 * Serves a registry holding the one type.
 * @param row - The type as the registry answers it.
 */
function serveOnly(row: Record<string, unknown>) {
	server.use(http.get('/api/types', () => HttpResponse.json({ items: [row] })))
}

test('names the list the root opens with its only second level heading, beside the sidebar', async () => {
	servePosts()

	renderAt('/')

	const main = await screen.findByRole('main')
	await within(main).findByRole('heading', { level: 3, name: 'No items yet' })
	expect(within(main).getByRole('heading', { level: 2 })).toHaveTextContent('Posts')
	expect(within(main).queryAllByRole('heading', { level: 1 })).toHaveLength(0)
})

test('names the posts screen with its only second level heading, beside the sidebar', async () => {
	servePosts()

	renderAt('/content/post')

	const main = await screen.findByRole('main')
	await within(main).findByRole('heading', { level: 3, name: 'No items yet' })
	expect(within(main).getByRole('heading', { level: 2 })).toHaveTextContent('Posts')
	expect(within(main).queryAllByRole('heading', { level: 1 })).toHaveLength(0)
})

test('says under the posts title what the type describes itself as', async () => {
	servePosts()

	renderAt('/content/post')

	expect(await screen.findByText('Manage the posts on this site.')).toHaveClass('godmin-page__subtitle')
})

test('names the posts list in the words of WordPress es_ES and says what it is for in Spanish', async () => {
	setLocaleData(await catalogFor('es-ES'), 'gophenberg')
	onTestFinished(() => resetLocaleData({}, 'gophenberg'))
	servePosts()

	renderAt('/content/post')

	const main = await screen.findByRole('main')
	expect(await within(main).findByRole('heading', { level: 2, name: 'Entradas' })).toBeInTheDocument()
	expect(await screen.findByText('Gestiona las entradas de este sitio.')).toHaveClass('godmin-page__subtitle')
	expect(screen.getByRole('region', { name: 'Entradas' })).toBeInTheDocument()
	expect(screen.getByRole('searchbox', { name: 'Buscar entradas…' })).toBeInTheDocument()
})

test('keeps the list of a plugin type under a built-in key in the plugin words for a Spanish reader', async () => {
	setLocaleData(await catalogFor('es-ES'), 'gophenberg')
	onTestFinished(() => resetLocaleData({}, 'gophenberg'))
	servePosts()
	serveOnly({
		...POST_ROW,
		key: 'page',
		singular_label: 'Page',
		plural_label: 'Pages',
		description: 'Manage the pages on this site.',
		route_word: 'pages',
		origin: 'shop',
	})

	renderAt('/content/page')

	await screen.findByRole('heading', { level: 2, name: 'Pages' })
	expect(await screen.findByText('Manage the pages on this site.')).toHaveClass('godmin-page__subtitle')
})

test('offers a French reader to search the articles in the words of WordPress fr_FR', async () => {
	setLocaleData(await catalogFor('fr-FR'), 'gophenberg')
	onTestFinished(() => resetLocaleData({}, 'gophenberg'))
	servePosts()

	renderAt('/content/post')

	expect(await screen.findByRole('searchbox', { name: 'Rechercher des articles…' })).toBeInTheDocument()
})

test('shows a description a person wrote for posts as they typed it to a Spanish reader', async () => {
	setLocaleData(await catalogFor('es-ES'), 'gophenberg')
	onTestFinished(() => resetLocaleData({}, 'gophenberg'))
	servePosts()
	serveOnly({ ...POST_ROW, description: 'Everything the team writes.' })

	renderAt('/content/post')

	await screen.findByRole('heading', { level: 2, name: 'Entradas' })
	expect(await screen.findByText('Everything the team writes.')).toHaveClass('godmin-page__subtitle')
})

test('says what a list is for in words of its own when the type describes nothing', async () => {
	servePosts()
	serveOnly({ ...POST_ROW, description: '  ' })

	renderAt('/content/post')

	await screen.findByRole('heading', { level: 2, name: 'Posts' })
	expect(screen.getByText('Manage the items of this content type.')).toHaveClass('godmin-page__subtitle')
})

test('says nothing under the posts title until the type registry answers', async () => {
	servePosts()
	const registry = gate()
	server.use(
		http.get('/api/types', async () => {
			await registry.held
			return HttpResponse.json({ items: [POST_ROW] })
		}),
	)

	renderAt('/content/post')

	await screen.findByRole('heading', { level: 2, name: 'Content' })
	expect(screen.queryByText('Manage the items of this content type.')).not.toBeInTheDocument()
	registry.release()
	expect(await screen.findByText('Manage the posts on this site.')).toHaveClass('godmin-page__subtitle')
})

test('titles the sidebar with the site name, the only first level heading of the page', async () => {
	servePosts()

	renderAt('/')

	await screen.findByRole('heading', { level: 2, name: 'Posts' })
	const sidebar = screen.getByRole('region', { name: 'Navigation' })
	expect(within(sidebar).getAllByRole('heading')).toHaveLength(1)
	expect(within(sidebar).getByRole('heading', { level: 1 })).toHaveTextContent('Gophenberg')
	expect(screen.getAllByRole('heading', { level: 1 })).toEqual([within(sidebar).getByRole('heading')])
})

test('announces a posts listing it could not load in place of the list', async () => {
	vi.spyOn(console, 'error').mockImplementation(() => {})
	server.use(http.get('/api/content', () => HttpResponse.json({}, { status: 500 })))

	renderAt('/content/post')

	expect(await screen.findByRole('alert')).toHaveTextContent('Items could not be loaded.')
	expect(screen.queryByRole('searchbox')).not.toBeInTheDocument()
})

test('keeps the posts heading when the listing fails', async () => {
	vi.spyOn(console, 'error').mockImplementation(() => {})
	server.use(http.get('/api/content', () => HttpResponse.json({}, { status: 500 })))

	renderAt('/content/post')

	await screen.findByRole('alert')
	const main = screen.getByRole('main')
	expect(within(main).getByRole('heading', { level: 2 })).toHaveTextContent('Posts')
})
