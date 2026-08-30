// SPDX-License-Identifier: Apache-2.0

import { http, HttpResponse, server } from '@gophenberg/frontend-sdk/testing'
import { screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { beforeAll, beforeEach, expect, test } from 'vitest'

import { renderAt } from './render'
import { storedPost } from './postFixture'

const EDITOR_PATH = `/content/post/${storedPost.id}/edit`

const NEWS = { id: '019fb000-0000-7000-8000-0000000000a1', title: 'News' }

const GUIDES = { id: '019fb000-0000-7000-8000-0000000000a2', title: 'Guides' }

/**
 * Returns the post type declaring the given fields.
 * @param fields - The field definitions the type declares.
 * @returns The type as the registry answers it.
 */
function typeDeclaring(fields: unknown[]) {
	return {
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
		fields,
	}
}

const ONE_CATEGORY = {
	key: 'category',
	label: 'Category',
	kind: 'relation',
	relates_to: 'category',
	many: false,
	required: false,
	updated_at: '2026-08-01T10:00:00Z',
}

const MANY_CATEGORIES = { ...ONE_CATEGORY, key: 'categories', label: 'Categories', many: true }

beforeAll(async () => {
	await import('../content/EditorScreen')
}, 120000)

beforeEach(() => {
	server.use(
		http.get(`/api/content/${storedPost.id}`, () => HttpResponse.json(storedPost)),
		http.get('/api/content', ({ request }) => {
			if (new URL(request.url).searchParams.get('type') !== 'category') {
				return HttpResponse.json({ items: [], total: 0 })
			}
			return HttpResponse.json({
				items: [
					{ ...storedPost, ...NEWS, type: 'category', slug: 'news' },
					{ ...storedPost, ...GUIDES, type: 'category', slug: 'guides' },
				],
				total: 2,
			})
		}),
	)
})

test('leaves a trashed item out of what a relation field may point at', async () => {
	server.use(
		http.get('/api/types', () => HttpResponse.json({ items: [typeDeclaring([ONE_CATEGORY])] })),
		http.get('/api/content', () =>
			HttpResponse.json({
				items: [
					{ ...storedPost, ...NEWS, type: 'category', slug: 'news' },
					{ ...storedPost, ...GUIDES, type: 'category', slug: 'guides', status: 'trash' },
				],
				total: 2,
			}),
		),
	)
	renderAt(EDITOR_PATH)

	const picker = await screen.findByLabelText('Category')
	await userEvent.click(picker)

	expect(await screen.findByRole('option', { name: 'News' })).toBeInTheDocument()
	expect(screen.queryByRole('option', { name: 'Guides' })).not.toBeInTheDocument()
})

test('offers the items of the type a relation field points at', async () => {
	server.use(http.get('/api/types', () => HttpResponse.json({ items: [typeDeclaring([ONE_CATEGORY])] })))
	renderAt(EDITOR_PATH)

	const picker = await screen.findByLabelText('Category')
	await userEvent.click(picker)

	expect(await screen.findByRole('option', { name: 'News' })).toBeInTheDocument()
	expect(screen.getByRole('option', { name: 'Guides' })).toBeInTheDocument()
})

test('sends the target a relation field was pointed at', async () => {
	const sent: Record<string, unknown>[] = []
	server.use(
		http.get('/api/types', () => HttpResponse.json({ items: [typeDeclaring([ONE_CATEGORY])] })),
		http.patch(`/api/content/${storedPost.id}`, async ({ request }) => {
			sent.push((await request.json()) as Record<string, unknown>)
			return HttpResponse.json({ ...storedPost, fields: { category: [NEWS.id] } })
		}),
	)
	renderAt(EDITOR_PATH)

	await userEvent.click(await screen.findByLabelText('Category'))
	await userEvent.click(await screen.findByRole('option', { name: 'News' }))
	await userEvent.click(screen.getByRole('button', { name: 'Save draft' }))

	await waitFor(() => expect(sent).toHaveLength(1))
	expect(sent[0]).toMatchObject({ fields: { category: [NEWS.id] } })
})

test('shows the targets a many field already holds', async () => {
	server.use(
		http.get('/api/types', () => HttpResponse.json({ items: [typeDeclaring([MANY_CATEGORIES])] })),
		http.get(`/api/content/${storedPost.id}`, () =>
			HttpResponse.json({ ...storedPost, fields: { categories: [NEWS.id] } }),
		),
	)
	renderAt(EDITOR_PATH)

	const held = await screen.findByRole('list', { name: 'Categories' })

	expect(await within(held).findByText('News')).toBeInTheDocument()
})

test('adds a second target to a many field', async () => {
	const sent: Record<string, unknown>[] = []
	server.use(
		http.get('/api/types', () => HttpResponse.json({ items: [typeDeclaring([MANY_CATEGORIES])] })),
		http.get(`/api/content/${storedPost.id}`, () =>
			HttpResponse.json({ ...storedPost, fields: { categories: [NEWS.id] } }),
		),
		http.patch(`/api/content/${storedPost.id}`, async ({ request }) => {
			sent.push((await request.json()) as Record<string, unknown>)
			return HttpResponse.json({ ...storedPost, fields: { categories: [NEWS.id, GUIDES.id] } })
		}),
	)
	renderAt(EDITOR_PATH)

	await userEvent.click(await screen.findByLabelText('Add Categories'))
	await userEvent.click(await screen.findByRole('option', { name: 'Guides' }))
	await userEvent.click(screen.getByRole('button', { name: 'Save draft' }))

	await waitFor(() => expect(sent).toHaveLength(1))
	expect(sent[0]).toMatchObject({ fields: { categories: [NEWS.id, GUIDES.id] } })
})

test('takes a target off a many field', async () => {
	const sent: Record<string, unknown>[] = []
	server.use(
		http.get('/api/types', () => HttpResponse.json({ items: [typeDeclaring([MANY_CATEGORIES])] })),
		http.get(`/api/content/${storedPost.id}`, () =>
			HttpResponse.json({ ...storedPost, fields: { categories: [NEWS.id] } }),
		),
		http.patch(`/api/content/${storedPost.id}`, async ({ request }) => {
			sent.push((await request.json()) as Record<string, unknown>)
			return HttpResponse.json({ ...storedPost, fields: { categories: [] } })
		}),
	)
	renderAt(EDITOR_PATH)
	const held = await screen.findByRole('list', { name: 'Categories' })

	await userEvent.click(await within(held).findByRole('button', { name: 'Remove News' }))
	await userEvent.click(screen.getByRole('button', { name: 'Save draft' }))

	await waitFor(() => expect(sent).toHaveLength(1))
	expect(sent[0]).toMatchObject({ fields: { categories: [] } })
})

test('leaves the item itself out of a relation to its own type', async () => {
	const ownType = { ...ONE_CATEGORY, key: 'related', label: 'Related', relates_to: 'post' }
	server.use(
		http.get('/api/types', () => HttpResponse.json({ items: [typeDeclaring([ownType])] })),
		http.get('/api/content', () =>
			HttpResponse.json({
				items: [storedPost, { ...storedPost, ...NEWS, slug: 'news' }],
				total: 2,
			}),
		),
	)
	renderAt(EDITOR_PATH)

	await userEvent.click(await screen.findByLabelText('Related'))

	expect(await screen.findByRole('option', { name: 'News' })).toBeInTheDocument()
	expect(screen.queryByRole('option', { name: storedPost.title })).not.toBeInTheDocument()
})

test('holds the media a media field was pointed at', async () => {
	const sent: Record<string, unknown>[] = []
	const mediaField = {
		key: 'cover',
		label: 'Cover',
		kind: 'media',
		relates_to: '',
		many: false,
		required: false,
		updated_at: '2026-08-01T10:00:00Z',
	}
	server.use(
		http.get('/api/types', () => HttpResponse.json({ items: [typeDeclaring([mediaField])] })),
		http.get('/api/media', () =>
			HttpResponse.json({
				items: [
					{
						id: 12,
						type: 'image',
						file: '2026/08/shore.jpg',
						title: 'Shore',
						alt_text: 'A quiet shore',
						caption: '',
						mime_type: 'image/jpeg',
						width: 1600,
						height: 1000,
						filesize: 254000,
						sizes: {},
						updated_at: storedPost.updated_at,
					},
				],
				total: 1,
			}),
		),
		http.patch(`/api/content/${storedPost.id}`, async ({ request }) => {
			sent.push((await request.json()) as Record<string, unknown>)
			return HttpResponse.json({ ...storedPost, fields: { cover: 12 } })
		}),
	)
	renderAt(EDITOR_PATH)

	await userEvent.click(await screen.findByRole('button', { name: 'Choose Cover' }))
	await userEvent.click(await screen.findByText('Shore'))
	await userEvent.click(screen.getByRole('button', { name: /^Select$/ }))
	await userEvent.click(screen.getByRole('button', { name: 'Save draft' }))

	await waitFor(() => expect(sent).toHaveLength(1))
	expect(sent[0]).toMatchObject({ fields: { cover: 12 } })
})

test('clears the media a field held', async () => {
	const sent: Record<string, unknown>[] = []
	const mediaField = {
		key: 'cover',
		label: 'Cover',
		kind: 'media',
		relates_to: '',
		many: false,
		required: false,
		updated_at: '2026-08-01T10:00:00Z',
	}
	server.use(
		http.get('/api/types', () => HttpResponse.json({ items: [typeDeclaring([mediaField])] })),
		http.get(`/api/content/${storedPost.id}`, () =>
			HttpResponse.json({ ...storedPost, fields: { cover: 12 } }),
		),
		http.patch(`/api/content/${storedPost.id}`, async ({ request }) => {
			sent.push((await request.json()) as Record<string, unknown>)
			return HttpResponse.json({ ...storedPost, fields: {} })
		}),
	)
	renderAt(EDITOR_PATH)

	await userEvent.click(await screen.findByRole('button', { name: 'Clear Cover' }))
	await userEvent.click(screen.getByRole('button', { name: 'Save draft' }))

	await waitFor(() => expect(sent).toHaveLength(1))
	expect(sent[0]).toMatchObject({ fields: { cover: null } })
})

test('shows none when the target held is no longer offered', async () => {
	server.use(
		http.get('/api/types', () => HttpResponse.json({ items: [typeDeclaring([ONE_CATEGORY])] })),
		http.get(`/api/content/${storedPost.id}`, () =>
			HttpResponse.json({ ...storedPost, fields: { category: ['019fb000-0000-7000-8000-0000000000ff'] } }),
		),
	)
	renderAt(EDITOR_PATH)

	const picker = await screen.findByLabelText('Category')
	await userEvent.click(picker)

	expect(await screen.findByRole('option', { name: 'News' })).toBeInTheDocument()
	expect(picker).not.toHaveTextContent('019fb000')
})

test('adds nothing when the placeholder is chosen', async () => {
	const sent: Record<string, unknown>[] = []
	server.use(
		http.get('/api/types', () => HttpResponse.json({ items: [typeDeclaring([MANY_CATEGORIES])] })),
		http.get(`/api/content/${storedPost.id}`, () =>
			HttpResponse.json({ ...storedPost, fields: { categories: [NEWS.id] } }),
		),
		http.patch(`/api/content/${storedPost.id}`, async ({ request }) => {
			sent.push((await request.json()) as Record<string, unknown>)
			return HttpResponse.json({ ...storedPost, fields: { categories: [NEWS.id] } })
		}),
	)
	renderAt(EDITOR_PATH)

	await userEvent.click(await screen.findByLabelText('Add Categories'))
	await userEvent.click(await screen.findByRole('option', { name: 'Add Categories' }))
	await userEvent.type(screen.getByRole('textbox', { name: 'Title' }), '!')
	await userEvent.click(screen.getByRole('button', { name: 'Save draft' }))

	await waitFor(() => expect(sent).toHaveLength(1))
	expect(sent[0].fields).toEqual({})
})

const GALLERY_FIELD = {
	key: 'gallery',
	label: 'Gallery',
	kind: 'media',
	relates_to: '',
	many: true,
	required: false,
	updated_at: '2026-08-01T10:00:00Z',
}

const SHORE_LISTING = {
	items: [
		{
			id: 15,
			type: 'image',
			file: '2026/08/shore.jpg',
			title: 'Shore',
			alt_text: 'A quiet shore',
			caption: '',
			mime_type: 'image/jpeg',
			width: 1600,
			height: 1000,
			filesize: 254000,
			sizes: {},
			updated_at: storedPost.updated_at,
		},
	],
	total: 1,
}

test('adds a picked item to the gallery a field holds', async () => {
	const sent: Record<string, unknown>[] = []
	server.use(
		http.get('/api/types', () => HttpResponse.json({ items: [typeDeclaring([GALLERY_FIELD])] })),
		http.get(`/api/content/${storedPost.id}`, () =>
			HttpResponse.json({ ...storedPost, fields: { gallery: [12] } }),
		),
		http.get('/api/media', () => HttpResponse.json(SHORE_LISTING)),
		http.patch(`/api/content/${storedPost.id}`, async ({ request }) => {
			sent.push((await request.json()) as Record<string, unknown>)
			return HttpResponse.json({ ...storedPost, fields: { gallery: [12, 15] } })
		}),
	)
	renderAt(EDITOR_PATH)

	expect(await screen.findByText('Media 12')).toBeInTheDocument()
	await userEvent.click(screen.getByRole('button', { name: 'Add to Gallery' }))
	await userEvent.click(await screen.findByText('Shore'))
	await userEvent.click(screen.getByRole('button', { name: /^Select$/ }))
	await userEvent.click(screen.getByRole('button', { name: 'Save draft' }))

	await waitFor(() => expect(sent).toHaveLength(1))
	expect(sent[0]).toMatchObject({ fields: { gallery: [12, 15] } })
})

test('never adds the item the gallery already holds', async () => {
	server.use(
		http.get('/api/types', () => HttpResponse.json({ items: [typeDeclaring([GALLERY_FIELD])] })),
		http.get(`/api/content/${storedPost.id}`, () =>
			HttpResponse.json({ ...storedPost, fields: { gallery: [15] } }),
		),
		http.get('/api/media', () => HttpResponse.json(SHORE_LISTING)),
	)
	renderAt(EDITOR_PATH)

	await userEvent.click(await screen.findByRole('button', { name: 'Add to Gallery' }))
	await userEvent.click(await screen.findByText('Shore'))
	await userEvent.click(screen.getByRole('button', { name: /^Select$/ }))

	expect(screen.getAllByText('Media 15')).toHaveLength(1)
})

test('removes one item from the gallery and clears an emptied one', async () => {
	const sent: Record<string, unknown>[] = []
	server.use(
		http.get('/api/types', () => HttpResponse.json({ items: [typeDeclaring([GALLERY_FIELD])] })),
		http.get(`/api/content/${storedPost.id}`, () =>
			HttpResponse.json({ ...storedPost, fields: { gallery: [12, 15] } }),
		),
		http.patch(`/api/content/${storedPost.id}`, async ({ request }) => {
			sent.push((await request.json()) as Record<string, unknown>)
			return HttpResponse.json({ ...storedPost, fields: {} })
		}),
	)
	renderAt(EDITOR_PATH)

	await userEvent.click(await screen.findByRole('button', { name: 'Remove Media 12' }))
	await userEvent.click(screen.getByRole('button', { name: 'Remove Media 15' }))
	await userEvent.click(screen.getByRole('button', { name: 'Save draft' }))

	await waitFor(() => expect(sent).toHaveLength(1))
	expect(sent[0]).toMatchObject({ fields: { gallery: null } })
})
