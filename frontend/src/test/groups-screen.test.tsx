// SPDX-License-Identifier: Apache-2.0

import { http, HttpResponse, server } from '@gophenberg/frontend-sdk/testing'
import { screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { beforeEach, expect, test, vi } from 'vitest'

import { groupErrorMessage, placementOf } from '../content/GroupsScreen'
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
}

const RECIPE_TYPE = {
	...POST_TYPE,
	key: 'recipe',
	singular_label: 'Recipe',
	plural_label: 'Recipes',
	route_word: 'recipes',
	default: false,
}

const SUBTITLE = {
	key: 'subtitle',
	label: 'Subtitle',
	kind: 'text',
	many: false,
	required: false,
	created_at: '2026-08-01T10:00:00Z',
	updated_at: '2026-08-01T10:00:00Z',
}

const DETAILS_GROUP = {
	id: 3,
	title: 'Article details',
	location: [[{ source: 'content_type', operator: '==', value: 'post' }]],
	position: 1,
	active: true,
	fields: [SUBTITLE],
	created_at: '2026-08-01T10:00:00Z',
	updated_at: '2026-08-01T10:00:00Z',
}

const EXTRAS_GROUP = {
	...DETAILS_GROUP,
	id: 4,
	title: 'Extras',
	position: 2,
	fields: [],
}

const SOURCES = {
	items: [
		{
			source: 'content_type',
			operators: ['==', '!='],
			values: [
				{ value: '*', label: 'Any content type' },
				{ value: 'post', label: 'Posts' },
				{ value: 'recipe', label: 'Recipes' },
			],
		},
	],
}

/** Serves the given groups from the listing endpoint. */
function listing(groups: unknown[]) {
	server.use(http.get('/api/groups', () => HttpResponse.json({ items: groups })))
}

beforeEach(() => {
	server.use(http.get('/api/types', () => HttpResponse.json({ items: [POST_TYPE, RECIPE_TYPE] })))
	server.use(http.get('/api/groups/params', () => HttpResponse.json(SOURCES)))
	listing([DETAILS_GROUP, EXTRAS_GROUP])
})

test('lists every group with where it appears and how many fields it holds', async () => {
	renderAt('/field-groups')

	const table = await screen.findByRole('region', { name: 'Field Groups' })

	const details = within(table).getByRole('row', { name: /Article details/ })
	expect(within(details).getByText('Posts')).toBeInTheDocument()
	expect(within(details).getByText('1')).toBeInTheDocument()
	expect(within(table).getByText('Extras')).toBeInTheDocument()
})

test('says a group with no rules appears nowhere', async () => {
	listing([{ ...DETAILS_GROUP, location: [] }])
	renderAt('/field-groups')

	const table = await screen.findByRole('region', { name: 'Field Groups' })

	expect(within(table).getByText('Nowhere')).toBeInTheDocument()
})

test('marks a group that is inactive', async () => {
	listing([{ ...DETAILS_GROUP, active: false }])
	renderAt('/field-groups')

	const table = await screen.findByRole('region', { name: 'Field Groups' })
	const row = within(table).getByRole('row', { name: /Article details/ })

	expect(within(row).getByText('Inactive')).toBeInTheDocument()
})

test('reports groups it could not load', async () => {
	vi.spyOn(console, 'error').mockImplementation(() => {})
	server.use(http.get('/api/groups', () => new HttpResponse(null, { status: 500 })))
	renderAt('/field-groups')

	expect(await screen.findByText('The field groups could not be loaded.')).toBeInTheDocument()
})

test('creates a group named after what the operator typed', async () => {
	let sent: unknown
	server.use(
		http.post('/api/groups', async ({ request }) => {
			sent = await request.json()
			return HttpResponse.json(EXTRAS_GROUP, { status: 201 })
		}),
	)
	renderAt('/field-groups')
	await screen.findByRole('region', { name: 'Field Groups' })

	await userEvent.click(screen.getByRole('button', { name: 'Add New Group' }))
	await userEvent.type(await screen.findByLabelText('Title'), 'Extras')
	await userEvent.click(screen.getByRole('button', { name: 'Create' }))

	await waitFor(() => expect(sent).toMatchObject({ title: 'Extras' }))
})

test('seeds a new group with a rule so it is never born matching nowhere', async () => {
	let sent: { location?: unknown } = {}
	server.use(
		http.post('/api/groups', async ({ request }) => {
			sent = (await request.json()) as { location?: unknown }
			return HttpResponse.json(EXTRAS_GROUP, { status: 201 })
		}),
	)
	renderAt('/field-groups')
	await screen.findByRole('region', { name: 'Field Groups' })

	await userEvent.click(screen.getByRole('button', { name: 'Add New Group' }))
	await userEvent.type(await screen.findByLabelText('Title'), 'Extras')
	await userEvent.click(screen.getByRole('button', { name: 'Create' }))

	await waitFor(() => expect(sent.location).toEqual([[{ source: 'content_type', operator: '==', value: 'post' }]]))
})

test('deactivates a group so it stops appearing anywhere', async () => {
	const sent: unknown[] = []
	server.use(
		http.patch('/api/groups/3', async ({ request }) => {
			sent.push(await request.json())
			return HttpResponse.json({ ...DETAILS_GROUP, active: false })
		}),
	)
	renderAt('/field-groups')
	const table = await screen.findByRole('region', { name: 'Field Groups' })
	const row = within(table).getByRole('row', { name: /Article details/ })

	await userEvent.click(within(row).getByRole('button', { name: 'Deactivate' }))

	await waitFor(() => expect(sent).toEqual([{ active: false }]))
})

test('deletes a group once the warning is accepted', async () => {
	let hit = false
	server.use(
		http.delete('/api/groups/3', () => {
			hit = true
			return new HttpResponse(null, { status: 204 })
		}),
	)
	renderAt('/field-groups')
	const table = await screen.findByRole('region', { name: 'Field Groups' })
	const row = within(table).getByRole('row', { name: /Article details/ })

	await userEvent.click(within(row).getByRole('button', { name: 'Delete' }))
	const dialog = await screen.findByRole('dialog')
	await userEvent.click(within(dialog).getByRole('button', { name: 'Delete the group' }))

	await waitFor(() => expect(hit).toBe(true))
})

test('warns that deleting a group takes the values its fields hold', async () => {
	renderAt('/field-groups')
	const table = await screen.findByRole('region', { name: 'Field Groups' })
	const row = within(table).getByRole('row', { name: /Article details/ })

	await userEvent.click(within(row).getByRole('button', { name: 'Delete' }))

	const dialog = await screen.findByRole('dialog')
	expect(within(dialog).getByText(/values/i)).toBeInTheDocument()
})

test('keeps a group when the warning is dismissed', async () => {
	let hit = false
	server.use(
		http.delete('/api/groups/3', () => {
			hit = true
			return new HttpResponse(null, { status: 204 })
		}),
	)
	renderAt('/field-groups')
	const table = await screen.findByRole('region', { name: 'Field Groups' })
	const row = within(table).getByRole('row', { name: /Article details/ })

	await userEvent.click(within(row).getByRole('button', { name: 'Delete' }))
	const dialog = await screen.findByRole('dialog')
	await userEvent.click(within(dialog).getByRole('button', { name: 'Keep it' }))

	expect(hit).toBe(false)
})

test('moves a group up the order', async () => {
	let sent: unknown
	server.use(
		http.put('/api/groups/order', async ({ request }) => {
			sent = await request.json()
			return HttpResponse.json({ items: [EXTRAS_GROUP, DETAILS_GROUP] })
		}),
	)
	renderAt('/field-groups')
	const table = await screen.findByRole('region', { name: 'Field Groups' })
	const row = within(table).getByRole('row', { name: /Extras/ })

	await userEvent.click(within(row).getByRole('button', { name: 'Move Extras up' }))

	await waitFor(() => expect(sent).toEqual({ order: [4, 3] }))
})

test('moves a group down the order', async () => {
	let sent: unknown
	server.use(
		http.put('/api/groups/order', async ({ request }) => {
			sent = await request.json()
			return HttpResponse.json({ items: [EXTRAS_GROUP, DETAILS_GROUP] })
		}),
	)
	renderAt('/field-groups')
	const table = await screen.findByRole('region', { name: 'Field Groups' })
	const row = within(table).getByRole('row', { name: /Article details/ })

	await userEvent.click(within(row).getByRole('button', { name: 'Move Article details down' }))

	await waitFor(() => expect(sent).toEqual({ order: [4, 3] }))
})

test('leaves the first group with no way up and the last with no way down', async () => {
	renderAt('/field-groups')
	const table = await screen.findByRole('region', { name: 'Field Groups' })
	const first = within(table).getByRole('row', { name: /Article details/ })
	const last = within(table).getByRole('row', { name: /Extras/ })

	expect(within(first).getByRole('button', { name: 'Move Article details up' })).toHaveAttribute(
		'aria-disabled',
		'true',
	)
	expect(within(last).getByRole('button', { name: 'Move Extras down' })).toHaveAttribute(
		'aria-disabled',
		'true',
	)
})

test('reports why a group write was turned away', async () => {
	server.use(
		http.patch('/api/groups/3', () =>
			HttpResponse.json({ error: 'content: field key taken', code: 'field_taken' }, { status: 422 }),
		),
	)
	renderAt('/field-groups')
	const table = await screen.findByRole('region', { name: 'Field Groups' })
	const row = within(table).getByRole('row', { name: /Article details/ })

	await userEvent.click(within(row).getByRole('button', { name: 'Deactivate' }))

	expect(await screen.findByRole('alert')).toHaveTextContent(/field/i)
})

test('closes the warning and reports why a group could not be deleted', async () => {
	server.use(
		http.delete('/api/groups/3', () =>
			HttpResponse.json({ error: 'content: no such group', code: 'group_missing' }, { status: 404 }),
		),
	)
	renderAt('/field-groups')
	const table = await screen.findByRole('region', { name: 'Field Groups' })
	const row = within(table).getByRole('row', { name: /Article details/ })

	await userEvent.click(within(row).getByRole('button', { name: 'Delete' }))
	const dialog = await screen.findByRole('dialog')
	await userEvent.click(within(dialog).getByRole('button', { name: 'Delete the group' }))

	expect(await screen.findByRole('alert')).toHaveTextContent(/group/i)
	await waitFor(() => expect(screen.queryByRole('dialog')).not.toBeInTheDocument())
})

test('closes the form and reports why a group could not be created', async () => {
	server.use(
		http.post('/api/groups', () =>
			HttpResponse.json({ error: 'content: a group already carries that title', code: 'group_taken' }, {
				status: 422,
			}),
		),
	)
	renderAt('/field-groups')
	await screen.findByRole('region', { name: 'Field Groups' })

	await userEvent.click(screen.getByRole('button', { name: 'Add New Group' }))
	await userEvent.type(await screen.findByLabelText('Title'), 'Extras')
	await userEvent.click(screen.getByRole('button', { name: 'Create' }))

	expect(await screen.findByRole('alert')).toHaveTextContent(/title/i)
	await waitFor(() => expect(screen.queryByRole('dialog')).not.toBeInTheDocument())
})

test('stores nothing when the create form is abandoned', async () => {
	let hit = false
	server.use(
		http.post('/api/groups', () => {
			hit = true
			return HttpResponse.json(EXTRAS_GROUP, { status: 201 })
		}),
	)
	renderAt('/field-groups')
	await screen.findByRole('region', { name: 'Field Groups' })

	await userEvent.click(screen.getByRole('button', { name: 'Add New Group' }))
	await userEvent.type(await screen.findByLabelText('Title'), 'Extras')
	await userEvent.click(screen.getByRole('button', { name: 'Cancel' }))

	await waitFor(() => expect(screen.queryByRole('dialog')).not.toBeInTheDocument())
	expect(hit).toBe(false)
})

test('names the fallback when a group write fails without an error', () => {
	expect(groupErrorMessage(new Error('content: a group already carries that title'))).toBe(
		'content: a group already carries that title',
	)
	expect(groupErrorMessage('nonsense')).toBe('The field groups could not be reached.')
})

test('phrases where a group appears from its rules alone', () => {
	const types = [
		{ key: 'post', pluralLabel: 'Posts' },
		{ key: 'recipe', pluralLabel: 'Recipes' },
	]

	expect(placementOf([], types)).toBe('Nowhere')
	expect(placementOf([[{ source: 'content_type', operator: '==', value: '*' }]], types)).toBe(
		'Every content type',
	)
	expect(placementOf([[{ source: 'content_type', operator: '==', value: 'post' }]], types)).toBe('Posts')
	expect(
		placementOf(
			[
				[{ source: 'content_type', operator: '==', value: 'post' }],
				[{ source: 'content_type', operator: '==', value: 'recipe' }],
			],
			types,
		),
	).toBe('Posts or Recipes')
	expect(placementOf([[{ source: 'content_type', operator: '!=', value: 'post' }]], types)).toBe('Not Posts')
	expect(placementOf([[{ source: 'content_type', operator: '==', value: 'vanished' }]], types)).toBe(
		'vanished',
	)
	expect(placementOf([[{ source: 'elsewhere', operator: '==', value: 'x' }]], types)).toBe('elsewhere is x')
})
