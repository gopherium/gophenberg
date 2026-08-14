// SPDX-License-Identifier: Apache-2.0

import { http, HttpResponse, server } from '@gophenberg/frontend-sdk/testing'
import { expect, test } from 'vitest'

import { createType, deleteType, listTypes, updateType } from '../content/types'

const POST_ROW = {
	key: 'post',
	singular_label: 'Post',
	plural_label: 'Posts',
	route_word: '',
	hierarchical: false,
	revisions: true,
	revision_cap: 100,
	page_kind: 'single',
	default: true,
	active: true,
	created_at: '2026-08-01T10:00:00Z',
	updated_at: '2026-08-01T10:00:00Z',
}

const PAGE_ROW = { ...POST_ROW, key: 'page', singular_label: 'Page', plural_label: 'Pages' }

test('reads every registered type', async () => {
	server.use(http.get('/api/types', () => HttpResponse.json({ items: [POST_ROW, PAGE_ROW] })))

	const types = await listTypes()

	expect(types.map((registered) => registered.key)).toEqual(['post', 'page'])
	expect(types[0].pluralLabel).toBe('Posts')
	expect(types[0].isDefault).toBe(true)
	expect(types[1].routeWord).toBe('')
})

test('reports a registry it could not read', async () => {
	server.use(http.get('/api/types', () => HttpResponse.json({ error: 'nope' }, { status: 500 })))

	await expect(listTypes()).rejects.toThrow(/500/)
})

test('registers a type from its labels and route word', async () => {
	const sent: unknown[] = []
	server.use(
		http.post('/api/types', async ({ request }) => {
			sent.push(await request.json())
			return HttpResponse.json(PAGE_ROW, { status: 201 })
		}),
	)

	const created = await createType({
		key: 'page',
		singularLabel: 'Page',
		pluralLabel: 'Pages',
		routeWord: 'pages',
	})

	expect(sent[0]).toEqual({
		key: 'page',
		singular_label: 'Page',
		plural_label: 'Pages',
		route_word: 'pages',
	})
	expect(created.key).toBe('page')
})

test('carries the reason a type was refused', async () => {
	server.use(
		http.post('/api/types', () =>
			HttpResponse.json({ error: 'content: the route word is taken' }, { status: 422 }),
		),
	)

	await expect(
		createType({ key: 'page', singularLabel: 'Page', pluralLabel: 'Pages', routeWord: 'pages' }),
	).rejects.toThrow(/route word is taken/)
})

test('edits only the fields it was given', async () => {
	const sent: unknown[] = []
	server.use(
		http.patch('/api/types/page', async ({ request }) => {
			sent.push(await request.json())
			return HttpResponse.json(PAGE_ROW)
		}),
	)

	await updateType('page', { routeWord: 'sections' })

	expect(sent[0]).toEqual({ route_word: 'sections' })
})

test('removes a type that holds nothing', async () => {
	let asked = ''
	server.use(
		http.delete('/api/types/page', ({ request }) => {
			asked = new URL(request.url).pathname
			return new HttpResponse(null, { status: 204 })
		}),
	)

	await deleteType('page')

	expect(asked).toBe('/api/types/page')
})

test('reports an edit the registry could not take', async () => {
	server.use(http.patch('/api/types/page', () => new HttpResponse(null, { status: 503 })))

	await expect(updateType('page', { active: false })).rejects.toThrow(/503/)
})

test('sends every field an edit names', async () => {
	const sent: unknown[] = []
	server.use(
		http.patch('/api/types/page', async ({ request }) => {
			sent.push(await request.json())
			return HttpResponse.json(PAGE_ROW)
		}),
	)

	await updateType('page', {
		singularLabel: 'Section',
		pluralLabel: 'Sections',
		routeWord: 'sections',
		isDefault: true,
		active: true,
	})

	expect(sent[0]).toEqual({
		singular_label: 'Section',
		plural_label: 'Sections',
		route_word: 'sections',
		default: true,
		active: true,
	})
})

test('reports a removal the registry could not take', async () => {
	server.use(http.delete('/api/types/page', () => new HttpResponse(null, { status: 503 })))

	await expect(deleteType('page')).rejects.toThrow(/503/)
})
