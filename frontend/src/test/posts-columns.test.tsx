// SPDX-License-Identifier: Apache-2.0

import { http, HttpResponse, server } from '@gophenberg/frontend-sdk/testing'
import { screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { beforeEach, expect, test } from 'vitest'

import { fieldTerms } from '../content/fields'
import { renderAt, renderRoutedAt } from './render'
import { warmPostsScreen } from './warm'

warmPostsScreen()

const STAMP = '2026-07-20T10:00:00Z'

const POST_TYPE = {
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

beforeEach(() => {
	server.use(
		http.get('/api/content/counts', () =>
			HttpResponse.json({ draft: 0, published: 1, pending: 0, private: 0, trash: 0 }),
		),
	)
})

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

	await userEvent.click(screen.getAllByRole('button', { name: /On sale/ })[0])
	await userEvent.click(await screen.findByRole('option', { name: 'Yes' }))

	await waitFor(() => expect(asked.some((search) => search.includes('field%5Bon-sale%5D=true'))).toBe(true))
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

	expect(screen.getAllByRole('button', { name: /Colour/ })).toHaveLength(1)
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
	await userEvent.click(screen.getAllByRole('button', { name: /On sale/ })[0])
	await userEvent.click(await screen.findByRole('option', { name: 'Yes' }))
	await waitFor(() => expect(asked.some((search) => search.includes('field%5Bon-sale%5D'))).toBe(true))

	await router.navigate({ to: '/content/$typeKey', params: { typeKey: 'page' } })

	await waitFor(() => expect(asked.at(-1)).toContain('type=page'))
	expect(asked.at(-1)).not.toContain('field%5Bon-sale%5D')
})
