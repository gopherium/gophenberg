// SPDX-License-Identifier: Apache-2.0

import { http, HttpResponse, server } from '@gophenberg/frontend-sdk/testing'
import { act, screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { beforeEach, expect, test, vi } from 'vitest'

import { groupsQueryKey } from '../content/groups'
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

/**
 * Serves the given groups from the listing endpoint.
 * @param groups - The groups the listing answers with.
 */
function listing(groups: unknown[]) {
	server.use(http.get('/api/groups', () => HttpResponse.json({ items: groups })))
}

/**
 * Opens the fields of the named group.
 * @param named - The group whose fields to open.
 * @returns The dialog the group opened.
 */
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

	await waitFor(() =>
		expect(sent).toEqual({ label: 'Standfirst', updated_at: '2026-08-01T10:00:00Z' }),
	)
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

	await waitFor(() =>
		expect(sent).toEqual({ required: true, updated_at: '2026-08-01T10:00:00Z' }),
	)
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

test('declares a plain field while the content types are still out of reach', async () => {
	let sent: unknown
	vi.spyOn(console, 'error').mockImplementation(() => {})
	server.use(http.get('/api/types', () => new HttpResponse(null, { status: 500 })))
	server.use(
		http.post('/api/groups/3/fields', async ({ request }) => {
			sent = await request.json()
			return HttpResponse.json(SUBTITLE, { status: 201 })
		}),
	)
	renderAt('/field-groups')
	const dialog = await openFields()

	await userEvent.type(within(dialog).getByLabelText('Name'), 'Summary')
	await userEvent.click(within(dialog).getByLabelText('Kind'))
	await userEvent.click(await screen.findByRole('option', { name: 'Relation' }))
	await userEvent.click(within(dialog).getByRole('button', { name: 'Add field' }))

	expect(within(dialog).queryByLabelText('Points at')).not.toBeInTheDocument()
	expect(within(dialog).getByRole('button', { name: 'Add field' })).toHaveAttribute(
		'aria-disabled',
		'true',
	)
	expect(sent).toBeUndefined()
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

	await waitFor(() =>
		expect(sent).toEqual({ required: false, updated_at: '2026-08-01T10:00:00Z' }),
	)
})

test('renames on the timestamp the dialog opened with, not one that landed while it was open', async () => {
	let sent: unknown
	server.use(
		http.patch('/api/groups/3/fields/subtitle', async ({ request }) => {
			sent = await request.json()
			return HttpResponse.json({ ...SUBTITLE, label: 'Standfirst' })
		}),
	)
	const client = renderAt('/field-groups')
	const dialog = await openFields()
	await userEvent.click(within(dialog).getByRole('button', { name: 'Rename Subtitle' }))
	const renaming = await screen.findByRole('dialog', { name: 'Rename Subtitle' })
	const box = within(renaming).getByLabelText('Name')
	await userEvent.clear(box)
	await userEvent.type(box, 'Standfirst')

	listing([
		{ ...DETAILS, fields: [{ ...SUBTITLE, updated_at: '2026-08-02T09:00:00Z' }, READING_TIME] },
		EXTRAS,
	])
	await act(async () => {
		await client.invalidateQueries({ queryKey: groupsQueryKey })
	})
	await userEvent.click(within(renaming).getByRole('button', { name: 'Rename' }))

	await waitFor(() =>
		expect(sent).toEqual({ label: 'Standfirst', updated_at: '2026-08-01T10:00:00Z' }),
	)
})

test('settles on the timestamp the dialog opened with, not one that landed while it was open', async () => {
	let sent: unknown
	server.use(
		http.patch('/api/groups/3/fields/subtitle', async ({ request }) => {
			sent = await request.json()
			return HttpResponse.json(SUBTITLE)
		}),
	)
	const client = renderAt('/field-groups')
	const dialog = await openFields()
	await userEvent.click(within(dialog).getByRole('button', { name: 'Settings of Subtitle' }))
	const settings = await screen.findByRole('dialog', { name: 'Settings of Subtitle' })
	await userEvent.type(within(settings).getByLabelText('Instructions'), 'Say who wrote it.')

	listing([
		{ ...DETAILS, fields: [{ ...SUBTITLE, updated_at: '2026-08-02T09:00:00Z' }, READING_TIME] },
		EXTRAS,
	])
	await act(async () => {
		await client.invalidateQueries({ queryKey: groupsQueryKey })
	})
	await userEvent.click(within(settings).getByRole('button', { name: 'Save settings' }))

	await waitFor(() =>
		expect(sent).toEqual({
			settings: { instructions: 'Say who wrote it.' },
			updated_at: '2026-08-01T10:00:00Z',
		}),
	)
})

test('reports a rename that lost to a concurrent edit', async () => {
	server.use(
		http.patch('/api/groups/3/fields/subtitle', () =>
			HttpResponse.json(
				{ error: 'content: conflicting update', code: 'content_stale_update' },
				{ status: 409 },
			),
		),
	)
	renderAt('/field-groups')
	const dialog = await openFields()

	await userEvent.click(within(dialog).getByRole('button', { name: 'Rename Subtitle' }))
	const renaming = await screen.findByRole('dialog', { name: 'Rename Subtitle' })
	await userEvent.click(within(renaming).getByRole('button', { name: 'Rename' }))

	expect(await within(dialog).findByRole('alert')).toHaveTextContent(/someone else saved/i)
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

test('offers the settings a text field takes and stores them', async () => {
	let sent: unknown
	server.use(
		http.patch('/api/groups/3/fields/subtitle', async ({ request }) => {
			sent = await request.json()
			return HttpResponse.json({ ...SUBTITLE, settings: { maxlength: 80 } })
		}),
	)
	renderAt('/field-groups')
	const dialog = await openFields()

	await userEvent.click(within(dialog).getByRole('button', { name: 'Settings of Subtitle' }))
	const settings = await screen.findByRole('dialog', { name: 'Settings of Subtitle' })
	await userEvent.type(within(settings).getByLabelText('Instructions'), 'Say who wrote it.')
	await userEvent.type(within(settings).getByLabelText('Longest'), '80')
	await userEvent.click(within(settings).getByRole('button', { name: 'Save settings' }))

	await waitFor(() =>
		expect(sent).toEqual({
			settings: { instructions: 'Say who wrote it.', maxlength: 80 },
			updated_at: '2026-08-01T10:00:00Z',
		}),
	)
})

test('offers a number field its own bounds', async () => {
	listing([{ ...DETAILS, fields: [READING_TIME] }, EXTRAS])
	let sent: unknown
	server.use(
		http.patch('/api/groups/3/fields/reading-time', async ({ request }) => {
			sent = await request.json()
			return HttpResponse.json(READING_TIME)
		}),
	)
	renderAt('/field-groups')
	const dialog = await openFields()

	await userEvent.click(within(dialog).getByRole('button', { name: 'Settings of Reading time' }))
	const settings = await screen.findByRole('dialog', { name: 'Settings of Reading time' })
	await userEvent.type(within(settings).getByLabelText('Lowest'), '1')
	await userEvent.type(within(settings).getByLabelText('Highest'), '10')
	await userEvent.click(within(settings).getByRole('button', { name: 'Save settings' }))

	await waitFor(() =>
		expect(sent).toEqual({ settings: { min: 1, max: 10 }, updated_at: '2026-08-01T10:00:00Z' }),
	)
})

test('offers a media field nothing but instructions', async () => {
	listing([{ ...DETAILS, fields: [{ ...SUBTITLE, key: 'cover', label: 'Cover', kind: 'media' }] }])
	renderAt('/field-groups')
	const dialog = await openFields()

	await userEvent.click(within(dialog).getByRole('button', { name: 'Settings of Cover' }))
	const settings = await screen.findByRole('dialog', { name: 'Settings of Cover' })

	expect(within(settings).getByLabelText('Instructions')).toBeInTheDocument()
	expect(within(settings).queryByLabelText('Longest')).not.toBeInTheDocument()
	expect(within(settings).queryByLabelText('Default')).not.toBeInTheDocument()
})

test('shows the settings a field already carries', async () => {
	listing([
		{ ...DETAILS, fields: [{ ...SUBTITLE, settings: { instructions: 'Held.', maxlength: 40 } }] },
	])
	renderAt('/field-groups')
	const dialog = await openFields()

	await userEvent.click(within(dialog).getByRole('button', { name: 'Settings of Subtitle' }))
	const settings = await screen.findByRole('dialog', { name: 'Settings of Subtitle' })

	expect(within(settings).getByLabelText('Instructions')).toHaveValue('Held.')
	expect(within(settings).getByLabelText('Longest')).toHaveValue(40)
})

test('stores nothing for a setting left empty', async () => {
	let sent: unknown
	server.use(
		http.patch('/api/groups/3/fields/subtitle', async ({ request }) => {
			sent = await request.json()
			return HttpResponse.json(SUBTITLE)
		}),
	)
	renderAt('/field-groups')
	const dialog = await openFields()

	await userEvent.click(within(dialog).getByRole('button', { name: 'Settings of Subtitle' }))
	const settings = await screen.findByRole('dialog', { name: 'Settings of Subtitle' })
	await userEvent.click(within(settings).getByRole('button', { name: 'Save settings' }))

	await waitFor(() =>
		expect(sent).toEqual({ settings: {}, updated_at: '2026-08-01T10:00:00Z' }),
	)
})

test('stores no settings when the dialog is abandoned', async () => {
	let hit = false
	server.use(
		http.patch('/api/groups/3/fields/subtitle', () => {
			hit = true
			return HttpResponse.json(SUBTITLE)
		}),
	)
	renderAt('/field-groups')
	const dialog = await openFields()

	await userEvent.click(within(dialog).getByRole('button', { name: 'Settings of Subtitle' }))
	const settings = await screen.findByRole('dialog', { name: 'Settings of Subtitle' })
	await userEvent.type(within(settings).getByLabelText('Instructions'), 'Never stored.')
	await userEvent.click(within(settings).getByRole('button', { name: 'Cancel' }))

	await waitFor(() =>
		expect(screen.queryByRole('dialog', { name: 'Settings of Subtitle' })).not.toBeInTheDocument(),
	)
	expect(hit).toBe(false)
})

test('closes the settings and reports why they were turned away', async () => {
	server.use(
		http.patch('/api/groups/3/fields/subtitle', () =>
			HttpResponse.json(
				{ error: 'content: settings disagree', code: 'setting_bounds', meta: { setting: 'min' } },
				{ status: 422 },
			),
		),
	)
	renderAt('/field-groups')
	const dialog = await openFields()

	await userEvent.click(within(dialog).getByRole('button', { name: 'Settings of Subtitle' }))
	const settings = await screen.findByRole('dialog', { name: 'Settings of Subtitle' })
	await userEvent.click(within(settings).getByRole('button', { name: 'Save settings' }))

	expect(await within(dialog).findByRole('alert')).toHaveTextContent(/setting/i)
})

test('keeps what was typed when the settings are turned away', async () => {
	server.use(
		http.patch('/api/groups/3/fields/subtitle', () =>
			HttpResponse.json(
				{ error: 'content: setting shape', code: 'setting_shape', meta: { setting: 'maxlength' } },
				{ status: 422 },
			),
		),
	)
	renderAt('/field-groups')
	const dialog = await openFields()

	await userEvent.click(within(dialog).getByRole('button', { name: 'Settings of Subtitle' }))
	const settings = await screen.findByRole('dialog', { name: 'Settings of Subtitle' })
	await userEvent.type(within(settings).getByLabelText('Instructions'), 'Say who wrote it.')
	await userEvent.click(within(settings).getByRole('button', { name: 'Save settings' }))
	await within(dialog).findByRole('alert')
	await userEvent.click(within(dialog).getByRole('button', { name: 'Settings of Subtitle' }))

	const reopened = await screen.findByRole('dialog', { name: 'Settings of Subtitle' })
	expect(within(reopened).getByLabelText('Instructions')).toHaveValue('Say who wrote it.')
})

test('keeps a setting the dialog does not offer', async () => {
	listing([
		{ ...DETAILS, fields: [{ ...READING_TIME, settings: { placeholder: 'How many minutes' } }] },
	])
	let sent: unknown
	server.use(
		http.patch('/api/groups/3/fields/reading-time', async ({ request }) => {
			sent = await request.json()
			return HttpResponse.json(READING_TIME)
		}),
	)
	renderAt('/field-groups')
	const dialog = await openFields()

	await userEvent.click(within(dialog).getByRole('button', { name: 'Settings of Reading time' }))
	const settings = await screen.findByRole('dialog', { name: 'Settings of Reading time' })
	await userEvent.type(within(settings).getByLabelText('Lowest'), '1')
	await userEvent.click(within(settings).getByRole('button', { name: 'Save settings' }))

	await waitFor(() =>
		expect(sent).toEqual({
			settings: { placeholder: 'How many minutes', min: 1 },
			updated_at: '2026-08-01T10:00:00Z',
		}),
	)
})

test('offers a number field no placeholder, which its control never shows', async () => {
	listing([{ ...DETAILS, fields: [READING_TIME] }])
	renderAt('/field-groups')
	const dialog = await openFields()

	await userEvent.click(within(dialog).getByRole('button', { name: 'Settings of Reading time' }))
	const settings = await screen.findByRole('dialog', { name: 'Settings of Reading time' })

	expect(within(settings).queryByLabelText('Placeholder')).not.toBeInTheDocument()
})

test('declares a radio group as the choice kind presented as radio', async () => {
	let sent: unknown
	server.use(
		http.post('/api/groups/3/fields', async ({ request }) => {
			sent = await request.json()
			return HttpResponse.json({ ...SUBTITLE, key: 'style', label: 'Style', kind: 'choice' })
		}),
	)
	renderAt('/field-groups')
	const dialog = await openFields()

	await userEvent.type(within(dialog).getByLabelText('Name'), 'Style')
	await userEvent.click(within(dialog).getByRole('combobox', { name: 'Kind' }))
	await userEvent.click(await screen.findByRole('option', { name: 'Radio group' }))
	await userEvent.click(within(dialog).getByRole('button', { name: 'Add field' }))

	await waitFor(() =>
		expect(sent).toMatchObject({
			key: 'style',
			kind: 'choice',
			settings: { presentation: 'radio' },
		}),
	)
})

test('declares an email field as text checked as an email', async () => {
	let sent: unknown
	server.use(
		http.post('/api/groups/3/fields', async ({ request }) => {
			sent = await request.json()
			return HttpResponse.json({ ...SUBTITLE, key: 'contact', label: 'Contact' })
		}),
	)
	renderAt('/field-groups')
	const dialog = await openFields()

	await userEvent.type(within(dialog).getByLabelText('Name'), 'Contact')
	await userEvent.click(within(dialog).getByRole('combobox', { name: 'Kind' }))
	await userEvent.click(await screen.findByRole('option', { name: 'Email' }))
	await userEvent.click(within(dialog).getByRole('button', { name: 'Add field' }))

	await waitFor(() =>
		expect(sent).toMatchObject({ kind: 'text', settings: { variant: 'email' } }),
	)
})

test('declares a gallery as a media field holding many', async () => {
	let sent: unknown
	server.use(
		http.post('/api/groups/3/fields', async ({ request }) => {
			sent = await request.json()
			return HttpResponse.json({ ...SUBTITLE, key: 'gallery', label: 'Gallery', kind: 'media' })
		}),
	)
	renderAt('/field-groups')
	const dialog = await openFields()

	await userEvent.type(within(dialog).getByLabelText('Name'), 'Gallery')
	await userEvent.click(within(dialog).getByRole('combobox', { name: 'Kind' }))
	await userEvent.click(await screen.findByRole('option', { name: 'Gallery' }))
	await userEvent.click(within(dialog).getByRole('button', { name: 'Add field' }))

	await waitFor(() => expect(sent).toMatchObject({ kind: 'media', many: true }))
	expect((sent as { settings?: unknown }).settings).toBeUndefined()
})

test('offers a choice field its choices and stores the pairs', async () => {
	listing([
		{ ...DETAILS, fields: [{ ...SUBTITLE, key: 'style', label: 'Style', kind: 'choice' }] },
	])
	let sent: unknown
	server.use(
		http.patch('/api/groups/3/fields/style', async ({ request }) => {
			sent = await request.json()
			return HttpResponse.json({ ...SUBTITLE, key: 'style', kind: 'choice' })
		}),
	)
	renderAt('/field-groups')
	const dialog = await openFields()

	await userEvent.click(within(dialog).getByRole('button', { name: 'Settings of Style' }))
	const settings = await screen.findByRole('dialog', { name: 'Settings of Style' })
	await userEvent.click(within(settings).getByRole('button', { name: 'Add choice' }))
	await userEvent.type(within(settings).getByLabelText('Value'), 'ipa')
	await userEvent.type(within(settings).getByLabelText('Label'), 'IPA')
	await userEvent.click(within(settings).getByRole('button', { name: 'Save settings' }))

	await waitFor(() =>
		expect(sent).toEqual({
			settings: { choices: [{ value: 'ipa', label: 'IPA' }] },
			updated_at: '2026-08-01T10:00:00Z',
		}),
	)
})

test('shows the pairs a choice field already carries and drops a removed one', async () => {
	listing([
		{
			...DETAILS,
			fields: [
				{
					...SUBTITLE,
					key: 'style',
					label: 'Style',
					kind: 'choice',
					settings: {
						choices: [
							{ value: 'ipa', label: 'IPA' },
							{ value: 'stout', label: 'Stout' },
						],
					},
				},
			],
		},
	])
	let sent: unknown
	server.use(
		http.patch('/api/groups/3/fields/style', async ({ request }) => {
			sent = await request.json()
			return HttpResponse.json({ ...SUBTITLE, key: 'style', kind: 'choice' })
		}),
	)
	renderAt('/field-groups')
	const dialog = await openFields()

	await userEvent.click(within(dialog).getByRole('button', { name: 'Settings of Style' }))
	const settings = await screen.findByRole('dialog', { name: 'Settings of Style' })
	expect(within(settings).getAllByLabelText('Value')).toHaveLength(2)

	await userEvent.type(within(settings).getAllByLabelText('Label')[1], ' Beer')
	await userEvent.type(within(settings).getAllByLabelText('Value')[1], 's')
	await userEvent.click(within(settings).getAllByRole('button', { name: 'Remove' })[0])
	await userEvent.click(within(settings).getByRole('button', { name: 'Save settings' }))

	await waitFor(() =>
		expect(sent).toEqual({
			settings: { choices: [{ value: 'stouts', label: 'Stout Beer' }] },
			updated_at: '2026-08-01T10:00:00Z',
		}),
	)
})

test('shows only the pairs a stray choices value actually holds', async () => {
	listing([
		{
			...DETAILS,
			fields: [
				{
					...SUBTITLE,
					key: 'style',
					label: 'Style',
					kind: 'choice',
					settings: {
						choices: [{ value: 'ipa', label: 'IPA' }, 'stray', { value: 1, label: 'One' }],
					},
				},
			],
		},
	])
	renderAt('/field-groups')
	const dialog = await openFields()

	await userEvent.click(within(dialog).getByRole('button', { name: 'Settings of Style' }))
	const settings = await screen.findByRole('dialog', { name: 'Settings of Style' })

	expect(within(settings).getAllByLabelText('Value')).toHaveLength(1)
	expect(within(settings).getByLabelText('Value')).toHaveValue('ipa')
})

test('offers a choice field the flags its kind takes', async () => {
	listing([
		{ ...DETAILS, fields: [{ ...SUBTITLE, key: 'style', label: 'Style', kind: 'choice' }] },
	])
	let sent: unknown
	server.use(
		http.patch('/api/groups/3/fields/style', async ({ request }) => {
			sent = await request.json()
			return HttpResponse.json({ ...SUBTITLE, key: 'style', kind: 'choice' })
		}),
	)
	renderAt('/field-groups')
	const dialog = await openFields()

	await userEvent.click(within(dialog).getByRole('button', { name: 'Settings of Style' }))
	const settings = await screen.findByRole('dialog', { name: 'Settings of Style' })
	await userEvent.click(within(settings).getByRole('combobox', { name: 'Allow custom' }))
	await userEvent.click(await screen.findByRole('option', { name: 'Yes' }))
	await userEvent.click(within(settings).getByRole('button', { name: 'Save settings' }))

	await waitFor(() =>
		expect(sent).toEqual({ settings: { allow_custom: true }, updated_at: '2026-08-01T10:00:00Z' }),
	)
})

test('offers a boolean field the value it starts on', async () => {
	listing([
		{ ...DETAILS, fields: [{ ...SUBTITLE, key: 'boxed', label: 'Boxed', kind: 'boolean' }] },
	])
	let sent: unknown
	server.use(
		http.patch('/api/groups/3/fields/boxed', async ({ request }) => {
			sent = await request.json()
			return HttpResponse.json(SUBTITLE)
		}),
	)
	renderAt('/field-groups')
	const dialog = await openFields()

	await userEvent.click(within(dialog).getByRole('button', { name: 'Settings of Boxed' }))
	const settings = await screen.findByRole('dialog', { name: 'Settings of Boxed' })
	await userEvent.click(within(settings).getByRole('combobox', { name: 'Default' }))
	await userEvent.click(await screen.findByRole('option', { name: 'Yes' }))
	await userEvent.click(within(settings).getByRole('button', { name: 'Save settings' }))

	await waitFor(() =>
		expect(sent).toEqual({ settings: { default: true }, updated_at: '2026-08-01T10:00:00Z' }),
	)
})
