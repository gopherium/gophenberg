// SPDX-License-Identifier: Apache-2.0

import { http, HttpResponse, server } from '@gophenberg/frontend-sdk/testing'
import { screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { expect, test } from 'vitest'

import { gate } from './gate'
import {
	DETAILS,
	EXTRAS,
	fieldRow,
	fieldsOf,
	listing,
	NO_FIELDS,
	NO_FIELDS_SENTENCE,
	openFields,
	READING_TIME,
	serveFieldGroups,
	SUBTITLE,
} from './groupFieldsFixtures'
import { renderAt } from './render'
import { declarations } from './stylesheet'
import { findToast } from './typeFixtures'

const ORDER_REJECTED = { error: 'content: the order has to name every field', code: 'field_order' }

serveFieldGroups()

/**
 * Reports whether one row of the fields table stands above another.
 * @param row - The row expected higher.
 * @param other - The row expected lower.
 * @returns Whether the row comes first.
 */
function standsAbove(row: HTMLElement, other: HTMLElement) {
	return Boolean(row.compareDocumentPosition(other) & Node.DOCUMENT_POSITION_FOLLOWING)
}

/**
 * Returns the place each named button takes among the buttons under a field label, and how many stand there.
 * @param main - The main work of the page.
 * @param label - The label of the field.
 * @param names - The names of the buttons, in the order expected.
 * @returns The count of buttons and the place of each named one.
 */
async function buttonsUnder(main: HTMLElement, label: string, names: string[]) {
	const [cell] = within(await fieldRow(main, label)).getAllByRole('cell')
	const shown = within(cell).getAllByRole('button')
	const places = names.map((name) => shown.indexOf(within(cell).getByRole('button', { name })))
	return { count: shown.length, places }
}

test('lists the fields the group holds under their kind', async () => {
	renderAt('/field-groups')

	const dialog = await openFields()

	const text = await fieldRow(dialog, 'Subtitle')
	const number = await fieldRow(dialog, 'Reading time')
	expect(within(text).getByText('Text')).toBeInTheDocument()
	expect(within(number).getByText('Number')).toBeInTheDocument()
})

test('lists the fields in a table named Fields under Label, Kind and API name, one row group per field', async () => {
	renderAt('/field-groups')

	const main = await openFields()

	const fields = fieldsOf(main)
	expect(fields).toHaveClass('godmin-table-scroll')
	expect(fields).toHaveAttribute('tabindex', '0')
	const headers = within(fields).getAllByRole('columnheader')
	expect(headers.map((header) => header.textContent)).toEqual(['Label', 'Kind', 'API name', 'Actions'])
	expect(headers[3]).toHaveClass('godmin-table__actions')
	expect(within(fields).getAllByRole('rowgroup')).toHaveLength(3)
})

test('shows the key a field stores its values under as code', async () => {
	renderAt('/field-groups')

	const main = await openFields()

	const row = await fieldRow(main, 'Reading time')
	expect(within(row).getByText('reading-time').tagName).toBe('CODE')
})

test('names a field stored as text checked as an email by the Email it was picked as', async () => {
	const storedAsEmail = { ...SUBTITLE, key: 'contact', label: 'Contact', settings: { variant: 'email' } }
	listing([{ ...DETAILS, fields: [storedAsEmail] }, EXTRAS])
	renderAt('/field-groups')

	const main = await openFields()

	const row = await fieldRow(main, 'Contact')
	expect(within(row).getByText('Email')).toBeInTheDocument()
	expect(within(row).queryByText('Text')).not.toBeInTheDocument()
})

test('keeps Move first among the buttons under a field label, then the ones its kind offers', async () => {
	const emptyContainer = { ...SUBTITLE, key: 'team', label: 'Team', kind: 'repeater', fields: [] }
	listing([{ ...DETAILS, fields: [SUBTITLE, READING_TIME, emptyContainer] }, EXTRAS])
	renderAt('/field-groups')

	const main = await openFields()

	const text = [
		'Move Subtitle elsewhere',
		'Require Subtitle',
		'Show Subtitle in the list',
		'Rules showing Subtitle',
		'Rename Subtitle',
		'Settings of Subtitle',
	]
	expect(await buttonsUnder(main, 'Subtitle', text)).toEqual({ count: 6, places: [0, 1, 2, 3, 4, 5] })
	const container = [
		'Move Team elsewhere',
		'Require Team',
		'Rules showing Team',
		'Rename Team',
		'Settings of Team',
		'Add field to Team',
	]
	expect(await buttonsUnder(main, 'Team', container)).toEqual({ count: 6, places: [0, 1, 2, 3, 4, 5] })
})

test('keeps the words of each button under a label on one line however narrow the column gets', async () => {
	renderAt('/field-groups')

	const main = await openFields()

	const rename = within(await fieldRow(main, 'Subtitle')).getByRole('button', { name: 'Rename Subtitle' })
	expect(rename.parentElement).toHaveClass('gophenberg-fields__actions')
	expect(declarations('.gophenberg-fields__actions button')).toEqual({ 'white-space': 'nowrap' })
})

test('marks the fields an item cannot be published without', async () => {
	renderAt('/field-groups')

	const dialog = await openFields()
	const [required] = within(await fieldRow(dialog, 'Reading time')).getAllByRole('cell')

	expect(within(required).getByText('Required')).toBeInTheDocument()
})

test('says so when a group holds no fields yet', async () => {
	renderAt('/field-groups')

	const dialog = await openFields('Extras')

	const fields = fieldsOf(dialog)
	expect(within(fields).getByRole('heading', { name: NO_FIELDS })).toBeInTheDocument()
	expect(within(fields).getByText(NO_FIELDS_SENTENCE)).toBeInTheDocument()
	expect(within(fields).queryByRole('table')).not.toBeInTheDocument()
	expect(within(fields).queryByRole('button')).not.toBeInTheDocument()
	expect(fields).toHaveAttribute('tabindex', '-1')
})

test('deletes a field from its trash once the red confirm warning about its values is accepted', async () => {
	let hit = false
	server.use(
		http.delete('/api/groups/3/fields/subtitle', () => {
			hit = true
			return new HttpResponse(null, { status: 204 })
		}),
	)
	renderAt('/field-groups')
	const dialog = await openFields()
	const row = await fieldRow(dialog, 'Subtitle')

	const trash = within(row).getByRole('button', { name: 'Delete Subtitle' })
	expect(trash.closest('td')).toHaveClass('godmin-table__actions')
	expect(within(row).queryByText('Delete')).not.toBeInTheDocument()
	await userEvent.click(trash)
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
	const down = within(await fieldRow(dialog, 'Subtitle')).getByRole('button', { name: 'Move Subtitle down' })

	expect(down.closest('td')).toHaveClass('godmin-table__actions')
	await userEvent.click(down)

	await waitFor(() => expect(sent).toEqual({ order: ['reading-time', 'subtitle'] }))
})

test('moves a field at once while the order it asks for is on its way', async () => {
	const answer = gate()
	server.use(
		http.put('/api/groups/3/fields/order', async () => {
			await answer.held
			return HttpResponse.json({ items: [READING_TIME, SUBTITLE] })
		}),
	)
	renderAt('/field-groups')
	const main = await openFields()

	await userEvent.click(within(await fieldRow(main, 'Subtitle')).getByRole('button', { name: 'Move Subtitle down' }))

	expect(standsAbove(await fieldRow(main, 'Reading time'), await fieldRow(main, 'Subtitle'))).toBe(true)
	answer.release()
	expect(await findToast('Order stored.')).toBeInTheDocument()
})

test('sends one order at a time and stores the newest once the order on its way is answered', async () => {
	const answer = gate()
	const sent: unknown[] = []
	server.use(
		http.put('/api/groups/3/fields/order', async ({ request }) => {
			sent.push(await request.json())
			await answer.held
			return HttpResponse.json({ items: [SUBTITLE, READING_TIME] })
		}),
	)
	renderAt('/field-groups')
	const main = await openFields()

	await userEvent.click(within(await fieldRow(main, 'Subtitle')).getByRole('button', { name: 'Move Subtitle down' }))
	await waitFor(() => expect(sent).toHaveLength(1))
	await userEvent.click(within(await fieldRow(main, 'Subtitle')).getByRole('button', { name: 'Move Subtitle up' }))
	expect(sent).toHaveLength(1)
	answer.release()

	await waitFor(() =>
		expect(sent).toEqual([{ order: ['reading-time', 'subtitle'] }, { order: ['subtitle', 'reading-time'] }]),
	)
	expect(await findToast('Order stored.')).toBeInTheDocument()
})

test('puts a field back in its stored place when the order it asked for is rejected', async () => {
	server.use(http.put('/api/groups/3/fields/order', () => HttpResponse.json(ORDER_REJECTED, { status: 422 })))
	renderAt('/field-groups')
	const main = await openFields()

	await userEvent.click(within(await fieldRow(main, 'Subtitle')).getByRole('button', { name: 'Move Subtitle down' }))

	expect(await within(main).findByRole('alert')).toHaveTextContent(/every field/)
	expect(standsAbove(await fieldRow(main, 'Subtitle'), await fieldRow(main, 'Reading time'))).toBe(true)
})

test('reads the groups again once a later order is rejected, showing the order the first one stored', async () => {
	const answer = gate()
	let orders = 0
	server.use(
		http.put('/api/groups/3/fields/order', async () => {
			orders += 1
			if (orders > 1) {
				return HttpResponse.json(ORDER_REJECTED, { status: 422 })
			}
			await answer.held
			listing([{ ...DETAILS, fields: [READING_TIME, SUBTITLE] }, EXTRAS])
			return HttpResponse.json({ items: [READING_TIME, SUBTITLE] })
		}),
	)
	renderAt('/field-groups')
	const main = await openFields()

	await userEvent.click(within(await fieldRow(main, 'Subtitle')).getByRole('button', { name: 'Move Subtitle down' }))
	await userEvent.click(within(await fieldRow(main, 'Subtitle')).getByRole('button', { name: 'Move Subtitle up' }))
	answer.release()

	expect(await within(main).findByRole('alert')).toHaveTextContent(/every field/)
	await waitFor(async () =>
		expect(standsAbove(await fieldRow(main, 'Reading time'), await fieldRow(main, 'Subtitle'))).toBe(true),
	)
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

	expect(await fieldRow(dialog, 'Subtitle')).toBeInTheDocument()
	expect(screen.queryByRole('dialog')).not.toBeInTheDocument()
})

test('reports a refused reorder inside the dialog', async () => {
	server.use(http.put('/api/groups/3/fields/order', () => HttpResponse.json(ORDER_REJECTED, { status: 422 })))
	renderAt('/field-groups')
	const dialog = await openFields()

	await userEvent.click(within(dialog).getByRole('button', { name: 'Move Subtitle down' }))

	expect(await within(dialog).findByRole('alert')).toHaveTextContent(/every field/)
})
