// SPDX-License-Identifier: Apache-2.0

import { http, HttpResponse, server } from '@gophenberg/frontend-sdk/testing'
import { screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { resetLocaleData, setLocaleData } from '@wordpress/i18n'
import { beforeAll, beforeEach, expect, onTestFinished, test } from 'vitest'

import { catalogFor } from '../i18n/catalog'
import { renderAt, renderRoutedAt } from './render'
import { storedPost } from './postFixture'
import { siteSettings } from './siteSettings'

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
		description: '',
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

/**
 * Presses Select in the open picker and waits for its dialog to leave.
 */
async function pressSelect() {
	await userEvent.click(screen.getByRole('button', { name: /^Select$/ }))
	await waitFor(() => expect(screen.queryByRole('dialog')).not.toBeInTheDocument())
}

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
				per_page: siteSettings.picker_page_size,
			}),
		),
		http.patch(`/api/content/${storedPost.id}`, async ({ request }) => {
			sent.push((await request.json()) as Record<string, unknown>)
			return HttpResponse.json({ ...storedPost, fields: { cover: 12 } })
		}),
	)
	renderAt(EDITOR_PATH)

	await userEvent.click(await screen.findByRole('button', { name: 'Choose Cover' }))
	await userEvent.click(await screen.findByRole('option', { name: 'Shore' }))
	await pressSelect()
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

test('names an empty pick Ninguno to a Spanish reader, apart from the Ninguna of the parent column', async () => {
	setLocaleData(await catalogFor('es-ES'), 'gophenberg')
	onTestFinished(() => resetLocaleData({}, 'gophenberg'))
	server.use(http.get('/api/types', () => HttpResponse.json({ items: [typeDeclaring([ONE_CATEGORY])] })))
	renderAt(EDITOR_PATH)

	await userEvent.click(await screen.findByLabelText('Category'))

	expect(await screen.findByRole('option', { name: 'Ninguno' })).toBeInTheDocument()
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

const HARBOR = {
	...SHORE_LISTING.items[0],
	id: 12,
	file: '2026/08/harbor.jpg',
	title: 'Harbor',
	alt_text: 'Boats at rest',
}

const DEED = {
	...SHORE_LISTING.items[0],
	id: 9,
	type: 'document',
	file: '2026/08/deed.pdf',
	title: 'Deed',
	alt_text: '',
	mime_type: 'application/pdf',
}

const PIER = {
	...SHORE_LISTING.items[0],
	id: 20,
	file: '2026/08/pier.jpg',
	title: 'Pier',
	alt_text: '',
}

const LIBRARY = [HARBOR, SHORE_LISTING.items[0], DEED, PIER]

/**
 * Serves the gallery field, a stored post holding the given files, and the library naming them.
 * @param held - The identities the gallery holds.
 * @returns The bodies the editor sent back and the library queries it made.
 */
function galleryOf(held: number[]) {
	const sent: Record<string, unknown>[] = []
	const asked: string[] = []
	server.use(
		http.get('/api/types', () => HttpResponse.json({ items: [typeDeclaring([GALLERY_FIELD])] })),
		http.get(`/api/content/${storedPost.id}`, () =>
			HttpResponse.json({ ...storedPost, fields: { gallery: held } }),
		),
		http.get('/api/media', ({ request }) => {
			const query = new URL(request.url).searchParams
			asked.push(query.toString())
			const ids = query.get('ids')
			if (ids === null) {
				return HttpResponse.json({ items: LIBRARY, total: LIBRARY.length, per_page: siteSettings.picker_page_size })
			}
			const named = LIBRARY.filter((item) => ids.split(',').includes(String(item.id)))
			return HttpResponse.json({ items: named, total: named.length })
		}),
		http.patch(`/api/content/${storedPost.id}`, async ({ request }) => {
			sent.push((await request.json()) as Record<string, unknown>)
			return HttpResponse.json({ ...storedPost, fields: { gallery: held } })
		}),
	)
	return { sent, asked }
}

test('shows each file a gallery holds by its picture and name, read in one call', async () => {
	const { asked } = galleryOf([12, 15])
	renderAt(EDITOR_PATH)

	const listed = await screen.findByRole('list', { name: 'Gallery' })

	expect(await within(listed).findByRole('img', { name: 'Boats at rest' })).toHaveAttribute(
		'src',
		'/media/2026/08/harbor.jpg',
	)
	expect(within(listed).getByText('Shore')).toBeInTheDocument()
	expect(asked.filter((query) => query.includes('ids='))).toEqual(['ids=12%2C15'])
})

test('reads a gallery of more files than the largest served size in calls of that size', async () => {
	server.use(http.get('/api/settings', () => HttpResponse.json({ ...siteSettings, list_page_sizes: [1, 2] })))
	const { asked } = galleryOf([12, 15, 9, 20])
	renderAt(EDITOR_PATH)

	const listed = await screen.findByRole('list', { name: 'Gallery' })

	expect(await within(listed).findByText('Pier')).toBeInTheDocument()
	expect(asked.filter((query) => query.includes('ids='))).toEqual(['ids=12%2C15', 'ids=9%2C20'])
})

test('adds every file picked at once to the gallery a field holds', async () => {
	const { sent } = galleryOf([])
	renderAt(EDITOR_PATH)

	await userEvent.click(await screen.findByRole('button', { name: 'Add to Gallery' }))
	const picker = await screen.findByRole('dialog', { name: 'Select Media' })
	await userEvent.click(await within(picker).findByRole('option', { name: 'Harbor' }))
	await userEvent.click(within(picker).getByRole('option', { name: 'Shore' }))
	await pressSelect()
	await userEvent.click(screen.getByRole('button', { name: 'Save draft' }))

	await waitFor(() => expect(sent).toHaveLength(1))
	expect(sent[0]).toMatchObject({ fields: { gallery: [12, 15] } })
})

test('never adds the item the gallery already holds', async () => {
	galleryOf([15])
	renderAt(EDITOR_PATH)

	await userEvent.click(await screen.findByRole('button', { name: 'Add to Gallery' }))
	const picker = await screen.findByRole('dialog', { name: 'Select Media' })
	await userEvent.click(await within(picker).findByRole('option', { name: 'Shore' }))
	await pressSelect()

	const listed = screen.getByRole('list', { name: 'Gallery' })
	expect(within(listed).getAllByRole('listitem')).toHaveLength(1)
})

test('leaves the address alone while a reader searches, pages and sorts the picker', async () => {
	const asked: URLSearchParams[] = []
	galleryOf([])
	server.use(
		http.get('/api/media', ({ request }) => {
			asked.push(new URL(request.url).searchParams)
			return HttpResponse.json({ items: LIBRARY, total: 120, per_page: siteSettings.picker_page_size })
		}),
	)
	const { router } = renderRoutedAt(EDITOR_PATH)
	await userEvent.click(await screen.findByRole('button', { name: 'Add to Gallery' }))
	const picker = await screen.findByRole('dialog', { name: 'Select Media' })
	await within(picker).findByRole('option', { name: 'Harbor' })
	const address = router.state.location.href

	await userEvent.type(within(picker).getByRole('searchbox', { name: 'Search media…' }), 'harbor')
	await waitFor(() => expect(asked.at(-1)?.get('search')).toBe('harbor'))
	await userEvent.click(within(picker).getByRole('button', { name: 'Next page' }))
	await waitFor(() => expect(asked.at(-1)?.get('page')).toBe('2'))
	await userEvent.click(within(picker).getByRole('button', { name: 'View options' }))
	await userEvent.selectOptions(await screen.findByRole('combobox', { name: 'Sort by' }), 'title')
	await waitFor(() => expect(asked.at(-1)?.get('orderby')).toBe('title'))

	expect(router.state.location.href).toBe(address)
})

/**
 * Opens the picker behind a media field's button, then its View options.
 * @param choose - The button that opens the picker.
 * @returns The picker, and the Sort by control inside it.
 */
async function sortOfPickerFrom(choose: HTMLElement) {
	await userEvent.click(choose)
	const picker = await screen.findByRole('dialog', { name: 'Select Media' })
	await within(picker).findByRole('option', { name: 'Harbor' })
	await userEvent.click(within(picker).getByRole('button', { name: 'View options' }))
	return { picker, sort: await within(picker).findByRole('combobox', { name: 'Sort by' }) }
}

test('holds a sort chosen in the picker of one media field when the next field opens its own', async () => {
	const fields = [
		{ ...GALLERY_FIELD, key: 'cover', label: 'Cover', many: false },
		{ ...GALLERY_FIELD, key: 'hero', label: 'Hero', many: false },
	]
	galleryOf([])
	server.use(http.get('/api/types', () => HttpResponse.json({ items: [typeDeclaring(fields)] })))
	renderAt(EDITOR_PATH)
	const first = await sortOfPickerFrom(await screen.findByRole('button', { name: 'Choose Cover' }))
	await userEvent.selectOptions(first.sort, 'title')
	await userEvent.click(within(first.picker).getByRole('button', { name: 'Close' }))
	await waitFor(() => expect(screen.queryByRole('dialog')).not.toBeInTheDocument())

	const second = await sortOfPickerFrom(screen.getByRole('button', { name: 'Choose Hero' }))

	expect(second.sort).toHaveValue('title')
})

test('moves a file up the gallery', async () => {
	const { sent } = galleryOf([12, 15])
	renderAt(EDITOR_PATH)

	await userEvent.click(await screen.findByRole('button', { name: 'Move Shore up' }))

	expect(screen.getByRole('button', { name: 'Move Shore up' })).toHaveAttribute('aria-disabled', 'true')
	expect(screen.getByRole('button', { name: 'Move Harbor down' })).toHaveAttribute('aria-disabled', 'true')
	await userEvent.click(screen.getByRole('button', { name: 'Save draft' }))

	await waitFor(() => expect(sent).toHaveLength(1))
	expect(sent[0]).toMatchObject({ fields: { gallery: [15, 12] } })
})

test('moves a file down the gallery', async () => {
	const { sent } = galleryOf([12, 15])
	renderAt(EDITOR_PATH)

	await userEvent.click(await screen.findByRole('button', { name: 'Move Harbor down' }))
	await userEvent.click(screen.getByRole('button', { name: 'Save draft' }))

	await waitFor(() => expect(sent).toHaveLength(1))
	expect(sent[0]).toMatchObject({ fields: { gallery: [15, 12] } })
})

test('removes one item from the gallery and clears an emptied one', async () => {
	const { sent } = galleryOf([12, 15])
	renderAt(EDITOR_PATH)

	await userEvent.click(await screen.findByRole('button', { name: 'Remove Harbor' }))
	await userEvent.click(screen.getByRole('button', { name: 'Remove Shore' }))
	await userEvent.click(screen.getByRole('button', { name: 'Save draft' }))

	await waitFor(() => expect(sent).toHaveLength(1))
	expect(sent[0]).toMatchObject({ fields: { gallery: null } })
})

test('says so when the files a gallery holds cannot be read', async () => {
	server.use(
		http.get('/api/types', () => HttpResponse.json({ items: [typeDeclaring([GALLERY_FIELD])] })),
		http.get(`/api/content/${storedPost.id}`, () =>
			HttpResponse.json({ ...storedPost, fields: { gallery: [12] } }),
		),
		http.get('/api/media', () => new HttpResponse(null, { status: 500 })),
	)
	renderAt(EDITOR_PATH)

	const listed = await screen.findByRole('list', { name: 'Gallery' })

	expect(await screen.findByText('These files could not be read.')).toBeInTheDocument()
	expect(within(listed).getByText('Media 12')).toBeInTheDocument()
	expect(within(listed).getByRole('button', { name: 'Remove Media 12' })).toBeInTheDocument()
	expect(within(listed).queryByRole('img')).not.toBeInTheDocument()
})

test('names a file the library no longer holds by its identity', async () => {
	galleryOf([12, 99])
	renderAt(EDITOR_PATH)

	const listed = await screen.findByRole('list', { name: 'Gallery' })

	expect(await within(listed).findByText('Harbor')).toBeInTheDocument()
	expect(within(listed).getByText('Media 99')).toBeInTheDocument()
	expect(within(listed).getByRole('button', { name: 'Remove Media 99' })).toBeInTheDocument()
})

test('names a picture without alt text by its title', async () => {
	galleryOf([20])
	renderAt(EDITOR_PATH)

	const listed = await screen.findByRole('list', { name: 'Gallery' })

	expect(await within(listed).findByRole('img', { name: 'Pier' })).toHaveAttribute(
		'src',
		'/media/2026/08/pier.jpg',
	)
})

test('shows a file that is no picture by its name alone', async () => {
	galleryOf([9])
	renderAt(EDITOR_PATH)

	const listed = await screen.findByRole('list', { name: 'Gallery' })

	expect(await within(listed).findByText('Deed')).toBeInTheDocument()
	expect(within(listed).queryByRole('img')).not.toBeInTheDocument()
})

const LINKED_FROM = {
	key: 'linked-from',
	label: 'Linked from',
	kind: 'backlinks',
	many: false,
	required: false,
	settings: { source_group: 'cars', source_field: ['maker'] },
	updated_at: '2026-08-01T10:00:00Z',
}

const POINTERS = [
	{ id: NEWS.id, title: 'News', path: 'news', type: 'category' },
	{ id: GUIDES.id, title: 'Guides', path: 'guides', type: 'page' },
]

/**
 * Serves a post pointed at by the given items under the linked from field.
 * @param pointers - The items pointing at the post.
 * @param totals - How many point at it in all.
 */
function pointedAtBy(pointers: unknown[], totals: Record<string, number>) {
	server.use(
		http.get('/api/types', () => HttpResponse.json({ items: [typeDeclaring([LINKED_FROM])] })),
		http.get(`/api/content/${storedPost.id}`, () =>
			HttpResponse.json({
				...storedPost,
				fields: { 'linked-from': pointers },
				field_totals: totals,
			}),
		),
	)
}

test('lists what points at the item under the field reading them', async () => {
	pointedAtBy(POINTERS, { 'linked-from': 2 })
	renderAt(EDITOR_PATH)

	const listed = await screen.findByRole('list', { name: 'Linked from' })

	expect(within(listed).getByRole('link', { name: 'News' })).toHaveAttribute(
		'href',
		`/admin/content/category/${NEWS.id}/edit`,
	)
	expect(within(listed).getByRole('link', { name: 'Guides' })).toHaveAttribute(
		'href',
		`/admin/content/page/${GUIDES.id}/edit`,
	)
})

test('says how many point at the item when more do than the page carries', async () => {
	pointedAtBy(POINTERS, { 'linked-from': 1234 })
	renderAt(EDITOR_PATH)

	expect(await screen.findByText('Showing 2 of 1.234.')).toBeInTheDocument()
})

test('says nothing about the count when the page carries every pointer', async () => {
	pointedAtBy(POINTERS, { 'linked-from': 2 })
	renderAt(EDITOR_PATH)

	await screen.findByRole('list', { name: 'Linked from' })

	expect(screen.queryByText(/^Showing /)).not.toBeInTheDocument()
})

test('says so when nothing points at the item yet', async () => {
	pointedAtBy([], { 'linked-from': 0 })
	renderAt(EDITOR_PATH)

	expect(await screen.findByText('Nothing points here yet.')).toBeInTheDocument()
})

const ROWS_HOLDING_A_RELATION = {
	key: 'team',
	label: 'Team',
	kind: 'repeater',
	many: false,
	required: false,
	updated_at: '2026-08-01T10:00:00Z',
	fields: [{ ...ONE_CATEGORY, key: 'filed', label: 'Filed' }],
}

test('picks a target inside a row of a repeater', async () => {
	const sent: Record<string, unknown>[] = []
	server.use(
		http.get('/api/types', () =>
			HttpResponse.json({ items: [typeDeclaring([ROWS_HOLDING_A_RELATION])] }),
		),
		http.get(`/api/content/${storedPost.id}`, () =>
			HttpResponse.json({ ...storedPost, fields: { team: [{}] } }),
		),
		http.patch(`/api/content/${storedPost.id}`, async ({ request }) => {
			sent.push((await request.json()) as Record<string, unknown>)
			return HttpResponse.json({ ...storedPost, fields: { team: [{ filed: [NEWS.id] }] } })
		}),
	)
	renderAt(EDITOR_PATH)

	await userEvent.click(await screen.findByLabelText('Filed'))
	await userEvent.click(await screen.findByRole('option', { name: 'News' }))
	await userEvent.click(screen.getByRole('button', { name: 'Save draft' }))

	await waitFor(() => expect(sent).toHaveLength(1))
	expect(sent[0]).toMatchObject({ fields: { team: [{ filed: [NEWS.id] }] } })
})

test('shows the target a row already points at', async () => {
	server.use(
		http.get('/api/types', () =>
			HttpResponse.json({ items: [typeDeclaring([ROWS_HOLDING_A_RELATION])] }),
		),
		http.get(`/api/content/${storedPost.id}`, () =>
			HttpResponse.json({ ...storedPost, fields: { team: [{ filed: [NEWS.id] }] } }),
		),
	)
	renderAt(EDITOR_PATH)

	const picker = await screen.findByLabelText('Filed')

	await waitFor(() => expect(picker).toHaveTextContent('News'))
})

test('counts nothing over the page when the server sent no totals at all', async () => {
	server.use(
		http.get('/api/types', () => HttpResponse.json({ items: [typeDeclaring([LINKED_FROM])] })),
		http.get(`/api/content/${storedPost.id}`, () =>
			HttpResponse.json({ ...storedPost, fields: { 'linked-from': POINTERS } }),
		),
	)
	renderAt(EDITOR_PATH)

	await screen.findByRole('list', { name: 'Linked from' })

	expect(screen.queryByText(/^Showing /)).not.toBeInTheDocument()
})
