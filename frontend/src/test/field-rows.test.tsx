// SPDX-License-Identifier: Apache-2.0

import { http, HttpResponse, server } from '@gophenberg/frontend-sdk/testing'
import { act, screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { expect, test } from 'vitest'

import { groupsQueryKey } from '../content/groups'
import { gate } from './gate'
import {
	DETAILS,
	EXTRAS,
	fieldRow,
	fieldsOf,
	listing,
	NO_FIELDS,
	NO_FIELDS_SENTENCE,
	openField,
	openFields,
	partOf,
	READING_TIME,
	serveFieldGroups,
	SUBTITLE,
	toggleOf,
} from './groupFieldsFixtures'
import { renderAt } from './render'

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
 * Serves the groups from a list a test may change, counting each read.
 * @returns The reads so far and the groups the next read answers with.
 */
function countedReads() {
	const reads: { count: number; groups: unknown[] } = { count: 0, groups: [DETAILS, EXTRAS] }
	server.use(
		http.get('/api/groups', () => {
			reads.count += 1
			return HttpResponse.json({ items: reads.groups })
		}),
	)
	return reads
}

/**
 * Returns the Label cell of a field's own row.
 * @param row - The field's own row.
 * @returns The cell.
 */
function labelCell(row: HTMLElement) {
	return (row as HTMLTableRowElement).cells[0]
}

/**
 * Returns what an open field shows beside one of its facts.
 * @param part - The row holding the open part.
 * @param name - The name of the fact.
 * @returns The element holding the fact's value.
 */
function factOf(part: HTMLElement, name: string) {
	return within(part).getByText(name, { exact: true }).nextElementSibling as HTMLElement
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

test('holds the label button, then Move and the badges outside it, and no other button in the Label cell', async () => {
	const emptyContainer = { ...SUBTITLE, key: 'team', label: 'Team', kind: 'repeater', fields: [] }
	listing([{ ...DETAILS, fields: [SUBTITLE, READING_TIME, emptyContainer] }, EXTRAS])
	renderAt('/field-groups')

	const main = await openFields()

	for (const label of ['Reading time', 'Team']) {
		const row = await fieldRow(main, label)
		const toggle = toggleOf(row, label)
		const move = within(labelCell(row)).getByRole('button', { name: `Move ${label} elsewhere` })
		expect(within(labelCell(row)).getAllByRole('button')).toEqual([toggle, move])
		expect(toggle).toHaveTextContent(new RegExp(`^${label}$`))
		expect(toggle).not.toContainElement(move)
	}
	const required = within(labelCell(await fieldRow(main, 'Reading time'))).getByText('Required')
	expect(toggleOf(await fieldRow(main, 'Reading time'), 'Reading time')).not.toContainElement(required)
})

test('draws the label as a toggle with a chevron and the open part as one cell across the table', async () => {
	renderAt('/field-groups')
	const main = await openFields()

	const row = await fieldRow(main, 'Subtitle')

	const toggle = toggleOf(row, 'Subtitle')
	expect(toggle).toHaveClass('godmin-table__toggle')
	expect(toggle.querySelector('svg')).toHaveClass('godmin-table__toggle-icon')
	const part = partOf(row)
	expect(row.parentElement?.tagName).toBe('TBODY')
	expect(part.parentElement).toBe(row.parentElement)
	expect(part).toHaveClass('godmin-table__detail')
	expect(part).toHaveAttribute('hidden', 'until-found')
	const [cell] = (part as HTMLTableRowElement).cells
	expect(cell.colSpan).toBe(4)
	const motion = cell.firstElementChild as HTMLElement
	expect(motion).toHaveClass('godmin-table__detail-motion')
	const plain = motion.firstElementChild as HTMLElement
	expect(plain.className).toBe('')
	expect(plain.firstElementChild).toHaveClass('godmin-table__detail-body')
})

test('opens a field under its row from its label and closes it again, the label saying if it is open', async () => {
	renderAt('/field-groups')
	const main = await openFields()
	const toggle = toggleOf(await fieldRow(main, 'Subtitle'), 'Subtitle')
	expect(toggle).toHaveAttribute('aria-expanded', 'false')

	await userEvent.click(toggle)

	expect(toggle).toHaveAttribute('aria-expanded', 'true')
	expect(partOf(await fieldRow(main, 'Subtitle'))).not.toHaveAttribute('hidden')
	expect(toggleOf(await fieldRow(main, 'Reading time'), 'Reading time')).toHaveAttribute('aria-expanded', 'false')
	await userEvent.click(toggle)
	expect(toggle).toHaveAttribute('aria-expanded', 'false')
})

test('leaves the focus on the label as a field opens and closes from the keyboard', async () => {
	renderAt('/field-groups')
	const main = await openFields()
	const toggle = toggleOf(await fieldRow(main, 'Subtitle'), 'Subtitle')
	act(() => toggle.focus())

	await userEvent.keyboard('{Enter}')

	expect(toggle).toHaveAttribute('aria-expanded', 'true')
	expect(toggle).toHaveFocus()
	await userEvent.keyboard('{Enter}')
	expect(toggle).toHaveAttribute('aria-expanded', 'false')
	expect(toggle).toHaveFocus()
})

test('puts the focus back on the label once Close field closes the field', async () => {
	renderAt('/field-groups')
	const main = await openFields()
	const part = await openField(main, 'Subtitle')

	await userEvent.click(within(part).getByRole('button', { name: 'Close field' }))

	const toggle = toggleOf(await fieldRow(main, 'Subtitle'), 'Subtitle')
	expect(toggle).toHaveAttribute('aria-expanded', 'false')
	expect(toggle).toHaveFocus()
})

test('keeps a field open while the groups are read again', async () => {
	const summary = { ...SUBTITLE, key: 'summary', label: 'Summary' }
	const client = renderAt('/field-groups')
	const main = await openFields()
	await openField(main, 'Subtitle')

	listing([{ ...DETAILS, fields: [SUBTITLE, READING_TIME, summary] }, EXTRAS])
	await act(async () => {
		await client.invalidateQueries({ queryKey: groupsQueryKey })
	})

	expect(await fieldRow(main, 'Summary')).toBeInTheDocument()
	expect(toggleOf(await fieldRow(main, 'Subtitle'), 'Subtitle')).toHaveAttribute('aria-expanded', 'true')
})

test('marks a closed field holding unsaved changes, and no other', async () => {
	renderAt('/field-groups')
	const main = await openFields()
	const part = await openField(main, 'Subtitle')

	await userEvent.type(within(part).getByRole('textbox', { name: 'Label' }), ' line')
	await userEvent.click(within(part).getByRole('button', { name: 'Close field' }))

	const row = await fieldRow(main, 'Subtitle')
	expect(within(labelCell(row)).getByText('Unsaved changes')).toBeInTheDocument()
	expect(toggleOf(row, 'Subtitle')).toHaveTextContent(/^Subtitle$/)
	expect(within(labelCell(await fieldRow(main, 'Reading time'))).queryByText('Unsaved changes')).toBeNull()
})

test('shows an open field in tabs, its API name and kind as text and a note on renaming under Label', async () => {
	renderAt('/field-groups')
	const main = await openFields()

	const part = await openField(main, 'Reading time')

	const tabs = within(part).getAllByRole('tab')
	expect(tabs.map((tab) => tab.textContent)).toEqual(['General', 'Validation', 'Presentation'])
	expect(tabs[0]).toHaveAttribute('aria-selected', 'true')
	expect(within(part).getByRole('textbox', { name: 'Label' })).toHaveValue('Reading time')
	expect(within(part).getByRole('textbox', { name: 'Label' })).toHaveAccessibleDescription(
		'The name changes. Nothing stored under this field moves.',
	)
	expect(factOf(part, 'API name')).toHaveTextContent('reading-time')
	expect(within(factOf(part, 'API name')).getByText('reading-time').tagName).toBe('CODE')
	expect(factOf(part, 'Kind')).toHaveTextContent('Number')
})

test('shows what a relation points at and how many it holds, the key of a type out of reach', async () => {
	const author = { ...SUBTITLE, key: 'author', label: 'Author', kind: 'relation', relates_to: 'post' }
	const editors = { ...author, key: 'editors', label: 'Editors', many: true, relates_to: 'gone' }
	listing([{ ...DETAILS, fields: [author, editors] }, EXTRAS])
	renderAt('/field-groups')
	const main = await openFields()

	const one = await openField(main, 'Author')
	const many = await openField(main, 'Editors')

	expect(factOf(one, 'Points at')).toHaveTextContent('Posts')
	expect(factOf(one, 'Holds')).toHaveTextContent('One target')
	expect(factOf(many, 'Points at')).toHaveTextContent('gone')
	expect(factOf(many, 'Holds')).toHaveTextContent('Many targets')
})

test('shows what a Linked from field reads, the keys of a source out of reach', async () => {
	const maker = { ...SUBTITLE, key: 'maker', label: 'Maker', kind: 'relation', relates_to: 'post' }
	const cars = { ...EXTRAS, id: 6, key: 'cars', title: 'Cars', position: 4, fields: [maker] }
	const linked = {
		...SUBTITLE,
		key: 'linked-from',
		label: 'Linked from',
		kind: 'backlinks',
		settings: { source_group: 'cars', source_field: ['maker'] },
	}
	const lost = { ...linked, key: 'lost', label: 'Lost', settings: { source_group: 'boats', source_field: ['skipper'] } }
	const throughContainer = { source_group: 'cars', source_field: ['crew', 'maker'] }
	const deep = { ...linked, key: 'deep', label: 'Deep', settings: throughContainer }
	listing([{ ...DETAILS, fields: [linked, lost, deep] }, cars])
	renderAt('/field-groups')
	const main = await openFields()

	const reading = await openField(main, 'Linked from')
	const gone = await openField(main, 'Lost')
	const inside = await openField(main, 'Deep')

	expect(factOf(reading, 'Reads from')).toHaveTextContent('Cars')
	expect(factOf(reading, 'Through')).toHaveTextContent('Maker')
	expect(factOf(gone, 'Reads from')).toHaveTextContent('boats')
	expect(factOf(gone, 'Through')).toHaveTextContent('skipper')
	expect(within(inside).queryByText('Reads from')).toBeNull()
	expect(factOf(inside, 'Through')).toHaveTextContent(
		'It reads a relation inside a container, and only an import can point it elsewhere.',
	)
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

test('adds to the red confirm that the unsaved changes inside a field are lost with it', async () => {
	const role = { ...SUBTITLE, key: 'role', label: 'Role' }
	const team = { ...SUBTITLE, key: 'team', label: 'Team', kind: 'repeater', fields: [role] }
	listing([{ ...DETAILS, fields: [SUBTITLE, team] }, EXTRAS])
	renderAt('/field-groups')
	const main = await openFields()
	await openField(main, 'Team')
	const inside = await openField(main, 'Role')
	await userEvent.type(within(inside).getByRole('textbox', { name: 'Label' }), ' line')

	await userEvent.click(within(await fieldRow(main, 'Team')).getByRole('button', { name: 'Delete Team' }))

	const warning = await screen.findByRole('alertdialog', { name: 'Delete "Team"?' })
	expect(warning).toHaveAccessibleDescription(
		'Every value stored under this field goes with it, in every item the group reaches and in the revisions '
			+ 'behind them. Unsaved changes inside Team are lost too. This action cannot be undone.',
	)
	await userEvent.click(within(warning).getByRole('button', { name: 'Cancel' }))
	await waitFor(() => expect(screen.queryByRole('alertdialog')).toBeNull())
	await userEvent.click(within(await fieldRow(main, 'Subtitle')).getByRole('button', { name: 'Delete Subtitle' }))
	expect(await screen.findByRole('alertdialog', { name: 'Delete "Subtitle"?' })).toHaveAccessibleDescription(
		/^Every value stored under this field goes with it, .* behind them\. This action cannot be undone\.$/,
	)
})

test('drops the draft of a deleted field, so a field added later under its key starts with none', async () => {
	const summary = { ...SUBTITLE, key: 'summary', label: 'Summary' }
	server.use(
		http.delete('/api/groups/3/fields/summary', () => {
			listing([DETAILS, EXTRAS])
			return new HttpResponse(null, { status: 204 })
		}),
		http.post('/api/groups/3/fields', () => {
			listing([{ ...DETAILS, fields: [SUBTITLE, READING_TIME, summary] }, EXTRAS])
			return HttpResponse.json(summary, { status: 201 })
		}),
	)
	listing([{ ...DETAILS, fields: [SUBTITLE, READING_TIME, summary] }, EXTRAS])
	renderAt('/field-groups')
	const main = await openFields()
	const part = await openField(main, 'Summary')
	await userEvent.type(within(part).getByRole('textbox', { name: 'Label' }), ' line')
	await userEvent.click(within(await fieldRow(main, 'Summary')).getByRole('button', { name: 'Delete Summary' }))
	const warning = await screen.findByRole('alertdialog', { name: 'Delete "Summary"?' })
	await userEvent.click(within(warning).getByRole('button', { name: 'Delete the field' }))
	await waitFor(() => expect(within(fieldsOf(main)).queryByText('Summary')).toBeNull())

	const adding = within(main).getAllByRole('textbox', { name: 'Label' }).at(-1) as HTMLElement
	await userEvent.type(adding, 'Summary')
	await userEvent.click(within(main).getByRole('button', { name: 'Add field' }))

	const row = await fieldRow(main, 'Summary')
	expect(within(labelCell(row)).queryByText('Unsaved changes')).toBeNull()
	expect(within(main).getByRole('button', { name: 'Save' })).toHaveAttribute('aria-disabled', 'true')
	expect(toggleOf(row, 'Summary')).toHaveAttribute('aria-expanded', 'false')
})

test.each([
	{ deleted: 'Subtitle', left: [READING_TIME], focused: 'Delete Reading time' },
	{ deleted: 'Reading time', left: [SUBTITLE], focused: 'Delete Subtitle' },
])('puts the focus on the trash of the field taking the place of $deleted once it is deleted', async (asked) => {
	server.use(
		http.delete('/api/groups/3/fields/:key', () => {
			listing([{ ...DETAILS, fields: asked.left }, EXTRAS])
			return new HttpResponse(null, { status: 204 })
		}),
	)
	renderAt('/field-groups')
	const main = await openFields()

	await userEvent.click(within(main).getByRole('button', { name: `Delete ${asked.deleted}` }))
	const warning = await screen.findByRole('alertdialog', { name: `Delete "${asked.deleted}"?` })
	await userEvent.click(within(warning).getByRole('button', { name: 'Delete the field' }))

	await waitFor(() => expect(within(main).getByRole('button', { name: asked.focused })).toHaveFocus())
})

test('puts the focus on the Fields region once its last field is deleted', async () => {
	listing([{ ...DETAILS, fields: [SUBTITLE] }, EXTRAS])
	server.use(
		http.delete('/api/groups/3/fields/subtitle', () => {
			listing([{ ...DETAILS, fields: [] }, EXTRAS])
			return new HttpResponse(null, { status: 204 })
		}),
	)
	renderAt('/field-groups')
	const main = await openFields()

	await userEvent.click(within(main).getByRole('button', { name: 'Delete Subtitle' }))
	const warning = await screen.findByRole('alertdialog', { name: 'Delete "Subtitle"?' })
	await userEvent.click(within(warning).getByRole('button', { name: 'Delete the field' }))

	await waitFor(() => expect(fieldsOf(main)).toHaveFocus())
	expect(within(fieldsOf(main)).getByRole('heading', { name: NO_FIELDS })).toBeInTheDocument()
})

test('leaves the focus where the reader took it while a deleted field was on its way out', async () => {
	const reread = gate()
	server.use(
		http.delete('/api/groups/3/fields/subtitle', () => {
			server.use(
				http.get('/api/groups', async () => {
					await reread.held
					return HttpResponse.json({ items: [{ ...DETAILS, fields: [READING_TIME] }, EXTRAS] })
				}),
			)
			return new HttpResponse(null, { status: 204 })
		}),
	)
	renderAt('/field-groups')
	const main = await openFields()
	await userEvent.click(within(main).getByRole('button', { name: 'Delete Subtitle' }))
	const warning = await screen.findByRole('alertdialog', { name: 'Delete "Subtitle"?' })
	await userEvent.click(within(warning).getByRole('button', { name: 'Delete the field' }))
	await waitFor(() => expect(screen.queryByRole('alertdialog')).toBeNull())
	const adding = within(main).getAllByRole('textbox', { name: 'Label' }).at(-1) as HTMLElement
	act(() => adding.focus())

	reread.release()

	await waitFor(() => expect(within(fieldsOf(main)).queryByText('Subtitle')).toBeNull())
	expect(adding).toHaveFocus()
})

test('stores a new order with no toast, then reads the groups again', async () => {
	const reads = countedReads()
	server.use(http.put('/api/groups/3/fields/order', () => HttpResponse.json({ items: [READING_TIME, SUBTITLE] })))
	renderAt('/field-groups')
	const main = await openFields()
	const before = reads.count

	await userEvent.click(within(await fieldRow(main, 'Subtitle')).getByRole('button', { name: 'Move Subtitle down' }))

	await waitFor(() => expect(reads.count).toBeGreaterThan(before))
	await act(async () => {})
	expect(screen.queryByText('Order stored.')).toBeNull()
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
	const reads = countedReads()
	server.use(
		http.put('/api/groups/3/fields/order', async () => {
			await answer.held
			return HttpResponse.json({ items: [READING_TIME, SUBTITLE] })
		}),
	)
	renderAt('/field-groups')
	const main = await openFields()
	const before = reads.count

	await userEvent.click(within(await fieldRow(main, 'Subtitle')).getByRole('button', { name: 'Move Subtitle down' }))

	expect(standsAbove(await fieldRow(main, 'Reading time'), await fieldRow(main, 'Subtitle'))).toBe(true)
	expect(reads.count).toBe(before)
	answer.release()
	await waitFor(() => expect(reads.count).toBeGreaterThan(before))
})

test('sends one order at a time and stores the newest once the order on its way is answered', async () => {
	const answer = gate()
	const reads = countedReads()
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
	const before = reads.count

	await userEvent.click(within(await fieldRow(main, 'Subtitle')).getByRole('button', { name: 'Move Subtitle down' }))
	await waitFor(() => expect(sent).toHaveLength(1))
	await userEvent.click(within(await fieldRow(main, 'Subtitle')).getByRole('button', { name: 'Move Subtitle up' }))
	expect(sent).toHaveLength(1)
	answer.release()

	await waitFor(() =>
		expect(sent).toEqual([{ order: ['reading-time', 'subtitle'] }, { order: ['subtitle', 'reading-time'] }]),
	)
	await waitFor(() => expect(reads.count).toBeGreaterThan(before))
})

test('keeps the order a field was moved into while the groups are read again before it is answered', async () => {
	const answer = gate()
	const reads = countedReads()
	const summary = { ...SUBTITLE, key: 'summary', label: 'Summary' }
	server.use(
		http.put('/api/groups/3/fields/order', async () => {
			await answer.held
			return HttpResponse.json({ items: [READING_TIME, SUBTITLE, summary] })
		}),
	)
	const client = renderAt('/field-groups')
	const main = await openFields()
	await userEvent.click(within(await fieldRow(main, 'Subtitle')).getByRole('button', { name: 'Move Subtitle down' }))

	reads.groups = [{ ...DETAILS, fields: [SUBTITLE, READING_TIME, summary] }, EXTRAS]
	await act(async () => {
		await client.invalidateQueries({ queryKey: groupsQueryKey })
	})
	const before = reads.count

	expect(standsAbove(await fieldRow(main, 'Summary'), await fieldRow(main, 'Subtitle'))).toBe(false)
	expect(standsAbove(await fieldRow(main, 'Reading time'), await fieldRow(main, 'Subtitle'))).toBe(true)
	answer.release()
	await waitFor(() => expect(reads.count).toBeGreaterThan(before))
})

test('puts a field back in its stored place when the order it asked for is rejected', async () => {
	server.use(http.put('/api/groups/3/fields/order', () => HttpResponse.json(ORDER_REJECTED, { status: 422 })))
	renderAt('/field-groups')
	const main = await openFields()

	await userEvent.click(within(await fieldRow(main, 'Subtitle')).getByRole('button', { name: 'Move Subtitle down' }))

	expect(await within(main).findByRole('alert')).toHaveTextContent(/every field/)
	expect(standsAbove(await fieldRow(main, 'Subtitle'), await fieldRow(main, 'Reading time'))).toBe(true)
})

test('keeps the order the server holds and reads nothing again when an order never arrives', async () => {
	const reads = countedReads()
	server.use(http.put('/api/groups/3/fields/order', () => HttpResponse.error()))
	renderAt('/field-groups')
	const main = await openFields()
	const before = reads.count

	await userEvent.click(within(await fieldRow(main, 'Subtitle')).getByRole('button', { name: 'Move Subtitle down' }))

	expect(await within(main).findByRole('alert')).toHaveTextContent('The field groups could not be reached.')
	expect(standsAbove(await fieldRow(main, 'Subtitle'), await fieldRow(main, 'Reading time'))).toBe(true)
	await act(async () => {})
	expect(reads.count).toBe(before)
})

test('shows the order the first store landed and reads nothing again when the next order never arrives', async () => {
	const answer = gate()
	const reads = countedReads()
	let orders = 0
	server.use(
		http.put('/api/groups/3/fields/order', async () => {
			orders += 1
			if (orders > 1) {
				return HttpResponse.error()
			}
			await answer.held
			reads.groups = [{ ...DETAILS, fields: [READING_TIME, SUBTITLE] }, EXTRAS]
			return HttpResponse.json({ items: [READING_TIME, SUBTITLE] })
		}),
	)
	renderAt('/field-groups')
	const main = await openFields()
	const before = reads.count

	await userEvent.click(within(await fieldRow(main, 'Subtitle')).getByRole('button', { name: 'Move Subtitle down' }))
	await userEvent.click(within(await fieldRow(main, 'Subtitle')).getByRole('button', { name: 'Move Subtitle up' }))
	answer.release()

	expect(await within(main).findByRole('alert')).toHaveTextContent('The field groups could not be reached.')
	expect(standsAbove(await fieldRow(main, 'Reading time'), await fieldRow(main, 'Subtitle'))).toBe(true)
	await act(async () => {})
	expect(reads.count).toBe(before)
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
