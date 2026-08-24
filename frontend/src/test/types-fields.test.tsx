// SPDX-License-Identifier: Apache-2.0

import { http, HttpResponse, server } from '@gophenberg/frontend-sdk/testing'
import { screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { beforeEach, expect, test } from 'vitest'

import { renderAt } from './render'

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
	fields: [],
}

const CATEGORY_TYPE = {
	...POST_TYPE,
	key: 'category',
	singular_label: 'Category',
	plural_label: 'Categories',
	route_word: 'categories',
	default: false,
}

const COLOR = { key: 'color', label: 'Color', kind: 'text', many: false, required: false }

beforeEach(() => {
	server.use(
		http.get('/api/types', () => HttpResponse.json({ items: [POST_TYPE, CATEGORY_TYPE] })),
		http.get('/api/types/post/fields', () => HttpResponse.json({ items: [] })),
	)
})

/**
 * Opens the fields view of the post type.
 */
async function openFields() {
	renderAt('/content-types')
	const table = await screen.findByRole('region', { name: 'Content Types' })
	const posts = within(table).getByRole('row', { name: /Posts/ })
	await userEvent.click(within(posts).getByRole('button', { name: 'Fields' }))
}

test('lists the fields a type declares', async () => {
	server.use(http.get('/api/types/post/fields', () => HttpResponse.json({ items: [COLOR] })))

	await openFields()

	const held = await screen.findByRole('dialog')
	const row = within(held).getByText('Color').closest('li')!
	expect(within(row).getByText('Text')).toBeInTheDocument()
})

test('says when a type declares no fields yet', async () => {
	await openFields()

	const held = await screen.findByRole('dialog')
	expect(within(held).getByText(/no fields yet/i)).toBeInTheDocument()
})

test('declares a field from the label it was given', async () => {
	const sent: unknown[] = []
	server.use(
		http.post('/api/types/post/fields', async ({ request }) => {
			sent.push(await request.json())
			return HttpResponse.json(COLOR, { status: 201 })
		}),
	)
	await openFields()
	const held = await screen.findByRole('dialog')

	await userEvent.type(within(held).getByLabelText('Name'), 'Color')
	await userEvent.click(within(held).getByRole('button', { name: 'Add field' }))

	await waitFor(() => expect(sent).toHaveLength(1))
	expect(sent[0]).toMatchObject({ key: 'color', label: 'Color', kind: 'text' })
})

test('declares a relation field pointing at the type it names', async () => {
	const sent: unknown[] = []
	server.use(
		http.post('/api/types/post/fields', async ({ request }) => {
			sent.push(await request.json())
			return HttpResponse.json(COLOR, { status: 201 })
		}),
	)
	await openFields()
	const held = await screen.findByRole('dialog')

	await userEvent.type(within(held).getByLabelText('Name'), 'Categories')
	await userEvent.click(within(held).getByLabelText('Kind'))
	await userEvent.click(await screen.findByRole('option', { name: 'Relation' }))
	await userEvent.click(within(held).getByLabelText('Points at'))
	await userEvent.click(await screen.findByRole('option', { name: 'Categories' }))
	await userEvent.click(within(held).getByRole('button', { name: 'Add field' }))

	await waitFor(() => expect(sent).toHaveLength(1))
	expect(sent[0]).toMatchObject({ kind: 'relation', relates_to: 'category' })
})

test('relabels a field without touching its key', async () => {
	const sent: unknown[] = []
	server.use(
		http.get('/api/types/post/fields', () => HttpResponse.json({ items: [COLOR] })),
		http.patch('/api/types/post/fields/color', async ({ request }) => {
			sent.push(await request.json())
			return HttpResponse.json({ ...COLOR, label: 'Paint' })
		}),
	)
	await openFields()
	const held = await screen.findByRole('dialog')

	await userEvent.click(within(held).getByRole('button', { name: 'Rename Color' }))
	const renaming = await screen.findByRole('dialog', { name: /rename/i })
	await userEvent.clear(within(renaming).getByLabelText('Name'))
	await userEvent.type(within(renaming).getByLabelText('Name'), 'Paint')
	await userEvent.click(within(renaming).getByRole('button', { name: 'Rename' }))

	await waitFor(() => expect(sent).toHaveLength(1))
	expect(sent[0]).toEqual({ label: 'Paint' })
})

test('states what a delete sweeps before it sweeps it', async () => {
	server.use(http.get('/api/types/post/fields', () => HttpResponse.json({ items: [COLOR] })))
	await openFields()
	const held = await screen.findByRole('dialog')

	await userEvent.click(within(held).getByRole('button', { name: 'Delete Color' }))

	const confirming = await screen.findByRole('dialog', { name: /delete/i })
	expect(within(confirming).getByText(/every value/i)).toBeInTheDocument()
})

test('deletes a field once the sweep is confirmed', async () => {
	let asked = ''
	server.use(
		http.get('/api/types/post/fields', () => HttpResponse.json({ items: [COLOR] })),
		http.delete('/api/types/post/fields/color', ({ request }) => {
			asked = new URL(request.url).pathname
			return new HttpResponse(null, { status: 204 })
		}),
	)
	await openFields()
	const held = await screen.findByRole('dialog')

	await userEvent.click(within(held).getByRole('button', { name: 'Delete Color' }))
	const confirming = await screen.findByRole('dialog', { name: /delete/i })
	await userEvent.click(within(confirming).getByRole('button', { name: 'Delete the field' }))

	await waitFor(() => expect(asked).toBe('/api/types/post/fields/color'))
})

test('sweeps nothing when the delete is dismissed', async () => {
	let asked = ''
	server.use(
		http.get('/api/types/post/fields', () => HttpResponse.json({ items: [COLOR] })),
		http.delete('/api/types/post/fields/color', () => {
			asked = 'called'
			return new HttpResponse(null, { status: 204 })
		}),
	)
	await openFields()
	const held = await screen.findByRole('dialog')

	await userEvent.click(within(held).getByRole('button', { name: 'Delete Color' }))
	const confirming = await screen.findByRole('dialog', { name: /delete/i })
	await userEvent.click(within(confirming).getByRole('button', { name: 'Keep it' }))

	expect(asked).toBe('')
})

test('carries the reason the registry refused a field', async () => {
	server.use(
		http.post('/api/types/post/fields', () =>
			HttpResponse.json({ error: 'content: field key taken' }, { status: 422 }),
		),
	)
	await openFields()
	const held = await screen.findByRole('dialog')

	await userEvent.type(within(held).getByLabelText('Name'), 'Color')
	await userEvent.click(within(held).getByRole('button', { name: 'Add field' }))

	expect(await screen.findByText(/field key taken/)).toBeInTheDocument()
})

test('carries the reason a rename was refused', async () => {
	server.use(
		http.get('/api/types/post/fields', () => HttpResponse.json({ items: [COLOR] })),
		http.patch('/api/types/post/fields/color', () =>
			HttpResponse.json({ error: 'content: field not found' }, { status: 404 }),
		),
	)
	await openFields()
	const held = await screen.findByRole('dialog')

	await userEvent.click(within(held).getByRole('button', { name: 'Rename Color' }))
	const renaming = await screen.findByRole('dialog', { name: /rename/i })
	await userEvent.click(within(renaming).getByRole('button', { name: 'Rename' }))

	expect(await screen.findByText(/field not found/)).toBeInTheDocument()
})

test('carries the reason a delete was refused', async () => {
	server.use(
		http.get('/api/types/post/fields', () => HttpResponse.json({ items: [COLOR] })),
		http.delete('/api/types/post/fields/color', () =>
			HttpResponse.json({ error: 'content: field not found' }, { status: 404 }),
		),
	)
	await openFields()
	const held = await screen.findByRole('dialog')

	await userEvent.click(within(held).getByRole('button', { name: 'Delete Color' }))
	const confirming = await screen.findByRole('dialog', { name: /delete/i })
	await userEvent.click(within(confirming).getByRole('button', { name: 'Delete the field' }))

	expect(await screen.findByText(/field not found/)).toBeInTheDocument()
})

test('keeps a rename from being sent when it is dismissed', async () => {
	const sent: unknown[] = []
	server.use(
		http.get('/api/types/post/fields', () => HttpResponse.json({ items: [COLOR] })),
		http.patch('/api/types/post/fields/color', async ({ request }) => {
			sent.push(await request.json())
			return HttpResponse.json(COLOR)
		}),
	)
	await openFields()
	const held = await screen.findByRole('dialog')

	await userEvent.click(within(held).getByRole('button', { name: 'Rename Color' }))
	const renaming = await screen.findByRole('dialog', { name: /rename/i })
	await userEvent.click(within(renaming).getByRole('button', { name: 'Keep it' }))

	expect(sent).toHaveLength(0)
})

test('points a relation at the first type offered when none is chosen', async () => {
	const sent: unknown[] = []
	server.use(
		http.post('/api/types/post/fields', async ({ request }) => {
			sent.push(await request.json())
			return HttpResponse.json(COLOR, { status: 201 })
		}),
	)
	await openFields()
	const held = await screen.findByRole('dialog')

	await userEvent.type(within(held).getByLabelText('Name'), 'Related')
	await userEvent.click(within(held).getByLabelText('Kind'))
	await userEvent.click(await screen.findByRole('option', { name: 'Relation' }))
	await userEvent.click(within(held).getByRole('button', { name: 'Add field' }))

	await waitFor(() => expect(sent).toHaveLength(1))
	expect(sent[0]).toMatchObject({ kind: 'relation', relates_to: 'post', many: true })
})

test('keeps the chosen kind marked as chosen after the dialog redraws', async () => {
	await openFields()
	const held = await screen.findByRole('dialog')

	await userEvent.type(within(held).getByLabelText('Name'), 'Color')
	await userEvent.click(within(held).getByLabelText('Kind'))

	expect(await screen.findByRole('option', { name: 'Text' })).toHaveAttribute(
		'aria-selected',
		'true',
	)
})

const ENGINE = { key: 'engine', label: 'Engine', kind: 'text', many: false, required: false }

test('declares an optional field unless asked otherwise', async () => {
	const sent: unknown[] = []
	server.use(
		http.post('/api/types/post/fields', async ({ request }) => {
			sent.push(await request.json())
			return HttpResponse.json(COLOR, { status: 201 })
		}),
	)
	await openFields()
	const held = await screen.findByRole('dialog')

	await userEvent.type(within(held).getByLabelText('Name'), 'Color')
	await userEvent.click(within(held).getByRole('button', { name: 'Add field' }))

	await waitFor(() => expect(sent).toHaveLength(1))
	expect(sent[0]).toMatchObject({ required: false })
})

test('declares a required field when asked for one', async () => {
	const sent: unknown[] = []
	server.use(
		http.post('/api/types/post/fields', async ({ request }) => {
			sent.push(await request.json())
			return HttpResponse.json({ ...COLOR, required: true }, { status: 201 })
		}),
	)
	await openFields()
	const held = await screen.findByRole('dialog')

	await userEvent.type(within(held).getByLabelText('Name'), 'Color')
	await userEvent.click(within(held).getByLabelText('Required'))
	await userEvent.click(await screen.findByRole('option', { name: 'Yes' }))
	await userEvent.click(within(held).getByRole('button', { name: 'Add field' }))

	await waitFor(() => expect(sent).toHaveLength(1))
	expect(sent[0]).toMatchObject({ required: true })
})

test('declares a relation holding one target when asked', async () => {
	const sent: unknown[] = []
	server.use(
		http.post('/api/types/post/fields', async ({ request }) => {
			sent.push(await request.json())
			return HttpResponse.json(COLOR, { status: 201 })
		}),
	)
	await openFields()
	const held = await screen.findByRole('dialog')

	await userEvent.type(within(held).getByLabelText('Name'), 'Main category')
	await userEvent.click(within(held).getByLabelText('Kind'))
	await userEvent.click(await screen.findByRole('option', { name: 'Relation' }))
	await userEvent.click(within(held).getByLabelText('Holds'))
	await userEvent.click(await screen.findByRole('option', { name: 'One target' }))
	await userEvent.click(within(held).getByRole('button', { name: 'Add field' }))

	await waitFor(() => expect(sent).toHaveLength(1))
	expect(sent[0]).toMatchObject({ kind: 'relation', many: false })
})

test('requires a declared field from its row', async () => {
	const sent: unknown[] = []
	server.use(
		http.get('/api/types/post/fields', () => HttpResponse.json({ items: [COLOR] })),
		http.patch('/api/types/post/fields/color', async ({ request }) => {
			sent.push(await request.json())
			return HttpResponse.json({ ...COLOR, required: true })
		}),
	)
	await openFields()
	const held = await screen.findByRole('dialog')

	await userEvent.click(within(held).getByRole('button', { name: 'Require Color' }))

	await waitFor(() => expect(sent).toHaveLength(1))
	expect(sent[0]).toEqual({ required: true })
})

test('releases a required field from its row', async () => {
	const sent: unknown[] = []
	server.use(
		http.get('/api/types/post/fields', () =>
			HttpResponse.json({ items: [{ ...COLOR, required: true }] }),
		),
		http.patch('/api/types/post/fields/color', async ({ request }) => {
			sent.push(await request.json())
			return HttpResponse.json(COLOR)
		}),
	)
	await openFields()
	const held = await screen.findByRole('dialog')

	const row = within(held).getByText('Color').closest('li')!
	expect(within(row).getByText('Required')).toBeInTheDocument()
	await userEvent.click(within(held).getByRole('button', { name: 'Make Color optional' }))

	await waitFor(() => expect(sent).toHaveLength(1))
	expect(sent[0]).toEqual({ required: false })
})

test('carries the reason a required toggle was refused', async () => {
	server.use(
		http.get('/api/types/post/fields', () => HttpResponse.json({ items: [COLOR] })),
		http.patch('/api/types/post/fields/color', () =>
			HttpResponse.json({ error: 'content: field not found' }, { status: 404 }),
		),
	)
	await openFields()
	const held = await screen.findByRole('dialog')

	await userEvent.click(within(held).getByRole('button', { name: 'Require Color' }))

	expect(await screen.findByText(/field not found/)).toBeInTheDocument()
})

test('stores the order a field is moved up into', async () => {
	const sent: unknown[] = []
	server.use(
		http.get('/api/types/post/fields', () => HttpResponse.json({ items: [COLOR, ENGINE] })),
		http.put('/api/types/post/fields/order', async ({ request }) => {
			sent.push(await request.json())
			return HttpResponse.json({ items: [ENGINE, COLOR] })
		}),
	)
	await openFields()
	const held = await screen.findByRole('dialog')

	await userEvent.click(within(held).getByRole('button', { name: 'Move Engine up' }))

	await waitFor(() => expect(sent).toHaveLength(1))
	expect(sent[0]).toEqual({ order: ['engine', 'color'] })
})

test('stores the order a field is moved down into', async () => {
	const sent: unknown[] = []
	server.use(
		http.get('/api/types/post/fields', () => HttpResponse.json({ items: [COLOR, ENGINE] })),
		http.put('/api/types/post/fields/order', async ({ request }) => {
			sent.push(await request.json())
			return HttpResponse.json({ items: [ENGINE, COLOR] })
		}),
	)
	await openFields()
	const held = await screen.findByRole('dialog')

	await userEvent.click(within(held).getByRole('button', { name: 'Move Color down' }))

	await waitFor(() => expect(sent).toHaveLength(1))
	expect(sent[0]).toEqual({ order: ['engine', 'color'] })
})

test('holds the edges of the field order still', async () => {
	server.use(http.get('/api/types/post/fields', () => HttpResponse.json({ items: [COLOR, ENGINE] })))

	await openFields()

	const held = await screen.findByRole('dialog')
	expect(within(held).getByRole('button', { name: 'Move Color up' })).toHaveAttribute(
		'aria-disabled',
		'true',
	)
	expect(within(held).getByRole('button', { name: 'Move Engine down' })).toHaveAttribute(
		'aria-disabled',
		'true',
	)
})

test('carries the reason a reorder was refused', async () => {
	server.use(
		http.get('/api/types/post/fields', () => HttpResponse.json({ items: [COLOR, ENGINE] })),
		http.put('/api/types/post/fields/order', () =>
			HttpResponse.json({ error: 'content: incomplete field order' }, { status: 422 }),
		),
	)
	await openFields()
	const held = await screen.findByRole('dialog')

	await userEvent.click(within(held).getByRole('button', { name: 'Move Engine up' }))

	expect(await screen.findByText(/incomplete field order/)).toBeInTheDocument()
})

test('falls back to the stored kind when the admin offers no name for it', async () => {
	server.use(
		http.get('/api/types/post/fields', () =>
			HttpResponse.json({ items: [{ ...COLOR, kind: 'geography' }] }),
		),
	)

	await openFields()

	const held = await screen.findByRole('dialog')
	const row = within(held).getByText('Color').closest('li')!
	expect(within(row).getByText('geography')).toBeInTheDocument()
})
