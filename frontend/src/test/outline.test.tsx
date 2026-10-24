// SPDX-License-Identifier: Apache-2.0

import { http, HttpResponse, server } from '@gophenberg/frontend-sdk/testing'
import { screen, within } from '@testing-library/react'
import { beforeAll, beforeEach, expect, test } from 'vitest'

import { createAppRouter } from '../router'
import { renderAt } from './render'
import { storedPost } from './postFixture'
import { siteSettings } from './siteSettings'

const TITLES: Record<string, string> = {
	'/': 'Posts',
	'/content/$typeKey': 'Posts',
	'/content-types': 'Content Types',
	'/field-groups': 'Field Groups',
	'/media': 'Media',
	'/users': 'Users',
	'/users/new': 'Add User',
	'/themes': 'Themes',
	'/language': 'Language',
	'/settings': 'Settings',
	'/update-core': 'Gophenberg Updates',
}

const FRAMED = Object.keys(TITLES)

const VISITABLE = Object.entries(TITLES).map(([path, title]) => [path.replace('$typeKey', 'post'), title])

const EMPTY = [
	['/content/post', 'No items yet.'],
	['/media', 'No media items yet.'],
]

const CHROMELESS = ['/content/$typeKey/$postId/edit']

const EDITOR_URL = `/content/post/${storedPost.id}/edit`

beforeAll(async () => {
	await import('../content/EditorScreen')
}, 120000)

beforeEach(() => {
	server.use(
		http.get('/api/content', () => HttpResponse.json({ items: [], total: 0 })),
		http.get('/api/content/counts', () => HttpResponse.json({})),
		http.get('/api/users', () => HttpResponse.json([])),
		http.get('/api/themes', () => HttpResponse.json({ themes: [] })),
		http.get('/api/media', () => HttpResponse.json({ items: [], total: 0, per_page: siteSettings.list_page_size })),
		http.get('/api/locale', () => HttpResponse.json({ locale: 'en-US', supported: ['en-US'] })),
		http.get('/api/definitions/drift', () => HttpResponse.json({ orphans: [], collisions: [], nesting_kept: [] })),
		http.get(`/api/content/${storedPost.id}`, () => HttpResponse.json(storedPost)),
		http.get(`/api/content/${storedPost.id}/autosave`, () =>
			HttpResponse.json({}, { status: 404 }),
		),
	)
})

test('covers every route the application serves', () => {
	const served = new Set(
		Object.values(createAppRouter().routesById)
			.map((route) => route.fullPath)
			.filter((path) => path !== ''),
	)

	expect([...served].sort()).toEqual([...FRAMED, ...CHROMELESS].sort())
})

test.each(VISITABLE)('titles %s as %s at the second level, under the sidebar title', async (path, title) => {
	renderAt(path)

	const main = await screen.findByRole('main')

	await within(main).findByRole('heading', { level: 2, name: title })
	expect(within(main).getAllByRole('heading', { level: 2 })).toHaveLength(1)
	expect(within(main).queryAllByRole('heading', { level: 1 })).toHaveLength(0)
})

test.each(EMPTY)('titles the empty list on %s as %s one level under the page title', async (path, empty) => {
	renderAt(path)

	const main = await screen.findByRole('main')

	await within(main).findByRole('heading', { level: 3, name: empty })
	expect(within(main).getAllByRole('heading', { level: 2 })).toHaveLength(1)
})

test.each(VISITABLE)('leaves %s the sidebar title as its only first level heading', async (path, title) => {
	renderAt(path)

	await within(await screen.findByRole('main')).findByRole('heading', { level: 2, name: title })

	const sidebar = screen.getByRole('region', { name: 'Navigation' })
	expect(screen.getAllByRole('heading', { level: 1 })).toEqual([
		within(sidebar).getByRole('heading', { level: 1, name: 'Gophenberg' }),
	])
})

test('leaves the design canvas without a page heading', async () => {
	renderAt(EDITOR_URL)

	await screen.findByRole('textbox', { name: 'Title' })

	expect(screen.queryAllByRole('heading', { level: 1 })).toHaveLength(0)
})

test('keeps the title of a trashed post at the first level in the editor, with no sidebar beside it', async () => {
	server.use(
		http.get(`/api/content/${storedPost.id}`, () => HttpResponse.json({ ...storedPost, status: 'trash' })),
	)
	renderAt(EDITOR_URL)

	expect(await screen.findByRole('heading', { level: 1, name: storedPost.title })).toBeInTheDocument()
	expect(screen.getAllByRole('heading', { level: 1 })).toHaveLength(1)
	expect(screen.queryByRole('region', { name: 'Navigation' })).toBeNull()
})
