// SPDX-License-Identifier: Apache-2.0

import { http, HttpResponse, server } from '@gophenberg/frontend-sdk/testing'
import { screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { expect, test } from 'vitest'

import { openFields, READING_TIME, serveFieldGroups, SUBTITLE } from './groupFieldsFixtures'
import { renderAt } from './render'

serveFieldGroups()

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

test('deletes a field once the red confirm warning about its values is accepted', async () => {
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
	const warning = await screen.findByRole('alertdialog', { name: 'Delete "Subtitle"?' })
	expect(warning).toHaveAccessibleDescription(
		'Every value stored under this field goes with it, in every item the group reaches and in the revisions '
			+ 'behind them. This action cannot be undone.',
	)
	const confirm = within(warning).getByRole('button', { name: 'Delete the field' })
	expect(confirm.className).toMatch(/irreversible/)
	await userEvent.click(confirm)

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

test('closes the warning and reports why a delete was turned away', async () => {
	server.use(
		http.delete('/api/groups/3/fields/subtitle', () =>
			HttpResponse.json({ error: 'content: field not found', code: 'field_unknown' }, { status: 404 }),
		),
	)
	renderAt('/field-groups')
	const dialog = await openFields()

	await userEvent.click(within(dialog).getByRole('button', { name: 'Delete Subtitle' }))
	const warning = await screen.findByRole('alertdialog', { name: 'Delete "Subtitle"?' })
	await userEvent.click(within(warning).getByRole('button', { name: 'Delete the field' }))

	expect(await within(dialog).findByRole('alert')).toHaveTextContent(/field/i)
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
	const warning = await screen.findByRole('alertdialog', { name: 'Delete "Subtitle"?' })
	await userEvent.click(within(warning).getByRole('button', { name: 'Cancel' }))

	await waitFor(() =>
		expect(screen.queryByRole('alertdialog', { name: 'Delete "Subtitle"?' })).not.toBeInTheDocument(),
	)
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

test('opens the fields on the page of the group with no dialog over them', async () => {
	renderAt('/field-groups')
	const dialog = await openFields()

	expect(within(dialog).getByRole('listitem', { name: 'Subtitle' })).toBeInTheDocument()
	expect(screen.queryByRole('dialog')).not.toBeInTheDocument()
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
