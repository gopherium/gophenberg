// SPDX-License-Identifier: Apache-2.0

import { http, HttpResponse, server } from '@gophenberg/frontend-sdk/testing'
import { expect, test } from 'vitest'

import {
	deletePost,
	emptyTrash,
	fetchPost,
	listEveryPost,
	listPosts,
	restorePost,
	trashPost,
} from '../content/api'

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

/** The writes the server can refuse, each with the words its refusal falls back to. */
const REFUSABLE = [
	{
		action: 'trash',
		run: () => trashPost(ROW.id),
		refuse: (answer: () => Response) => server.use(http.delete(`/api/content/${ROW.id}`, answer)),
		fallback: 'The item could not be moved to the trash.',
	},
	{
		action: 'restore',
		run: () => restorePost(ROW.id),
		refuse: (answer: () => Response) => server.use(http.post(`/api/content/${ROW.id}/restore`, answer)),
		fallback: 'The item could not be restored.',
	},
	{
		action: 'permanent delete',
		run: () => deletePost(ROW.id),
		refuse: (answer: () => Response) => server.use(http.delete(`/api/content/${ROW.id}`, answer)),
		fallback: 'The item could not be permanently deleted.',
	},
	{
		action: 'empty trash',
		run: () => emptyTrash('post'),
		refuse: (answer: () => Response) => server.use(http.delete('/api/content/trash', answer)),
		fallback: 'The trash could not be emptied.',
	},
]

test.each(REFUSABLE)('reads a refused $action as the words its code stands for', async ({ run, refuse }) => {
	refuse(() =>
		HttpResponse.json(
			{ error: 'content: item holds children', code: 'content_holds_children' },
			{ status: 422 },
		),
	)

	await expect(run()).rejects.toEqual(
		new Error('This item still holds items nested inside it. Move or delete those first.'),
	)
})

test.each(REFUSABLE)('reads a refused $action with no reason as its own words', async ({ run, refuse, fallback }) => {
	refuse(() => HttpResponse.json({}, { status: 500 }))

	await expect(run()).rejects.toEqual(new Error(fallback))
})

test.each(REFUSABLE)('reads a refused $action with an unreadable body as its own words', async (refusable) => {
	const { run, refuse, fallback } = refusable
	refuse(() => new HttpResponse('not json', { status: 502 }))

	await expect(run()).rejects.toEqual(new Error(fallback))
})

test.each(REFUSABLE)('never shows the server prose for a refused $action', async ({ run, refuse, fallback }) => {
	refuse(() => HttpResponse.json({ error: 'content: something new', code: 'a_code_from_the_future' }, { status: 422 }))

	await expect(run()).rejects.toEqual(new Error(fallback))
})

test.each(REFUSABLE)('reads the $action the network dropped as its own words', async ({ run, refuse, fallback }) => {
	refuse(() => HttpResponse.error())

	await expect(run()).rejects.toEqual(new Error(fallback))
})

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

	await listPosts({ status: 'draft', search: 'gutenberg', page: 2, perPage: 50, orderBy: 'title', order: 'asc' })

	const asked = new URLSearchParams(queries[0])
	expect(asked.get('status')).toBe('draft')
	expect(asked.get('search')).toBe('gutenberg')
	expect(asked.get('page')).toBe('2')
	expect(asked.get('orderby')).toBe('title')
	expect(asked.get('order')).toBe('asc')
	expect(asked.get('per_page')).toBe('50')
})

test('omits filters that were not asked for', async () => {
	const queries = captureList()

	await listPosts({})

	const asked = new URLSearchParams(queries[0])
	expect(asked.has('status')).toBe(false)
	expect(asked.has('search')).toBe(false)
})

test('asks no page size while none is handed, so the server pages at its own', async () => {
	const queries = captureList()

	await listPosts({ type: 'post' })

	expect(new URLSearchParams(queries[0]).has('per_page')).toBe(false)
})

test('reads the page size the server says it used', async () => {
	server.use(http.get('/api/content', () => HttpResponse.json({ items: [ROW], total: 1, per_page: 7 })))

	const page = await listPosts({})

	expect(page.perPage).toBe(7)
})

test('reads the title of the parent each listed item sits under', async () => {
	server.use(
		http.get('/api/content', () =>
			HttpResponse.json({ items: [{ ...ROW, parent_id: 'p1', parent_title: 'About Us' }], total: 1 }),
		),
	)

	const page = await listPosts({})

	expect(page.items[0].parentTitle).toBe('About Us')
})

test('reads a listed item that names no moments and no parent as dated nowhere, at the top of its tree', async () => {
	const bare = { id: ROW.id, type: 'post', slug: 'welcome', title: 'Welcome', status: 'draft' }
	server.use(http.get('/api/content', () => HttpResponse.json({ items: [bare], total: 1 })))

	const page = await listPosts({})

	expect(page.items[0]).toMatchObject({ parentTitle: '', publishedAt: null, createdAt: '', updatedAt: '', date: '' })
})

test('dates a listed item by its publication, else by its last save, as the server sorts it', async () => {
	server.use(
		http.get('/api/content', () =>
			HttpResponse.json({ items: [ROW, { ...ROW, id: 'd1', published_at: null }], total: 2 }),
		),
	)

	const page = await listPosts({})

	expect(page.items.map((post) => post.date)).toEqual([ROW.published_at, ROW.updated_at])
})

test('reads no page size from a server that names none', async () => {
	captureList()

	const page = await listPosts({})

	expect(page.perPage).toBe(0)
})

test('reports a failed listing', async () => {
	server.use(http.get('/api/content', () => HttpResponse.json({}, { status: 500 })))

	await expect(listPosts({})).rejects.toThrow(/500/)
})

test('lists the type it was asked for', async () => {
	const queries = captureList()

	await listPosts({ type: 'page' })

	expect(new URLSearchParams(queries[0]).get('type')).toBe('page')
})

/**
 * Serves a listing of the given total, page by page at the size asked, or at the given size when none is asked.
 * @param total - How many items the listing holds.
 * @param served - The page size the server uses when the request names none.
 * @returns The query strings asked for.
 */
function servePaged(total: number, served: number): string[] {
	const queries: string[] = []
	server.use(
		http.get('/api/content', ({ request }) => {
			const params = new URL(request.url).searchParams
			queries.push(params.toString())
			const page = Number(params.get('page') ?? '1')
			const perPage = Number(params.get('per_page') ?? served)
			const held = Math.max(0, Math.min(perPage, total - (page - 1) * perPage))
			return HttpResponse.json({
				items: Array.from({ length: held }, (_, i) => ({
					...ROW,
					id: `019fb000-0000-7000-8000-${String((page - 1) * perPage + i).padStart(12, '0')}`,
				})),
				total,
				per_page: perPage,
			})
		}),
	)
	return queries
}

test('reads every post of a type page by page at the largest size the settings name', async () => {
	const queries = servePaged(130, 20)

	const held = await listEveryPost({ type: 'category' }, [10, 75, 50])

	expect(held).toHaveLength(130)
	expect(new Set(held.map((post) => post.id)).size).toBe(130)
	expect(queries.map((query) => new URLSearchParams(query).get('per_page'))).toEqual(['75', '75'])
})

test('reads every post of a type at the size the server serves when no size is handed', async () => {
	const queries = servePaged(45, 20)

	const held = await listEveryPost({ type: 'category' })

	expect(held).toHaveLength(45)
	expect(queries).toHaveLength(3)
	expect(queries.every((query) => !new URLSearchParams(query).has('per_page'))).toBe(true)
})

test('stops reading pages when the listing reports nothing', async () => {
	server.use(http.get('/api/content', () => HttpResponse.json({ items: [], total: 500 })))

	await expect(listEveryPost({ type: 'category' })).resolves.toEqual([])
})

test('stops reading pages once a later page comes back empty', async () => {
	const queries: string[] = []
	server.use(
		http.get('/api/content', ({ request }) => {
			queries.push(new URL(request.url).search)
			const first = !new URL(request.url).searchParams.has('page')
			return HttpResponse.json({ items: first ? [ROW] : [], total: 500, per_page: 1 })
		}),
	)

	const held = await listEveryPost({ type: 'category' })

	expect(held).toHaveLength(1)
	expect(queries).toHaveLength(2)
})

test('reads no more pages than the total of the first answer names', async () => {
	const queries: string[] = []
	server.use(
		http.get('/api/content', ({ request }) => {
			queries.push(new URL(request.url).search)
			const first = !new URL(request.url).searchParams.has('page')
			return HttpResponse.json({ items: [ROW], total: first ? 3 : Number.MAX_SAFE_INTEGER, per_page: 1 })
		}),
	)

	const held = await listEveryPost({ type: 'category' })

	expect(held).toHaveLength(3)
	expect(queries).toHaveLength(3)
})

test('reads one page from a server that names no page size', async () => {
	server.use(http.get('/api/content', () => HttpResponse.json({ items: [ROW], total: 1 })))

	await expect(listEveryPost({ type: 'category' })).resolves.toHaveLength(1)
})

test('empties the trash of the type it was asked for in one call', async () => {
	const asked: URL[] = []
	const each: string[] = []
	server.use(
		http.delete('/api/content/trash', ({ request }) => {
			asked.push(new URL(request.url))
			return HttpResponse.json({ deleted: 3, kept: 1 })
		}),
		http.delete('/api/content/:id', ({ params }) => {
			each.push(String(params.id))
			return new HttpResponse(null, { status: 204 })
		}),
	)

	const emptied = await emptyTrash('page')

	expect(emptied).toEqual({ deleted: 3, kept: 1 })
	expect(asked.map((url) => url.searchParams.get('type'))).toEqual(['page'])
	expect(each).toEqual([])
})

test('reads how many items point at the one it fetched', async () => {
	server.use(
		http.get(`/api/content/${ROW.id}`, () =>
			HttpResponse.json({
				...ROW,
				content: '',
				fields: { 'linked-from': [] },
				field_totals: { 'linked-from': 47 },
			}),
		),
	)

	const post = await fetchPost(ROW.id)

	expect(post.fieldTotals).toEqual({ 'linked-from': 47 })
})

test('counts nothing pointing at an item the server said nothing about', async () => {
	server.use(
		http.get(`/api/content/${ROW.id}`, () => HttpResponse.json({ ...ROW, content: '' })),
	)

	const post = await fetchPost(ROW.id)

	expect(post.fieldTotals).toEqual({})
})
