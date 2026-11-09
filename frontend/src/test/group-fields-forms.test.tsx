// SPDX-License-Identifier: Apache-2.0

import { http, HttpResponse, server } from '@gophenberg/frontend-sdk/testing'
import { act, screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { beforeEach, expect, test } from 'vitest'

import { groupsQueryKey } from '../content/groups'
import { gate } from './gate'
import { DETAILS, EXTRAS, listing, POST_TYPE, SUBTITLE } from './groupFieldsFixtures'
import { renderAt } from './render'
import { expectReadOnly, waitBusy } from './typeFixtures'

/**
 * Opens the fields of the article details group.
 * @returns The dialog the group opened.
 */
async function openFields() {
	const table = await screen.findByRole('region', { name: 'Field Groups' })
	const row = within(table).getByRole('row', { name: /Article details/ })
	await userEvent.click(within(row).getByRole('button', { name: 'Fields' }))
	return screen.findByRole('dialog')
}

beforeEach(() => {
	server.use(http.get('/api/types', () => HttpResponse.json({ items: [POST_TYPE] })))
	listing([DETAILS, EXTRAS])
})

test('says inside its own dialog why a field declared into a container was turned away', async () => {
	listing([
		{
			...DETAILS,
			fields: [{ ...SUBTITLE, key: 'author', label: 'Author', kind: 'section', fields: [] }],
		},
		EXTRAS,
	])
	server.use(
		http.post('/api/groups/3/fields/author', () =>
			HttpResponse.json({ error: 'content: field key taken', code: 'field_taken' }, { status: 422 }),
		),
	)
	renderAt('/field-groups')
	const dialog = await openFields()

	await userEvent.click(within(dialog).getByRole('button', { name: 'Add field to Author' }))
	const adding = await screen.findByRole('dialog', { name: 'Add field to Author' })
	await userEvent.type(within(adding).getByLabelText('Name'), 'Full name')
	await userEvent.click(within(adding).getByRole('button', { name: 'Add field' }))

	const said = 'A field with that name already exists on the items this group reaches. Pick another name.'
	expect(await within(adding).findByText(said)).toBeInTheDocument()
	expect(screen.getAllByText(said)).toHaveLength(1)
	expect(within(adding).getByLabelText('Name')).toHaveValue('Full name')
})

test('carries a field to the first place still on offer once the group picked left while it asked', async () => {
	let sent: unknown
	const later = { ...EXTRAS, id: 5, key: 'later', title: 'Later', position: 3 }
	server.use(
		http.post('/api/groups/3/fields/subtitle/move', async ({ request }) => {
			sent = await request.json()
			return HttpResponse.json(SUBTITLE)
		}),
	)
	listing([DETAILS, EXTRAS, later])
	const client = renderAt('/field-groups')
	const dialog = await openFields()
	await userEvent.click(within(dialog).getByRole('button', { name: 'Move Subtitle elsewhere' }))
	const moving = await screen.findByRole('dialog', { name: 'Move Subtitle elsewhere' })
	await userEvent.click(within(moving).getByLabelText('Destination'))
	await userEvent.click(await screen.findByRole('option', { name: 'Later' }))

	listing([DETAILS, EXTRAS])
	await act(async () => {
		await client.invalidateQueries({ queryKey: groupsQueryKey })
	})
	await waitFor(() => expect(within(moving).getByLabelText('Destination')).toHaveTextContent('Extras'))
	await userEvent.click(within(moving).getByRole('button', { name: 'Move the field' }))

	await waitFor(() => expect(sent).toEqual({ to_group: 4 }))
})

test('keeps the rename open and says inside why it was turned away', async () => {
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

	expect(await within(renaming).findByText('content: field not found')).toBeInTheDocument()
	expect(screen.getAllByText('content: field not found')).toHaveLength(1)
})

test('keeps the controls of a field declared into a container read only while it is declared', async () => {
	const answer = gate()
	listing([
		{
			...DETAILS,
			fields: [{ ...SUBTITLE, key: 'author', label: 'Author', kind: 'section', fields: [] }],
		},
		EXTRAS,
	])
	server.use(
		http.post('/api/groups/3/fields/author', async () => {
			await answer.held
			return HttpResponse.json({ ...SUBTITLE, key: 'name', label: 'Name' }, { status: 201 })
		}),
	)
	renderAt('/field-groups')
	const dialog = await openFields()

	await userEvent.click(within(dialog).getByRole('button', { name: 'Add field to Author' }))
	const adding = await screen.findByRole('dialog', { name: 'Add field to Author' })
	await userEvent.type(within(adding).getByLabelText('Name'), 'Full name')
	await userEvent.click(within(adding).getByRole('button', { name: 'Add field' }))
	await waitBusy(adding, 'Add field')

	expectReadOnly(adding)
	answer.release()
	await waitFor(() => expect(screen.queryByRole('dialog', { name: 'Add field to Author' })).not.toBeInTheDocument())
})

test('keeps the name and the kind of a new field read only while it is declared', async () => {
	const answer = gate()
	server.use(
		http.post('/api/groups/3/fields', async () => {
			await answer.held
			return HttpResponse.json({ ...SUBTITLE, key: 'summary', label: 'Summary' }, { status: 201 })
		}),
	)
	renderAt('/field-groups')
	const dialog = await openFields()

	await userEvent.type(within(dialog).getByLabelText('Name'), 'Summary')
	await userEvent.click(within(dialog).getByRole('button', { name: 'Add field' }))
	await waitBusy(dialog, 'Add field')

	expect(within(dialog).getByLabelText('Name')).toHaveAttribute('readonly')
	expect(within(dialog).getByRole('combobox', { name: 'Kind' })).toHaveAttribute('aria-readonly', 'true')
	answer.release()
	await waitFor(() => expect(within(dialog).getByLabelText('Name')).not.toHaveAttribute('readonly'))
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
