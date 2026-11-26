// SPDX-License-Identifier: Apache-2.0

import { http, HttpResponse, server } from '@gophenberg/frontend-sdk/testing'
import { screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { expect, test } from 'vitest'

import { gate } from './gate'
import {
	addFormOf,
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
import { siteSettings } from './siteSettings'
import { expectReadOnly, findToast, waitBusy } from './typeFixtures'

/** A time limit far shorter than any test. */
const SHORT_LIMIT = 20

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

/**
 * Returns the region the layouts of an open flexible content field scroll in.
 * @param part - The row holding the flexible content field's open part.
 * @param label - The label of the flexible content field.
 * @returns The region.
 */
function layoutsOf(part: HTMLElement, label: string) {
	return within(part).getByRole('region', { name: `Layouts of ${label}` })
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

test('puts a container its tabs in its form, then its sub fields, add form, Close field and Discard', async () => {
	listing([{ ...DETAILS, fields: [SUBTITLE, AUTHOR] }, EXTRAS])
	renderAt('/field-groups')
	const main = await openFields()
	const part = await openField(main, 'Author')
	await userEvent.type(within(part).getAllByRole('textbox', { name: 'Label' })[0], ' line')

	const form = within(part).getAllByRole('textbox', { name: 'Label' })[0].closest('form') as HTMLFormElement
	const adding = addFormOf(part, 'Author')
	const shown = [
		form,
		subFieldsOf(part, 'Author'),
		adding,
		within(part).getByRole('button', { name: 'Close field' }),
		within(part).getByRole('button', { name: 'Discard changes' }),
	]

	expect(form).toContainElement(within(part).getByRole('tab', { name: 'Conditional logic' }))
	expect(adding).toBeInstanceOf(HTMLFormElement)
	expect(adding.parentElement?.closest('form')).toBeNull()
	const following = shown.slice(1).map((node, at) => shown[at].compareDocumentPosition(node))
	expect(following.map((position) => (position & Node.DOCUMENT_POSITION_FOLLOWING) !== 0)).toEqual([
		true,
		true,
		true,
		true,
	])
})

test('declares a sub field from the form inside the container it was opened from, with no dialog', async () => {
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

	const adding = addFormOf(part, 'Author')
	await userEvent.type(within(adding).getByRole('textbox', { name: 'Label' }), 'Full name')
	await userEvent.click(within(adding).getByRole('button', { name: 'Add field to Author' }))

	await waitFor(() => expect(sent).toMatchObject({ key: 'full-name', label: 'Full name' }))
	expect(where).toBe('/api/groups/3/fields/author')
	expect(screen.queryByRole('dialog')).toBeNull()
})

test('clears the add form of a container once its field lands, the new row closed and its button focused', async () => {
	listing([{ ...DETAILS, fields: [EMPTY_AUTHOR] }, EXTRAS])
	server.use(
		http.post('/api/groups/3/fields/author', () => {
			listing([{ ...DETAILS, fields: [AUTHOR] }, EXTRAS])
			return HttpResponse.json(NAME, { status: 201 })
		}),
	)
	renderAt('/field-groups')
	const main = await openFields()
	const part = await openField(main, 'Author')
	const adding = addFormOf(part, 'Author')
	const add = within(adding).getByRole('button', { name: 'Add field to Author' })

	await userEvent.type(within(adding).getByRole('textbox', { name: 'Label' }), 'Name')
	await userEvent.click(add)

	expect(await findToast('Name declared.')).toBeInTheDocument()
	const row = await fieldRow(main, 'Name')
	expect(subFieldsOf(part, 'Author')).toContainElement(row)
	expect(toggleOf(row, 'Name')).toHaveAttribute('aria-expanded', 'false')
	expect(toggleOf(await fieldRow(main, 'Author'), 'Author')).toHaveAttribute('aria-expanded', 'true')
	expect(within(adding).getByRole('textbox', { name: 'Label' })).toHaveValue('')
	expect(add).toHaveFocus()
})

test('heads the sub fields of an open container and, under them, the form adding one', async () => {
	listing([{ ...DETAILS, fields: [AUTHOR] }, EXTRAS])
	renderAt('/field-groups')
	const main = await openFields()
	const part = await openField(main, 'Author')

	const titles = within(part).getAllByRole('heading')
	expect(titles.map((title) => [title.tagName, title.textContent])).toEqual([
		['H3', 'Sub fields'],
		['H4', 'Add a field'],
	])
	expect(titles[0].parentElement).toContainElement(subFieldsOf(part, 'Author'))
	expect(titles[1].parentElement).toContainElement(addFormOf(part, 'Author'))
	expect(within(main).getAllByRole('heading', { level: 3, name: 'Add a field' })).toHaveLength(1)
})

test('says an open container holds no sub fields yet, under its Sub fields title and over its add form', async () => {
	listing([{ ...DETAILS, fields: [EMPTY_AUTHOR] }, EXTRAS])
	renderAt('/field-groups')
	const main = await openFields()
	const part = await openField(main, 'Author')

	const empty = within(part).getByText('No sub fields yet.')
	const shown = [within(part).getByRole('heading', { level: 3, name: 'Sub fields' }), empty, addFormOf(part, 'Author')]
	expect(within(part).queryByRole('region')).toBeNull()
	expect(shown[0].compareDocumentPosition(shown[1]) & Node.DOCUMENT_POSITION_FOLLOWING).toBeTruthy()
	expect(shown[1].compareDocumentPosition(shown[2]) & Node.DOCUMENT_POSITION_FOLLOWING).toBeTruthy()
})

test('names the add button of each open container after it, apart from the add button of the group', async () => {
	const address = { ...SUBTITLE, key: 'address', label: 'Address', kind: 'section', fields: [] }
	listing([{ ...DETAILS, fields: [{ ...AUTHOR, fields: [address] }] }, EXTRAS])
	renderAt('/field-groups')
	const main = await openFields()
	await openField(main, 'Author')
	await openField(main, 'Address')

	const adds = within(main).getAllByRole('button', { name: /^Add field/ })

	expect(adds.map((add) => add.textContent)).toEqual(['Add field', 'Add field', 'Add field'])
	expect(adds[0]).toHaveAccessibleName('Add field to Address')
	expect(adds[1]).toHaveAccessibleName('Add field to Author')
	expect(adds[2]).toHaveAccessibleName('Add field')
})

test('names the add form of each open container after it, leaving the form of the group unnamed', async () => {
	listing([{ ...DETAILS, fields: [AUTHOR, FEATURES] }, EXTRAS])
	renderAt('/field-groups')
	const main = await openFields()
	await openField(main, 'Author')
	await openField(main, 'Features')

	const named = within(main).getAllByRole('form', { name: /^Add / })

	expect(named.map((form) => form.getAttribute('aria-label'))).toEqual(['Add field to Author', 'Add layout to Features'])
	const group = within(main).getByRole('button', { name: 'Add field' }).closest('form')
	expect(group).toBeInstanceOf(HTMLFormElement)
	expect(group).not.toHaveAttribute('aria-label')
})

test('keeps the draft of a container and the header Save as they were once a field lands inside it', async () => {
	listing([{ ...DETAILS, fields: [EMPTY_AUTHOR] }, EXTRAS])
	server.use(
		http.post('/api/groups/3/fields/author', () => {
			listing([{ ...DETAILS, fields: [AUTHOR] }, EXTRAS])
			return HttpResponse.json(NAME, { status: 201 })
		}),
	)
	renderAt('/field-groups')
	const main = await openFields()
	const part = await openField(main, 'Author')
	const label = within(part).getAllByRole('textbox', { name: 'Label' })[0]
	await userEvent.type(label, ' line')
	const adding = addFormOf(part, 'Author')

	await userEvent.type(within(adding).getByRole('textbox', { name: 'Label' }), 'Name')
	await userEvent.click(within(adding).getByRole('button', { name: 'Add field to Author' }))

	expect(await findToast('Name declared.')).toBeInTheDocument()
	expect(subFieldsOf(part, 'Author')).toContainElement(await fieldRow(main, 'Name'))
	expect(label).toHaveValue('Author line')
	expect(saveOf(main)).not.toHaveAttribute('aria-disabled', 'true')
	const [held] = ((await fieldRow(main, 'Author')) as HTMLTableRowElement).cells
	expect(within(held).getByText('Unsaved changes')).toBeInTheDocument()
})

test('shows no sub fields part and no add form inside an open field that holds no fields', async () => {
	listing([{ ...DETAILS, fields: [SUBTITLE, AUTHOR] }, EXTRAS])
	renderAt('/field-groups')
	const main = await openFields()

	const part = await openField(main, 'Subtitle')

	expect(within(part).queryByRole('heading')).toBeNull()
	expect(within(part).queryByRole('button', { name: /^Add field/ })).toBeNull()
	expect(within(part).queryByText('No sub fields yet.')).toBeNull()
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

	const adding = addFormOf(part, 'Address')
	await userEvent.type(within(adding).getByRole('textbox', { name: 'Label' }), 'Street')
	await userEvent.click(within(adding).getByRole('button', { name: 'Add field to Address' }))

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

	const adding = addFormOf(part, 'Features', 'Add layout')
	await userEvent.type(within(adding).getByRole('textbox', { name: 'Label' }), 'Hero')
	await userEvent.click(within(adding).getByRole('button', { name: 'Add layout to Features' }))

	await waitFor(() => expect(sent).toMatchObject({ key: 'hero', label: 'Hero', kind: 'layout' }))
	expect(screen.queryByRole('dialog')).toBeNull()
})

test('heads the layouts of an open flexible content field and, under them, the form adding a layout', async () => {
	listing([{ ...DETAILS, fields: [{ ...FEATURES, fields: [HERO] }] }, EXTRAS])
	renderAt('/field-groups')
	const main = await openFields()
	const part = await openField(main, 'Features')

	const titles = within(part).getAllByRole('heading')
	const add = within(part).getByRole('button', { name: 'Add layout to Features' })

	expect(titles.map((title) => [title.tagName, title.textContent])).toEqual([
		['H3', 'Layouts'],
		['H4', 'Add a layout'],
	])
	expect(titles[0].parentElement).toContainElement(layoutsOf(part, 'Features'))
	expect(titles[1].parentElement).toContainElement(addFormOf(part, 'Features', 'Add layout'))
	expect(within(part).queryByRole('region', { name: 'Sub fields of Features' })).toBeNull()
	expect(add.textContent).toBe('Add layout')
})

test('says a flexible content field holds no layouts yet, between its Layouts title and its add form', async () => {
	listing([{ ...DETAILS, fields: [FEATURES] }, EXTRAS])
	renderAt('/field-groups')
	const main = await openFields()
	const part = await openField(main, 'Features')

	const empty = within(part).getByText('No layouts yet.')
	const shown = [
		within(part).getByRole('heading', { level: 3, name: 'Layouts' }),
		empty,
		addFormOf(part, 'Features', 'Add layout'),
	]
	expect(within(part).queryByText('No sub fields yet.')).toBeNull()
	expect(within(part).queryByRole('region')).toBeNull()
	expect(shown[0].compareDocumentPosition(shown[1]) & Node.DOCUMENT_POSITION_FOLLOWING).toBeTruthy()
	expect(shown[1].compareDocumentPosition(shown[2]) & Node.DOCUMENT_POSITION_FOLLOWING).toBeTruthy()
})

test('opens a layout to its own row limits, then its own sub fields and the form adding a field to it', async () => {
	const title = { ...SUBTITLE, key: 'title', label: 'Title' }
	listing([{ ...DETAILS, fields: [{ ...FEATURES, fields: [{ ...HERO, fields: [title] }] }] }, EXTRAS])
	renderAt('/field-groups')
	const main = await openFields()
	const flexible = await openField(main, 'Features')
	const layout = await openField(main, 'Hero')

	await showTab(layout, 'Validation')

	expect(within(layout).getByRole('spinbutton', { name: 'Fewest rows' })).toBeInTheDocument()
	expect(within(layout).getByRole('spinbutton', { name: 'Most rows' })).toBeInTheDocument()
	expect(within(flexible).getAllByRole('heading').map((heading) => heading.textContent)).toEqual([
		'Layouts',
		'Sub fields',
		'Add a field',
		'Add a layout',
	])
	expect(subFieldsOf(layout, 'Hero')).toContainElement(await fieldRow(main, 'Title'))
	await userEvent.click(within(addFormOf(layout, 'Hero')).getByRole('combobox', { name: 'Kind' }))
	expect(await screen.findByRole('option', { name: 'Text' })).toBeInTheDocument()
	expect(screen.queryByRole('option', { name: 'Layout' })).toBeNull()
})

test('lists a flexible content field with the layouts and fields beneath it, each level opening in place', async () => {
	const title = { ...SUBTITLE, key: 'title', label: 'Title' }
	listing([{ ...DETAILS, fields: [{ ...FEATURES, fields: [{ ...HERO, fields: [title] }] }] }, EXTRAS])
	renderAt('/field-groups')
	const main = await openFields()

	const flexible = await openField(main, 'Features')
	expect(within(layoutsOf(flexible, 'Features')).getByRole('button', { name: 'Hero' })).toBeInTheDocument()
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

	const adding = addFormOf(part, 'Hero')
	await userEvent.type(within(adding).getByRole('textbox', { name: 'Label' }), 'Title')
	await userEvent.click(within(adding).getByRole('button', { name: 'Add field to Hero' }))

	await waitFor(() => expect(where).toBe('/api/groups/3/fields/features.hero'))
})

test('reports a field declared into a container that was rejected and keeps what its form holds', async () => {
	listing([{ ...DETAILS, fields: [EMPTY_AUTHOR] }, EXTRAS])
	server.use(
		http.post('/api/groups/3/fields/author', () =>
			HttpResponse.json({ error: 'content: field key taken', code: 'field_taken' }, { status: 422 }),
		),
	)
	renderAt('/field-groups')
	const main = await openFields()
	const part = await openField(main, 'Author')

	const adding = addFormOf(part, 'Author')
	await userEvent.type(within(adding).getByRole('textbox', { name: 'Label' }), 'Full name')
	await userEvent.click(within(adding).getByRole('button', { name: 'Add field to Author' }))

	const said = 'A field with that name already exists on the items this group reaches. Pick another name.'
	expect(await within(main).findByRole('alert')).toHaveTextContent(said)
	expect(await findToast(`Could not add the field. ${said}`)).toBeInTheDocument()
	expect(within(adding).getByRole('textbox', { name: 'Label' })).toHaveValue('Full name')
})

test("stops a field declared into a container past the site's time limit, keeping what its form holds", async () => {
	listing([{ ...DETAILS, fields: [EMPTY_AUTHOR] }, EXTRAS])
	server.use(
		http.get('/api/settings', () =>
			HttpResponse.json({ ...siteSettings, dialog_write_timeout_milliseconds: SHORT_LIMIT }),
		),
		http.post('/api/groups/3/fields/author', () => new Promise<never>(() => {})),
	)
	renderAt('/field-groups')
	const main = await openFields()
	const part = await openField(main, 'Author')

	const adding = addFormOf(part, 'Author')
	await userEvent.type(within(adding).getByRole('textbox', { name: 'Label' }), 'Full name')
	await userEvent.click(within(adding).getByRole('button', { name: 'Add field to Author' }))

	expect(await findToast('Could not add the field. The field groups could not be reached.')).toBeInTheDocument()
	expect(within(adding).getByRole('textbox', { name: 'Label' })).toHaveValue('Full name')
	expect(within(adding).getByRole('textbox', { name: 'Label' })).not.toHaveAttribute('readonly')
})

test('keeps the controls of the form inside a container read only while its field is declared', async () => {
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

	const adding = addFormOf(part, 'Author')
	await userEvent.type(within(adding).getByRole('textbox', { name: 'Label' }), 'Full name')
	await userEvent.click(within(adding).getByRole('button', { name: 'Add field to Author' }))
	await waitBusy(adding, 'Add field to Author')

	expectReadOnly(adding)
	answer.release()
	await waitFor(() => expect(within(adding).getByRole('textbox', { name: 'Label' })).toHaveValue(''))
	expect(within(adding).getByRole('textbox', { name: 'Label' })).not.toHaveAttribute('readonly')
})
