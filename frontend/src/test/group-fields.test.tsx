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

const READING_TIME = {
	...SUBTITLE,
	key: 'reading-time',
	label: 'Reading time',
	kind: 'number',
	required: true,
}

const DETAILS = {
	id: 3,
	title: 'Article details',
	location: [[{ source: 'content_type', operator: '==', value: 'post' }]],
	position: 1,
	active: true,
	fields: [SUBTITLE, READING_TIME],
	created_at: '2026-08-01T10:00:00Z',
	updated_at: '2026-08-01T10:00:00Z',
}

const EXTRAS = {
	...DETAILS,
	id: 4,
	title: 'Extras',
	position: 2,
	fields: [],
}

/** Serves the given groups from the listing endpoint. */
function listing(groups: unknown[]) {
	server.use(http.get('/api/groups', () => HttpResponse.json({ items: groups })))
}

/** Opens the fields of the named group and answers with its dialog. */
async function openFields(named = 'Article details') {
	const table = await screen.findByRole('region', { name: 'Field Groups' })
	const row = within(table).getByRole('row', { name: new RegExp(named) })
	await userEvent.click(within(row).getByRole('button', { name: 'Fields' }))
	return screen.findByRole('dialog')
}

beforeEach(() => {
	server.use(http.get('/api/types', () => HttpResponse.json({ items: [POST_TYPE] })))
	listing([DETAILS, EXTRAS])
})

test('lists the fields the group holds under their kind', async () => {
	renderAt('/field-groups')

	const dialog = await openFields()

	const text = within(dialog).getByRole('listitem', { name: 'Subtitle' })
	const number = within(dialog).getByRole('listitem', { name: 'Reading time' })
	expect(within(text).getByText('Text')).toBeInTheDocument()
	expect(within(number).getByText('Number')).toBeInTheDocument()
})

test('marks the fields an item cannot be published without', async () => {
	renderAt('/field-groups')

	const dialog = await openFields()
	const required = within(dialog).getByRole('listitem', { name: 'Reading time' })

	expect(within(required).getByText('Required')).toBeInTheDocument()
})

test('says so when a group holds no fields yet', async () => {
	renderAt('/field-groups')

	const dialog = await openFields('Extras')

	expect(within(dialog).getByText('This group holds no fields yet.')).toBeInTheDocument()
})

test('declares a field into the group it was opened from', async () => {
	let sent: unknown
	server.use(
		http.post('/api/groups/3/fields', async ({ request }) => {
			sent = await request.json()
			return HttpResponse.json(SUBTITLE, { status: 201 })
		}),
	)
	renderAt('/field-groups')
	const dialog = await openFields()

	await userEvent.type(within(dialog).getByLabelText('Name'), 'Summary')
	await userEvent.click(within(dialog).getByRole('button', { name: 'Add field' }))

	await waitFor(() =>
		expect(sent).toMatchObject({ key: 'summary', label: 'Summary', kind: 'text' }),
	)
})

test('renames a field without moving what is stored under it', async () => {
	let sent: unknown
	server.use(
		http.patch('/api/groups/3/fields/subtitle', async ({ request }) => {
			sent = await request.json()
			return HttpResponse.json({ ...SUBTITLE, label: 'Standfirst' })
		}),
	)
	renderAt('/field-groups')
	const dialog = await openFields()

	await userEvent.click(within(dialog).getByRole('button', { name: 'Rename Subtitle' }))
	const renaming = await screen.findByRole('dialog', { name: 'Rename Subtitle' })
	const box = within(renaming).getByLabelText('Name')
	await userEvent.clear(box)
	await userEvent.type(box, 'Standfirst')
	await userEvent.click(within(renaming).getByRole('button', { name: 'Rename' }))

	await waitFor(() => expect(sent).toEqual({ label: 'Standfirst' }))
})

test('requires a field that was optional', async () => {
	let sent: unknown
	server.use(
		http.patch('/api/groups/3/fields/subtitle', async ({ request }) => {
			sent = await request.json()
			return HttpResponse.json({ ...SUBTITLE, required: true })
		}),
	)
	renderAt('/field-groups')
	const dialog = await openFields()

	await userEvent.click(within(dialog).getByRole('button', { name: 'Require Subtitle' }))

	await waitFor(() => expect(sent).toEqual({ required: true }))
})

test('deletes a field once the warning about its values is accepted', async () => {
	let hit = false
	server.use(
		http.delete('/api/groups/3/fields/subtitle', () => {
			hit = true
			return new HttpResponse(null, { status: 204 })
		}),
	)
	renderAt('/field-groups')
	const dialog = await openFields()

	await userEvent.click(within(dialog).getByRole('button', { name: 'Delete Subtitle' }))
	const warning = await screen.findByRole('dialog', { name: 'Delete Subtitle' })
	expect(within(warning).getByText(/value/i)).toBeInTheDocument()
	await userEvent.click(within(warning).getByRole('button', { name: 'Delete the field' }))

	await waitFor(() => expect(hit).toBe(true))
})

test('stores the order a field is moved down into', async () => {
	let sent: unknown
	server.use(
		http.put('/api/groups/3/fields/order', async ({ request }) => {
			sent = await request.json()
			return HttpResponse.json({ items: [READING_TIME, SUBTITLE] })
		}),
	)
	renderAt('/field-groups')
	const dialog = await openFields()

	await userEvent.click(within(dialog).getByRole('button', { name: 'Move Subtitle down' }))

	await waitFor(() => expect(sent).toEqual({ order: ['reading-time', 'subtitle'] }))
})

test('leaves the first field with no way up and the last with no way down', async () => {
	renderAt('/field-groups')

	const dialog = await openFields()

	expect(within(dialog).getByRole('button', { name: 'Move Subtitle up' })).toHaveAttribute(
		'aria-disabled',
		'true',
	)
	expect(within(dialog).getByRole('button', { name: 'Move Reading time down' })).toHaveAttribute(
		'aria-disabled',
		'true',
	)
})

test('carries a field over to another group', async () => {
	let sent: unknown
	server.use(
		http.post('/api/groups/3/fields/subtitle/move', async ({ request }) => {
			sent = await request.json()
			return HttpResponse.json(SUBTITLE)
		}),
	)
	renderAt('/field-groups')
	const dialog = await openFields()

	await userEvent.click(within(dialog).getByRole('button', { name: 'Move Subtitle elsewhere' }))
	const moving = await screen.findByRole('dialog', { name: 'Move Subtitle elsewhere' })
	await userEvent.click(within(moving).getByLabelText('Group'))
	await userEvent.click(await screen.findByRole('option', { name: 'Extras' }))
	await userEvent.click(within(moving).getByRole('button', { name: 'Move the field' }))

	await waitFor(() => expect(sent).toEqual({ to_group: 4 }))
})

test('offers nowhere to carry a field when the group is the only one', async () => {
	listing([DETAILS])
	renderAt('/field-groups')

	const dialog = await openFields()

	expect(within(dialog).queryByRole('button', { name: 'Move Subtitle elsewhere' })).not.toBeInTheDocument()
})

test('reports a refused field where the operator is looking', async () => {
	server.use(
		http.post('/api/groups/3/fields', () =>
			HttpResponse.json({ error: 'content: field key taken', code: 'field_taken' }, { status: 422 }),
		),
	)
	renderAt('/field-groups')
	const dialog = await openFields()

	await userEvent.type(within(dialog).getByLabelText('Name'), 'Subtitle')
	await userEvent.click(within(dialog).getByRole('button', { name: 'Add field' }))

	const notice = await within(dialog).findByRole('alert')
	expect(notice).toHaveTextContent(/field with that name/i)
})

test('declares a relation field pointing at the type and holding what was picked', async () => {
	let sent: unknown
	server.use(
		http.post('/api/groups/3/fields', async ({ request }) => {
			sent = await request.json()
			return HttpResponse.json(SUBTITLE, { status: 201 })
		}),
	)
	renderAt('/field-groups')
	const dialog = await openFields()

	await userEvent.type(within(dialog).getByLabelText('Name'), 'Author')
	await userEvent.click(within(dialog).getByLabelText('Kind'))
	await userEvent.click(await screen.findByRole('option', { name: 'Relation' }))
	await userEvent.click(within(dialog).getByLabelText('Points at'))
	await userEvent.click(await screen.findByRole('option', { name: 'Posts' }))
	await userEvent.click(within(dialog).getByLabelText('Holds'))
	await userEvent.click(await screen.findByRole('option', { name: 'One target' }))
	await userEvent.click(within(dialog).getByLabelText('Required'))
	await userEvent.click(await screen.findByRole('option', { name: 'Yes' }))
	await userEvent.click(within(dialog).getByRole('button', { name: 'Add field' }))

	await waitFor(() =>
		expect(sent).toMatchObject({
			key: 'author',
			kind: 'relation',
			relates_to: 'post',
			many: false,
			required: true,
		}),
	)
})

test('makes a required field optional again', async () => {
	let sent: unknown
	server.use(
		http.patch('/api/groups/3/fields/reading-time', async ({ request }) => {
			sent = await request.json()
			return HttpResponse.json({ ...READING_TIME, required: false })
		}),
	)
	renderAt('/field-groups')
	const dialog = await openFields()

	await userEvent.click(within(dialog).getByRole('button', { name: 'Make Reading time optional' }))

	await waitFor(() => expect(sent).toEqual({ required: false }))
})

test('closes the rename and reports why it was turned away', async () => {
	server.use(
		http.patch('/api/groups/3/fields/subtitle', () =>
			HttpResponse.json({ error: 'content: field not found', code: 'field_unknown' }, { status: 404 }),
		),
	)
	renderAt('/field-groups')
	const dialog = await openFields()

	await userEvent.click(within(dialog).getByRole('button', { name: 'Rename Subtitle' }))
	const renaming = await screen.findByRole('dialog', { name: 'Rename Subtitle' })
	await userEvent.click(within(renaming).getByRole('button', { name: 'Rename' }))

	expect(await within(dialog).findByRole('alert')).toHaveTextContent(/field/i)
})

test('closes the warning and reports why a delete was turned away', async () => {
	server.use(
		http.delete('/api/groups/3/fields/subtitle', () =>
			HttpResponse.json({ error: 'content: field not found', code: 'field_unknown' }, { status: 404 }),
		),
	)
	renderAt('/field-groups')
	const dialog = await openFields()

	await userEvent.click(within(dialog).getByRole('button', { name: 'Delete Subtitle' }))
	const warning = await screen.findByRole('dialog', { name: 'Delete Subtitle' })
	await userEvent.click(within(warning).getByRole('button', { name: 'Delete the field' }))

	expect(await within(dialog).findByRole('alert')).toHaveTextContent(/field/i)
})

test('closes the move and reports why it was turned away', async () => {
	server.use(
		http.post('/api/groups/3/fields/subtitle/move', () =>
			HttpResponse.json({ error: 'content: field key taken', code: 'field_taken' }, { status: 422 }),
		),
	)
	renderAt('/field-groups')
	const dialog = await openFields()

	await userEvent.click(within(dialog).getByRole('button', { name: 'Move Subtitle elsewhere' }))
	const moving = await screen.findByRole('dialog', { name: 'Move Subtitle elsewhere' })
	await userEvent.click(within(moving).getByRole('button', { name: 'Move the field' }))

	expect(await within(dialog).findByRole('alert')).toHaveTextContent(/field/i)
})

test('keeps a field where it is when the move is dismissed', async () => {
	let hit = false
	server.use(
		http.post('/api/groups/3/fields/subtitle/move', () => {
			hit = true
			return HttpResponse.json(SUBTITLE)
		}),
	)
	renderAt('/field-groups')
	const dialog = await openFields()

	await userEvent.click(within(dialog).getByRole('button', { name: 'Move Subtitle elsewhere' }))
	const moving = await screen.findByRole('dialog', { name: 'Move Subtitle elsewhere' })
	await userEvent.click(within(moving).getByRole('button', { name: 'Keep it here' }))

	expect(hit).toBe(false)
})

test('keeps a field when the delete warning is dismissed', async () => {
	let hit = false
	server.use(
		http.delete('/api/groups/3/fields/subtitle', () => {
			hit = true
			return new HttpResponse(null, { status: 204 })
		}),
	)
	renderAt('/field-groups')
	const dialog = await openFields()

	await userEvent.click(within(dialog).getByRole('button', { name: 'Delete Subtitle' }))
	const warning = await screen.findByRole('dialog', { name: 'Delete Subtitle' })
	await userEvent.click(within(warning).getByRole('button', { name: 'Keep it' }))

	expect(hit).toBe(false)
})

test('keeps the name when a rename is dismissed', async () => {
	let hit = false
	server.use(
		http.patch('/api/groups/3/fields/subtitle', () => {
			hit = true
			return HttpResponse.json(SUBTITLE)
		}),
	)
	renderAt('/field-groups')
	const dialog = await openFields()

	await userEvent.click(within(dialog).getByRole('button', { name: 'Rename Subtitle' }))
	const renaming = await screen.findByRole('dialog', { name: 'Rename Subtitle' })
	await userEvent.click(within(renaming).getByRole('button', { name: 'Keep it' }))

	expect(hit).toBe(false)
})

test('stores the order a field is moved up into', async () => {
	let sent: unknown
	server.use(
		http.put('/api/groups/3/fields/order', async ({ request }) => {
			sent = await request.json()
			return HttpResponse.json({ items: [READING_TIME, SUBTITLE] })
		}),
	)
	renderAt('/field-groups')
	const dialog = await openFields()

	await userEvent.click(within(dialog).getByRole('button', { name: 'Move Reading time up' }))

	await waitFor(() => expect(sent).toEqual({ order: ['reading-time', 'subtitle'] }))
})

test('leaves the fields alone when the dialog is closed again', async () => {
	renderAt('/field-groups')
	const dialog = await openFields()

	await userEvent.click(within(dialog).getByRole('button', { name: 'Close' }))

	await waitFor(() => expect(screen.queryByRole('dialog')).not.toBeInTheDocument())
})

test('reports a refused reorder inside the dialog', async () => {
	server.use(
		http.put('/api/groups/3/fields/order', () =>
			HttpResponse.json({ error: 'content: the order has to name every field', code: 'field_order' }, {
				status: 422,
			}),
		),
	)
	renderAt('/field-groups')
	const dialog = await openFields()

	await userEvent.click(within(dialog).getByRole('button', { name: 'Move Subtitle down' }))

	expect(await within(dialog).findByRole('alert')).toHaveTextContent(/every field/)
})
