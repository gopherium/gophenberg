// SPDX-License-Identifier: Apache-2.0

import { http, HttpResponse, server } from '@gophenberg/frontend-sdk/testing'
import { screen, within } from '@testing-library/react'
import { beforeAll, beforeEach, expect, test } from 'vitest'

import { createAppRouter } from '../router'
import { renderAt } from './render'
import { storedPost } from './postFixture'

const FRAMED = ['/', '/posts', '/users', '/users/new']

const CHROMELESS = ['/posts/$postId/edit']

const EDITOR_URL = `/posts/${storedPost.id}/edit`

beforeAll(async () => {
	await import('../posts/editorRoute.lazy')
}, 120000)

beforeEach(() => {
	server.use(
		http.get('/api/posts', () => HttpResponse.json({ items: [], total: 0 })),
		http.get('/api/posts/counts', () => HttpResponse.json({})),
		http.get('/api/users', () => HttpResponse.json([])),
		http.get(`/api/posts/${storedPost.id}`, () => HttpResponse.json(storedPost)),
		http.get(`/api/posts/${storedPost.id}/autosave`, () =>
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

test.each(FRAMED)('gives %s exactly one first level heading', async (path) => {
	renderAt(path)

	const main = await screen.findByRole('main')

	await within(main).findByRole('heading', { level: 1 })
	expect(within(main).getAllByRole('heading', { level: 1 })).toHaveLength(1)
})

test('gives the editor exactly one first level heading', async () => {
	renderAt(EDITOR_URL)

	await screen.findByRole('textbox', { name: 'Title' })

	expect(screen.getAllByRole('heading', { level: 1 })).toHaveLength(1)
})
