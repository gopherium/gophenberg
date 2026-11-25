// SPDX-License-Identifier: Apache-2.0

import { http, HttpResponse, server } from '@gophenberg/frontend-sdk/testing'
import { screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { expect, test } from 'vitest'

import { gate } from './gate'
import {
	DETAILS,
	EXTRAS,
	FEATURES,
	fieldRow,
	fieldsOf,
	HERO,
	listing,
	openField,
	openFields,
	partOf,
	saveOf,
	serveFieldGroups,
	showTab,
	SUBTITLE,
	toggleOf,
} from './groupFieldsFixtures'
import { renderAt } from './render'
import { expectReadOnly, waitBusy } from './typeFixtures'

const NAME = { ...SUBTITLE, key: 'name', label: 'Name' }

const BIO = { ...SUBTITLE, key: 'bio', label: 'Bio' }

const AUTHOR = { ...SUBTITLE, key: 'author', label: 'Author', kind: 'section', fields: [NAME] }

const EMPTY_AUTHOR = { ...AUTHOR, fields: [] }

serveFieldGroups()

/**
 * Returns the region the sub fields of an open container scroll in.
 * @param part - The row holding the container's open part.
 * @param label - The label of the container.
 * @returns The region.
 */
function subFieldsOf(part: HTMLElement, label: string) {
	return within(part).getByRole('region', { name: `Sub fields of ${label}` })
}

test('shows no sub field row while their container is closed, and a table of them in a region once open', async () => {
	listing([{ ...DETAILS, fields: [AUTHOR] }, EXTRAS])
	renderAt('/field-groups')
	const main = await openFields()
	expect(within(fieldsOf(main)).queryByRole('button', { name: 'Name' })).toBeNull()
	expect(within(fieldsOf(main)).queryByRole('region')).toBeNull()

	const part = await openField(main, 'Author')

	const held = subFieldsOf(part, 'Author')
	expect(held).toHaveClass('godmin-table-scroll')
	expect(held).toHaveAttribute('tabindex', '0')
	const headers = within(held).getAllByRole('columnheader')
	expect(headers.map((header) => header.textContent)).toEqual(['Label', 'Kind', 'API name', 'Actions'])
	expect(within(await fieldRow(main, 'Name')).getByText('name').tagName).toBe('CODE')
	expect(held).toContainElement(await fieldRow(main, 'Name'))
	const form = within(part).getAllByRole('textbox', { name: 'Label' })[0].closest('form') as HTMLFormElement
	expect(form).not.toContainElement(held)
	expect(form.compareDocumentPosition(held) & Node.DOCUMENT_POSITION_FOLLOWING).toBeTruthy()
})

test('puts a container its tabs in its form, then its sub fields, Add field and Close field', async () => {
	listing([{ ...DETAILS, fields: [SUBTITLE, AUTHOR] }, EXTRAS])
	renderAt('/field-groups')
	const main = await openFields()
	const part = await openField(main, 'Author')
	await userEvent.type(within(part).getAllByRole('textbox', { name: 'Label' })[0], ' line')

	const form = within(part).getAllByRole('textbox', { name: 'Label' })[0].closest('form') as HTMLFormElement
	const shown = [
		form,
		subFieldsOf(part, 'Author'),
		within(part).getByRole('button', { name: 'Add field to Author' }),
		within(part).getByRole('button', { name: 'Close field' }),
		within(part).getByRole('button', { name: 'Discard changes' }),
	]

	expect(form).toContainElement(within(part).getByRole('tab', { name: 'Conditional logic' }))
	const following = shown.slice(1).map((node, at) => shown[at].compareDocumentPosition(node))
	expect(following.map((position) => (position & Node.DOCUMENT_POSITION_FOLLOWING) !== 0)).toEqual([
		true,
		true,
		true,
		true,
	])
})

test('declares a sub field inside the container it was opened from', async () => {
	let sent: unknown
	let where = ''
	listing([{ ...DETAILS, fields: [EMPTY_AUTHOR] }, EXTRAS])
	server.use(
		http.post('/api/groups/3/fields/author', async ({ request }) => {
			sent = await request.json()
			where = new URL(request.url).pathname
			return HttpResponse.json(NAME, { status: 201 })
		}),
	)
	renderAt('/field-groups')
	const main = await openFields()
	const part = await openField(main, 'Author')
	expect(within(part).queryByRole('region')).toBeNull()

	await userEvent.click(within(part).getByRole('button', { name: 'Add field to Author' }))
	const adding = await screen.findByRole('dialog', { name: 'Add field to Author' })
	await userEvent.type(within(adding).getByLabelText('Label'), 'Full name')
	await userEvent.click(within(adding).getByRole('button', { name: 'Add field' }))

	await waitFor(() => expect(sent).toMatchObject({ key: 'full-name', label: 'Full name' }))
	expect(where).toBe('/api/groups/3/fields/author')
})

test('declares a field inside a container standing inside another', async () => {
	let where = ''
	const address = { ...SUBTITLE, key: 'address', label: 'Address', kind: 'section', fields: [] }
	listing([{ ...DETAILS, fields: [{ ...AUTHOR, fields: [address] }] }, EXTRAS])
	server.use(
		http.post('/api/groups/3/fields/author.address', ({ request }) => {
			where = new URL(request.url).pathname
			return HttpResponse.json({ ...SUBTITLE, key: 'street', label: 'Street' }, { status: 201 })
		}),
	)
	renderAt('/field-groups')
	const main = await openFields()
	await openField(main, 'Author')
	const part = await openField(main, 'Address')

	await userEvent.click(within(part).getByRole('button', { name: 'Add field to Address' }))
	const adding = await screen.findByRole('dialog', { name: 'Add field to Address' })
	await userEvent.type(within(adding).getByLabelText('Label'), 'Street')
	await userEvent.click(within(adding).getByRole('button', { name: 'Add field' }))

	await waitFor(() => expect(where).toBe('/api/groups/3/fields/author.address'))
})

test('opens a field inside a container in place and saves it on its dotted path from the header Save', async () => {
	const sent: unknown[] = []
	let where = ''
	listing([{ ...DETAILS, fields: [AUTHOR] }, EXTRAS])
	server.use(
		http.patch('/api/groups/3/fields/author.name', async ({ request }) => {
			where = new URL(request.url).pathname
			sent.push(await request.json())
			return HttpResponse.json({ ...NAME, required: true })
		}),
	)
	renderAt('/field-groups')
	const main = await openFields()
	await openField(main, 'Author')
	const part = await openField(main, 'Name')

	await userEvent.clear(within(part).getByRole('textbox', { name: 'Label' }))
	await userEvent.type(within(part).getByRole('textbox', { name: 'Label' }), 'Full name')
	await showTab(part, 'Validation')
	await userEvent.click(within(part).getByRole('checkbox', { name: 'Required' }))
	await userEvent.click(saveOf(main))

	await waitFor(() => expect(where).toBe('/api/groups/3/fields/author.name'))
	expect(sent).toEqual([{ label: 'Full name', required: true, settings: {}, updated_at: '2026-08-01T10:00:00Z' }])
})

test('moves the stale draft of a sub field onto its saved version, marking the label both changed', async () => {
	const theirs = { ...NAME, label: 'Given name', updated_at: '2026-08-02T09:00:00Z' }
	listing([{ ...DETAILS, fields: [AUTHOR] }, EXTRAS])
	server.use(
		http.patch('/api/groups/3/fields/author.name', () => {
			listing([{ ...DETAILS, fields: [{ ...AUTHOR, fields: [theirs] }] }, EXTRAS])
			return HttpResponse.json({ error: 'content: conflicting update', code: 'content_stale_update' }, { status: 409 })
		}),
	)
	renderAt('/field-groups')
	const main = await openFields()
	await openField(main, 'Author')
	const part = await openField(main, 'Name')
	await userEvent.type(within(part).getByRole('textbox', { name: 'Label' }), ' line')
	await userEvent.click(saveOf(main))

	await userEvent.click(await within(part).findByRole('button', { name: 'Load saved version' }))

	const label = within(part).getByRole('textbox', { name: 'Label' })
	await waitFor(() => expect(label).toHaveValue('Given name'))
	expect(label).toHaveAccessibleDescription('Changed elsewhere too. Your edit was not kept.')
	expect(within(part).queryByRole('alert')).toBeNull()
	expect(saveOf(main)).toHaveAttribute('aria-disabled', 'true')
})

test('marks a container whose sub field holds unsaved changes once both are closed', async () => {
	listing([{ ...DETAILS, fields: [AUTHOR] }, EXTRAS])
	renderAt('/field-groups')
	const main = await openFields()
	const author = await openField(main, 'Author')
	const part = await openField(main, 'Name')
	await userEvent.type(within(part).getByRole('textbox', { name: 'Label' }), 's')

	await userEvent.click(within(part).getByRole('button', { name: 'Close field' }))
	await userEvent.click(within(author).getByRole('button', { name: 'Close field' }))

	const row = await fieldRow(main, 'Author')
	expect(toggleOf(row, 'Author')).toHaveAttribute('aria-expanded', 'false')
	expect(within((row as HTMLTableRowElement).cells[0]).getByText('Unsaved changes')).toBeInTheDocument()
})

test('stands the fields inside a container in a new order at once', async () => {
	let where = ''
	let sent: unknown
	listing([{ ...DETAILS, fields: [{ ...AUTHOR, fields: [NAME, BIO] }] }, EXTRAS])
	server.use(
		http.put('/api/groups/3/inside/author/order', async ({ request }) => {
			where = new URL(request.url).pathname
			sent = await request.json()
			return HttpResponse.json({ items: [] })
		}),
	)
	renderAt('/field-groups')
	const main = await openFields()
	const part = await openField(main, 'Author')

	await userEvent.click(within(subFieldsOf(part, 'Author')).getByRole('button', { name: 'Move Bio up' }))

	await waitFor(() => expect(where).toBe('/api/groups/3/inside/author/order'))
	expect(sent).toMatchObject({ order: ['bio', 'name'] })
	expect(within(subFieldsOf(part, 'Author')).getAllByRole('button', { name: /^Delete / })[0]).toHaveAccessibleName(
		'Delete Bio',
	)
})

test('marks a required field inside a container as required', async () => {
	listing([{ ...DETAILS, fields: [{ ...AUTHOR, fields: [{ ...NAME, required: true }] }] }, EXTRAS])
	renderAt('/field-groups')
	const main = await openFields()

	await openField(main, 'Author')

	const [held] = (await fieldRow(main, 'Name') as HTMLTableRowElement).cells
	expect(within(held).getByText('Required')).toBeInTheDocument()
})

test('stands a field inside a container further down', async () => {
	let sent: unknown
	listing([{ ...DETAILS, fields: [{ ...AUTHOR, fields: [NAME, BIO] }] }, EXTRAS])
	server.use(
		http.put('/api/groups/3/inside/author/order', async ({ request }) => {
			sent = await request.json()
			return HttpResponse.json({ items: [] })
		}),
	)
	renderAt('/field-groups')
	const main = await openFields()
	const part = await openField(main, 'Author')

	await userEvent.click(within(part).getByRole('button', { name: 'Move Name down' }))

	await waitFor(() => expect(sent).toMatchObject({ order: ['bio', 'name'] }))
})

test('reports a rejected order inside a container', async () => {
	listing([{ ...DETAILS, fields: [{ ...AUTHOR, fields: [NAME, BIO] }] }, EXTRAS])
	server.use(
		http.put('/api/groups/3/inside/author/order', () =>
			HttpResponse.json(
				{ error: 'content: the order leaves declared fields out', code: 'field_order_incomplete' },
				{ status: 422 },
			),
		),
	)
	renderAt('/field-groups')
	const main = await openFields()
	const part = await openField(main, 'Author')

	await userEvent.click(within(part).getByRole('button', { name: 'Move Bio up' }))

	expect(await within(main).findByRole('alert')).toHaveTextContent(/field/i)
})

test('removes a sub field from the container that holds it', async () => {
	let hit = ''
	listing([{ ...DETAILS, fields: [AUTHOR] }, EXTRAS])
	server.use(
		http.delete('/api/groups/3/inside/author.name', ({ request }) => {
			hit = new URL(request.url).pathname
			return new HttpResponse(null, { status: 204 })
		}),
	)
	renderAt('/field-groups')
	const main = await openFields()
	const part = await openField(main, 'Author')

	await userEvent.click(within(part).getByRole('button', { name: 'Delete Name' }))
	const warning = await screen.findByRole('alertdialog', { name: 'Delete "Name"?' })
	await userEvent.click(within(warning).getByRole('button', { name: 'Delete the field' }))

	await waitFor(() => expect(hit).toBe('/api/groups/3/inside/author.name'))
})

test('puts the focus on the trash of the sub field taking the place of a deleted one', async () => {
	listing([{ ...DETAILS, fields: [{ ...AUTHOR, fields: [NAME, BIO] }] }, EXTRAS])
	server.use(
		http.delete('/api/groups/3/inside/author.name', () => {
			listing([{ ...DETAILS, fields: [{ ...AUTHOR, fields: [BIO] }] }, EXTRAS])
			return new HttpResponse(null, { status: 204 })
		}),
	)
	renderAt('/field-groups')
	const main = await openFields()
	const part = await openField(main, 'Author')

	await userEvent.click(within(part).getByRole('button', { name: 'Delete Name' }))
	const warning = await screen.findByRole('alertdialog', { name: 'Delete "Name"?' })
	await userEvent.click(within(warning).getByRole('button', { name: 'Delete the field' }))

	await waitFor(() => expect(within(part).getByRole('button', { name: 'Delete Bio' })).toHaveFocus())
})

test('puts the focus on the label of a container once its last sub field is deleted', async () => {
	listing([{ ...DETAILS, fields: [AUTHOR] }, EXTRAS])
	server.use(
		http.delete('/api/groups/3/inside/author.name', () => {
			listing([{ ...DETAILS, fields: [EMPTY_AUTHOR] }, EXTRAS])
			return new HttpResponse(null, { status: 204 })
		}),
	)
	renderAt('/field-groups')
	const main = await openFields()
	const part = await openField(main, 'Author')

	await userEvent.click(within(part).getByRole('button', { name: 'Delete Name' }))
	const warning = await screen.findByRole('alertdialog', { name: 'Delete "Name"?' })
	await userEvent.click(within(warning).getByRole('button', { name: 'Delete the field' }))

	await waitFor(() => expect(within(part).queryByRole('region')).toBeNull())
	expect(toggleOf(await fieldRow(main, 'Author'), 'Author')).toHaveFocus()
})

test('declares a layout inside the flexible content field it was opened from', async () => {
	let sent: unknown
	listing([{ ...DETAILS, fields: [FEATURES] }, EXTRAS])
	server.use(
		http.post('/api/groups/3/fields/features', async ({ request }) => {
			sent = await request.json()
			return HttpResponse.json(HERO, { status: 201 })
		}),
	)
	renderAt('/field-groups')
	const main = await openFields()
	const part = await openField(main, 'Features')

	await userEvent.click(within(part).getByRole('button', { name: 'Add layout to Features' }))
	const adding = await screen.findByRole('dialog', { name: 'Add layout to Features' })
	await userEvent.type(within(adding).getByLabelText('Label'), 'Hero')
	await userEvent.click(within(adding).getByRole('button', { name: 'Add layout' }))

	await waitFor(() => expect(sent).toMatchObject({ key: 'hero', label: 'Hero', kind: 'layout' }))
})

test('lists a flexible content field with the layouts and fields beneath it, each level opening in place', async () => {
	const title = { ...SUBTITLE, key: 'title', label: 'Title' }
	listing([{ ...DETAILS, fields: [{ ...FEATURES, fields: [{ ...HERO, fields: [title] }] }] }, EXTRAS])
	renderAt('/field-groups')
	const main = await openFields()

	const flexible = await openField(main, 'Features')
	expect(within(subFieldsOf(flexible, 'Features')).getByRole('button', { name: 'Hero' })).toBeInTheDocument()
	const layout = await openField(main, 'Hero')

	expect(within(await fieldRow(main, 'Features')).getByText('Flexible content')).toBeInTheDocument()
	expect(within(await fieldRow(main, 'Hero')).getByText('Layout')).toBeInTheDocument()
	expect(subFieldsOf(layout, 'Hero')).toContainElement(await fieldRow(main, 'Title'))
	expect(partOf(await fieldRow(main, 'Title'))).toHaveAttribute('hidden', 'until-found')
})

test('declares a field inside a layout of a flexible content field', async () => {
	let where = ''
	listing([{ ...DETAILS, fields: [{ ...FEATURES, fields: [HERO] }] }, EXTRAS])
	server.use(
		http.post('/api/groups/3/fields/features.hero', ({ request }) => {
			where = new URL(request.url).pathname
			return HttpResponse.json({ ...SUBTITLE, key: 'title', label: 'Title' }, { status: 201 })
		}),
	)
	renderAt('/field-groups')
	const main = await openFields()
	await openField(main, 'Features')
	const part = await openField(main, 'Hero')

	await userEvent.click(within(part).getByRole('button', { name: 'Add field to Hero' }))
	const adding = await screen.findByRole('dialog', { name: 'Add field to Hero' })
	await userEvent.type(within(adding).getByLabelText('Label'), 'Title')
	await userEvent.click(within(adding).getByRole('button', { name: 'Add field' }))

	await waitFor(() => expect(where).toBe('/api/groups/3/fields/features.hero'))
})

test('says inside its own dialog why a field declared into a container was rejected', async () => {
	listing([{ ...DETAILS, fields: [EMPTY_AUTHOR] }, EXTRAS])
	server.use(
		http.post('/api/groups/3/fields/author', () =>
			HttpResponse.json({ error: 'content: field key taken', code: 'field_taken' }, { status: 422 }),
		),
	)
	renderAt('/field-groups')
	const main = await openFields()
	const part = await openField(main, 'Author')

	await userEvent.click(within(part).getByRole('button', { name: 'Add field to Author' }))
	const adding = await screen.findByRole('dialog', { name: 'Add field to Author' })
	await userEvent.type(within(adding).getByLabelText('Label'), 'Full name')
	await userEvent.click(within(adding).getByRole('button', { name: 'Add field' }))

	const said = 'A field with that name already exists on the items this group reaches. Pick another name.'
	expect(await within(adding).findByText(said)).toBeInTheDocument()
	expect(screen.getAllByText(said)).toHaveLength(1)
	expect(within(adding).getByLabelText('Label')).toHaveValue('Full name')
})

test('keeps the controls of a field declared into a container read only while it is declared', async () => {
	const answer = gate()
	listing([{ ...DETAILS, fields: [EMPTY_AUTHOR] }, EXTRAS])
	server.use(
		http.post('/api/groups/3/fields/author', async () => {
			await answer.held
			return HttpResponse.json(NAME, { status: 201 })
		}),
	)
	renderAt('/field-groups')
	const main = await openFields()
	const part = await openField(main, 'Author')

	await userEvent.click(within(part).getByRole('button', { name: 'Add field to Author' }))
	const adding = await screen.findByRole('dialog', { name: 'Add field to Author' })
	await userEvent.type(within(adding).getByLabelText('Label'), 'Full name')
	await userEvent.click(within(adding).getByRole('button', { name: 'Add field' }))
	await waitBusy(adding, 'Add field')

	expectReadOnly(adding)
	answer.release()
	await waitFor(() => expect(screen.queryByRole('dialog', { name: 'Add field to Author' })).not.toBeInTheDocument())
})
