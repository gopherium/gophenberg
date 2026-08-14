// SPDX-License-Identifier: Apache-2.0

import { http, HttpResponse, server } from '@gophenberg/frontend-sdk/testing'
import { screen, within } from '@testing-library/react'
import { beforeAll, beforeEach, expect, test } from 'vitest'

import { createAppRouter } from '../router'
import { renderAt } from './render'
import { storedPost } from './postFixture'

const FRAMED = [
	'/',
	'/content/$typeKey',
	'/content-types',
	'/media',
	'/users',
	'/users/new',
	'/themes',
]

const VISITABLE = FRAMED.map((path) => path.replace('$typeKey', 'post'))

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
		http.get('/api/media', () => HttpResponse.json({ items: [], total: 0 })),
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

test.each(VISITABLE)('gives %s exactly one first level heading', async (path) => {
	renderAt(path)

	const main = await screen.findByRole('main')

	await within(main).findByRole('heading', { level: 1 })
	expect(within(main).getAllByRole('heading', { level: 1 })).toHaveLength(1)
})

test('leaves the design canvas without a page heading', async () => {
	renderAt(EDITOR_URL)

	await screen.findByRole('textbox', { name: 'Title' })

	expect(screen.queryAllByRole('heading', { level: 1 })).toHaveLength(0)
})
