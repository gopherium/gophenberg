// SPDX-License-Identifier: Apache-2.0

import { http, HttpResponse, server } from '@gophenberg/frontend-sdk/testing'
import { render, screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { resetLocaleData, setLocaleData } from '@wordpress/i18n'
import type { ReactElement } from 'react'
import { expect, onTestFinished, test, vi } from 'vitest'

import type { Post } from '../content/api'
import { fieldTerms, levelOf, offeredView, postFields } from '../content/fields'
import { placeholderType } from '../content/useContentType'
import { catalogFor } from '../i18n/catalog'
import { renderAt, renderRoutedAt } from './render'
import { warmPostsScreen } from './warm'

warmPostsScreen()

const STAMP = '2026-07-20T10:00:00Z'

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
	created_at: STAMP,
	updated_at: STAMP,
	fields: [],
}

/**
 * Returns a declared field of the kind carrying the settings.
 * @param key - The key the field answers to.
 * @param label - The label the column shows.
 * @param kind - The kind the field holds.
 * @param settings - The settings the field carries.
 * @returns The field as the registry serves it.
 */
function field(key: string, label: string, kind: string, settings: Record<string, unknown>) {
	return { key, label, kind, many: false, required: false, updated_at: STAMP, settings }
}

const PRICE = field('price', 'Price', 'number', { listed: true })
const ON_SALE = field('on-sale', 'On sale', 'boolean', { listed: true })
const SINCE = field('since', 'Since', 'date', { listed: true })
const COLOUR = field('colour', 'Colour', 'choice', {
	listed: true,
	choices: [
		{ value: 'red', label: 'Red' },
		{ value: 'blue', label: 'Blue' },
	],
})
const NOTE = field('note', 'Note', 'text', {})

const ITEM = {
	id: '019fb000-0000-7000-8000-000000000001',
	type: 'post',
	slug: 'welcome',
	title: 'Welcome',
	excerpt: '',
	status: 'published',
	author_id: '019fb000-0000-7000-8000-0000000000ff',
	author_name: 'Maria Perez',
	published_at: STAMP,
	created_at: STAMP,
	updated_at: STAMP,
	fields: {
		price: 10,
		'on-sale': true,
		since: '2026-09-05',
		colour: 'red',
		note: 'unlisted',
		source: { url: 'https://example.com/a', title: 'A page', new_tab: false },
		homepage: { url: 'https://example.com', title: '', new_tab: false },
	},
}

/**
 * Serves the type declaring the given fields and one item holding values.
 * @param fields - The fields the type declares.
 * @returns The addresses the listing was asked for.
 */
function declaring(fields: Record<string, unknown>[]) {
	const asked: string[] = []
	server.use(
		http.get('/api/types', () => HttpResponse.json({ items: [{ ...POST_TYPE, fields }] })),
		http.get('/api/content', ({ request }) => {
			asked.push(new URL(request.url).search)
			return HttpResponse.json({ items: [ITEM], total: 1 })
		}),
	)
	return asked
}

/**
 * Returns a listed item as the admin reads it, at the top of its tree.
 * @returns The item.
 */
function listedPost(): Post {
	return {
		id: '1',
		type: 'page',
		parentId: null,
		parentTitle: '',
		path: 'pages/one',
		slug: 'one',
		title: 'One',
		status: 'published',
		excerpt: '',
		authorId: '',
		authorName: '',
		publishedAt: STAMP,
		createdAt: STAMP,
		updatedAt: STAMP,
		date: STAMP,
	}
}

/**
 * Picks a filter through the Add filter menu, which opens its chip.
 * @param name - The label of the column to filter by.
 */
async function pickFilter(name: string) {
	await userEvent.click(screen.getByRole('button', { name: 'Add filter' }))
	await userEvent.click(await screen.findByRole('menuitem', { name }))
}

test('shows a column for every field the type marks for the list', async () => {
	declaring([PRICE, ON_SALE, SINCE, COLOUR, NOTE])
	renderAt('/content/post')

	const table = await screen.findByRole('table')

	for (const label of ['Price', 'On sale', 'Since', 'Colour']) {
		expect(within(table).getByRole('columnheader', { name: label })).toBeInTheDocument()
	}
	expect(within(table).queryByRole('columnheader', { name: 'Note' })).not.toBeInTheDocument()
})

test('reads each value the way its kind is written', async () => {
	declaring([PRICE, ON_SALE, SINCE, COLOUR])
	renderAt('/content/post')

	const row = within(await screen.findByRole('table')).getAllByRole('row')[1]

	expect(row).toHaveTextContent('10')
	expect(row).toHaveTextContent('Yes')
	expect(row).toHaveTextContent('Red')
})

test('writes a listed number in the site format to every decimal it holds', async () => {
	declaring([PRICE])
	server.use(
		http.get('/api/content', () =>
			HttpResponse.json({ items: [{ ...ITEM, fields: { price: 1234.5678 } }], total: 1 }),
		),
	)
	renderAt('/content/post')

	const row = within(await screen.findByRole('table')).getAllByRole('row')[1]

	expect(row).toHaveTextContent('1.234,5678')
})

test('shows a number field still holding text as the text it holds', async () => {
	declaring([PRICE])
	server.use(
		http.get('/api/content', () =>
			HttpResponse.json({ items: [{ ...ITEM, fields: { price: 'about 12' } }], total: 1 }),
		),
	)
	renderAt('/content/post')

	const row = within(await screen.findByRole('table')).getAllByRole('row')[1]

	expect(row).toHaveTextContent('about 12')
})

test('shows a listed text field as the text it holds', async () => {
	declaring([field('note', 'Note', 'text', { listed: true })])
	server.use(
		http.get('/api/content', () =>
			HttpResponse.json({ items: [{ ...ITEM, fields: { note: 'Hand made' } }], total: 1 }),
		),
	)
	renderAt('/content/post')

	const row = within(await screen.findByRole('table')).getAllByRole('row')[1]

	expect(row).toHaveTextContent('Hand made')
})

test('shows a date field holding something other than a day as it stands', async () => {
	declaring([SINCE])
	server.use(
		http.get('/api/content', () =>
			HttpResponse.json({ items: [{ ...ITEM, fields: { since: 20260905 } }], total: 1 }),
		),
	)
	renderAt('/content/post')

	const row = within(await screen.findByRole('table')).getAllByRole('row')[1]

	expect(row).toHaveTextContent('20260905')
})

test('shows a date field on the day it holds west of UTC', async () => {
	vi.stubEnv('TZ', 'America/New_York')
	onTestFinished(() => {
		vi.unstubAllEnvs()
	})
	declaring([SINCE])
	renderAt('/content/post')

	const row = within(await screen.findByRole('table')).getAllByRole('row')[1]

	expect(row).toHaveTextContent('05/09/2026')
})

test('shows a listed link by its title', async () => {
	declaring([field('source', 'Source', 'link', { listed: true })])
	renderAt('/content/post')

	const row = within(await screen.findByRole('table')).getAllByRole('row')[1]

	expect(row).toHaveTextContent('A page')
	expect(row).not.toHaveTextContent('object')
})

test('shows a listed link by its address when it carries no title', async () => {
	declaring([field('homepage', 'Homepage', 'link', { listed: true })])
	renderAt('/content/post')

	const row = within(await screen.findByRole('table')).getAllByRole('row')[1]

	expect(row).toHaveTextContent('https://example.com')
})

test('names a column by its key so a field called title never takes the title column', async () => {
	declaring([field('title', 'Headline', 'text', { listed: true })])
	renderAt('/content/post')

	const table = await screen.findByRole('table')

	expect(within(table).getByRole('columnheader', { name: 'Title' })).toBeInTheDocument()
	expect(within(table).getByRole('columnheader', { name: 'Headline' })).toBeInTheDocument()
})

test('asks the server for the field a chip narrows by', async () => {
	const asked = declaring([ON_SALE])
	renderAt('/content/post')
	await screen.findByRole('table')

	await pickFilter('On sale')
	await userEvent.click(await screen.findByRole('option', { name: 'Yes' }))

	await waitFor(() => expect(asked.some((search) => search.includes('field%5Bon-sale%5D=true'))).toBe(true))
})

test('keeps the chip of a listed field behind Add filter until it is picked', async () => {
	declaring([ON_SALE])
	renderAt('/content/post')

	await screen.findByRole('table')

	expect(screen.getAllByRole('button', { name: /On sale/ })).toHaveLength(1)
	expect(screen.getByRole('button', { name: 'Add filter' })).toBeInTheDocument()
})

test.each([
	{ operator: 'isNot', value: 'true' },
	{ operator: 'is', value: 'maybe' },
])('asks for no field the address narrows with $operator $value, which its chip never offers', async (filter) => {
	const asked = declaring([ON_SALE])
	const filters = encodeURIComponent(JSON.stringify([{ field: 'field.on-sale', ...filter }]))
	renderAt(`/content/post?filters=${filters}&search=sale`)

	await screen.findByRole('table')

	expect(asked[0]).toContain('search=sale')
	expect(asked[0]).not.toContain('field%5Bon-sale%5D')
})

test.each(['author', 'slug'])('asks the server to sort by %s', async (column) => {
	const asked = declaring([])
	renderAt(`/content/post?sort=${column}&order=asc`)

	await screen.findByRole('table')

	expect(asked[0]).toContain(`orderby=${column}&order=asc`)
})

test.each(['excerpt', 'status'])('asks no sort by the %s, which the list never sorts by', async (column) => {
	const asked = declaring([])
	renderAt(`/content/post?sort=${column}`)

	await screen.findByRole('table')

	expect(asked[0]).toContain('orderby=date')
})

test('offers to sort by the title, the author, the date and the slug only', async () => {
	declaring([PRICE])
	renderAt('/content/post')
	await screen.findByRole('table')

	await userEvent.click(screen.getByRole('button', { name: 'View options' }))

	const sorts = within(await screen.findByRole('combobox', { name: 'Sort by' })).getAllByRole('option')
	expect(sorts.map((option) => option.textContent)).toEqual(['Title', 'Author', 'Date', 'Slug'])
})

test('asks the server to sort a hierarchical type by its parents', async () => {
	const pageType = { ...POST_TYPE, key: 'page', plural_label: 'Pages', hierarchical: true, default: false }
	const asked: string[] = []
	server.use(
		http.get('/api/types', () => HttpResponse.json({ items: [POST_TYPE, pageType] })),
		http.get('/api/content', ({ request }) => {
			asked.push(new URL(request.url).search)
			return HttpResponse.json({ items: [{ ...ITEM, type: 'page' }], total: 1 })
		}),
	)
	renderAt('/content/page?sort=parent')

	await screen.findByRole('table')

	expect(asked[0]).toContain('orderby=parent')
})

test('offers no parent column on a type that does not nest', () => {
	expect(postFields(placeholderType('post')).map((column) => column.id)).not.toContain('parent')
})

test('names the parent of an item by its title, or None at the top', () => {
	const pages = { ...placeholderType('page'), hierarchical: true }
	const parent = postFields(pages).find((column) => column.id === 'parent')
	const nested = { ...listedPost(), parentTitle: 'About Us' }

	expect(parent?.getValue?.({ item: nested })).toBe('About Us')
	expect(parent?.getValue?.({ item: listedPost() })).toBe('None')
})

test('names the parent column and an item at the top as WordPress es_ES does for a Spanish reader', async () => {
	setLocaleData(await catalogFor('es-ES'), 'gophenberg')
	onTestFinished(() => resetLocaleData({}, 'gophenberg'))
	const pages = { ...placeholderType('page'), hierarchical: true }

	const parent = postFields(pages).find((column) => column.id === 'parent')

	expect(parent?.label).toBe('Superior')
	expect(parent?.getValue?.({ item: listedPost() })).toBe('Ninguna')
})

test('names a scheduled item by the badge WordPress es_ES draws for a Spanish reader', async () => {
	setLocaleData(await catalogFor('es-ES'), 'gophenberg')
	onTestFinished(() => resetLocaleData({}, 'gophenberg'))
	const status = postFields(placeholderType('post')).find((column) => column.id === 'status')
	const Cell = status?.render as (props: { item: Post }) => ReactElement

	render(<Cell item={{ ...listedPost(), status: 'scheduled' }} />)

	expect(screen.getByText('Programada')).toBeInTheDocument()
})

test('heads the columns of a nesting type as the WordPress page list does', () => {
	const pages = { ...placeholderType('page'), hierarchical: true }

	const labels = postFields(pages).map((column) => column.label)

	expect(labels).toEqual(['Title', 'Excerpt', 'Author', 'Status', 'Date', 'Slug', 'Parent'])
})

test('offers no way to hide the title column', async () => {
	declaring([])
	renderAt('/content/post')
	await screen.findByText('Welcome')

	await userEvent.click(screen.getByRole('button', { name: 'Title' }))

	expect(await screen.findAllByRole('menuitemradio')).toHaveLength(2)
	expect(screen.queryByRole('menuitem', { name: 'Hide column' })).not.toBeInTheDocument()
})

test('counts the levels of a nesting type with no route word from its first slash', () => {
	const nesting = { ...placeholderType('category'), hierarchical: true, routeWord: '' }

	expect(levelOf(nesting)({ ...listedPost(), path: 'news' })).toBe(0)
	expect(levelOf(nesting)({ ...listedPost(), path: 'news/local' })).toBe(1)
})

test('reads a view naming no sort and no filter as the opening order, narrowed by nothing', () => {
	const view = offeredView({ type: 'table' }, postFields(placeholderType('post')))

	expect(view.sort).toEqual({ field: 'date', direction: 'desc' })
	expect(view.filters).toEqual([])
})

test('joins the labels a field holding several choices carries', async () => {
	const tags = field('tags', 'Tags', 'choice', {
		listed: true,
		multiple: true,
		choices: [{ value: 'red', label: 'Red' }],
	})
	declaring([tags])
	server.use(
		http.get('/api/content', () =>
			HttpResponse.json({ items: [{ ...ITEM, fields: { tags: ['red', 'stray'] } }], total: 1 }),
		),
	)
	renderAt('/content/post')

	const row = within(await screen.findByRole('table')).getAllByRole('row')[1]

	expect(row).toHaveTextContent('Red, stray')
})

test('offers no chip on a choice field nobody gave options', async () => {
	declaring([field('colour', 'Colour', 'choice', { listed: true })])
	renderAt('/content/post')
	await screen.findByRole('table')

	await userEvent.click(screen.getByRole('button', { name: 'Add filter' }))

	expect(await screen.findByRole('menuitem', { name: 'Author' })).toBeInTheDocument()
	expect(screen.queryByRole('menuitem', { name: 'Colour' })).not.toBeInTheDocument()
})

test('reads the terms a view narrows by, ignoring what names no field', () => {
	const terms = fieldTerms([
		{ field: 'field.price', value: 10 },
		{ field: 'status', value: 'draft' },
		{ field: 'field.note', value: undefined },
	])

	expect(terms).toEqual({ price: '10' })
})

test('reads no term from a view carrying no filter', () => {
	expect(fieldTerms(undefined)).toEqual({})
})

test('reads a boolean nobody switched on as no', async () => {
	declaring([ON_SALE])
	server.use(
		http.get('/api/content', () =>
			HttpResponse.json({ items: [{ ...ITEM, fields: { 'on-sale': false } }], total: 1 }),
		),
	)
	renderAt('/content/post')

	const row = within(await screen.findByRole('table')).getAllByRole('row')[1]

	expect(row).toHaveTextContent('No')
})

test('drops a filter the type it moves to never declared', async () => {
	const pageType = { ...POST_TYPE, key: 'page', plural_label: 'Pages', default: false, fields: [] }
	const asked: string[] = []
	server.use(
		http.get('/api/types', () =>
			HttpResponse.json({ items: [{ ...POST_TYPE, fields: [ON_SALE] }, pageType] }),
		),
		http.get('/api/content', ({ request }) => {
			asked.push(new URL(request.url).search)
			return HttpResponse.json({ items: [ITEM], total: 1 })
		}),
	)
	const { router } = renderRoutedAt('/content/post')
	await screen.findByRole('table')
	await pickFilter('On sale')
	await userEvent.click(await screen.findByRole('option', { name: 'Yes' }))
	await waitFor(() => expect(asked.some((search) => search.includes('field%5Bon-sale%5D'))).toBe(true))

	await router.navigate({ to: '/content/$typeKey', params: { typeKey: 'page' } })

	await waitFor(() => expect(asked.at(-1)).toContain('type=page'))
	expect(asked.at(-1)).not.toContain('field%5Bon-sale%5D')
})

test('drops a filter the type it moves to declares under another kind', async () => {
	const colour = field('shared', 'Shared', 'choice', {
		listed: true,
		choices: [{ value: 'red', label: 'Red' }],
	})
	const flag = field('shared', 'Shared', 'boolean', { listed: true })
	const pageType = { ...POST_TYPE, key: 'page', plural_label: 'Pages', default: false, fields: [flag] }
	const asked: string[] = []
	server.use(
		http.get('/api/types', () =>
			HttpResponse.json({ items: [{ ...POST_TYPE, fields: [colour] }, pageType] }),
		),
		http.get('/api/content', ({ request }) => {
			asked.push(new URL(request.url).search)
			return HttpResponse.json({ items: [ITEM], total: 1 })
		}),
	)
	const { router } = renderRoutedAt('/content/post')
	await screen.findByRole('table')
	await pickFilter('Shared')
	await userEvent.click(await screen.findByRole('option', { name: 'Red' }))
	await waitFor(() => expect(asked.some((search) => search.includes('field%5Bshared%5D=red'))).toBe(true))

	await router.navigate({ to: '/content/$typeKey', params: { typeKey: 'page' } })

	await waitFor(() => expect(asked.at(-1)).toContain('type=page'))
	expect(asked.at(-1)).not.toContain('field%5Bshared%5D')
})
