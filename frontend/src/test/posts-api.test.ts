// SPDX-License-Identifier: Apache-2.0

import { http, HttpResponse, server } from '@gophenberg/frontend-sdk/testing'
import { expect, test } from 'vitest'

import { fetchPostCounts, listPosts } from '../content/api'

const ROW = {
	id: '019fb000-0000-7000-8000-000000000001',
	type: 'post',
	slug: 'welcome',
	title: 'Welcome to Gophenberg',
	excerpt: '',
	status: 'published',
	author_id: '019fb000-0000-7000-8000-0000000000ff',
	author_name: 'Maria Perez',
	published_at: '2026-07-20T10:00:00Z',
	created_at: '2026-07-19T10:00:00Z',
	updated_at: '2026-07-20T10:00:00Z',
}

/**
 * Serves one page of posts and records the query it was asked for.
 * @returns The recorded query strings.
 */
function captureList(): string[] {
	const queries: string[] = []
	server.use(
		http.get('/api/content', ({ request }) => {
			queries.push(new URL(request.url).search)
			return HttpResponse.json({ items: [ROW], total: 1 })
		}),
	)
	return queries
}

test('reads a page of posts with its total', async () => {
	captureList()

	const page = await listPosts({})

	expect(page.total).toBe(1)
	expect(page.items[0].title).toBe('Welcome to Gophenberg')
	expect(page.items[0].authorName).toBe('Maria Perez')
})

test('asks only for the parameters it was given', async () => {
	const queries = captureList()

	await listPosts({ status: 'draft', search: 'gutenberg', page: 2, orderBy: 'title', order: 'asc' })

	const asked = new URLSearchParams(queries[0])
	expect(asked.get('status')).toBe('draft')
	expect(asked.get('search')).toBe('gutenberg')
	expect(asked.get('page')).toBe('2')
	expect(asked.get('orderby')).toBe('title')
	expect(asked.get('order')).toBe('asc')
	expect(asked.get('per_page')).toBe('20')
})

test('omits filters that were not asked for', async () => {
	const queries = captureList()

	await listPosts({})

	const asked = new URLSearchParams(queries[0])
	expect(asked.has('status')).toBe(false)
	expect(asked.has('search')).toBe(false)
})

test('reports a failed listing', async () => {
	server.use(http.get('/api/content', () => HttpResponse.json({}, { status: 500 })))

	await expect(listPosts({})).rejects.toThrow(/500/)
})

test('reads the count of every status', async () => {
	server.use(
		http.get('/api/content/counts', () =>
			HttpResponse.json({ draft: 3, published: 2, pending: 0, private: 0, trash: 1 }),
		),
	)

	const counts = await fetchPostCounts()

	expect(counts.draft).toBe(3)
	expect(counts.trash).toBe(1)
})

test('reports a failed count', async () => {
	server.use(http.get('/api/content/counts', () => HttpResponse.json({}, { status: 500 })))

	await expect(fetchPostCounts()).rejects.toThrow(/500/)
})

test('lists the type it was asked for', async () => {
	const queries = captureList()

	await listPosts({ type: 'page' })

	expect(new URLSearchParams(queries[0]).get('type')).toBe('page')
})

test('counts the type it was asked for', async () => {
	const queries: string[] = []
	server.use(
		http.get('/api/content/counts', ({ request }) => {
			queries.push(new URL(request.url).search)
			return HttpResponse.json({ draft: 1 })
		}),
	)

	await fetchPostCounts('page')

	expect(new URLSearchParams(queries[0]).get('type')).toBe('page')
})
