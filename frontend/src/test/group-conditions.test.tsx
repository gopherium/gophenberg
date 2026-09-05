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

const ON_SALE = {
	key: 'on-sale',
	label: 'On sale',
	kind: 'boolean',
	many: false,
	required: false,
	created_at: '2026-08-01T10:00:00Z',
	updated_at: '2026-08-01T10:00:00Z',
}

const SALE_NOTE = { ...ON_SALE, key: 'sale-note', label: 'Sale note', kind: 'text' }

const PHOTO = { ...ON_SALE, key: 'photo', label: 'Photo', kind: 'media' }

const DETAILS = {
	id: 3,
	key: 'article-details',
	title: 'Article details',
	location: [[{ source: 'content_type', operator: '==', value: 'post' }]],
	position: 1,
	active: true,
	fields: [ON_SALE, SALE_NOTE, PHOTO],
	created_at: '2026-08-01T10:00:00Z',
	updated_at: '2026-08-01T10:00:00Z',
}

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

/**
 * Opens the conditions dialog of the named field.
 * @param named - The field whose conditions to open.
 * @returns The conditions dialog.
 */
async function openConditions(named: string) {
	const fields = await openFields()
	const row = within(fields).getByRole('listitem', { name: named })
	await userEvent.click(within(row).getByRole('button', { name: `Rules showing ${named}` }))
	return screen.findByRole('dialog', { name: `Rules showing ${named}` })
}

/**
 * Records the body of the next field settings write.
 * @returns The recorder holding what was sent.
 */
function recordingSettings(): { sent: unknown } {
	const held: { sent: unknown } = { sent: undefined }
	server.use(
		http.patch('/api/groups/3/fields/:key', async ({ request }) => {
			held.sent = await request.json()
			return HttpResponse.json(SALE_NOTE)
		}),
	)
	return held
}

beforeEach(() => {
	server.use(http.get('/api/types', () => HttpResponse.json({ items: [POST_TYPE] })))
	server.use(http.get('/api/groups', () => HttpResponse.json({ items: [DETAILS] })))
})

test('offers the siblings a rule may read, leaving the field itself out', async () => {
	renderAt('/field-groups')

	const dialog = await openConditions('Sale note')

	await userEvent.click(within(dialog).getByRole('button', { name: 'Add rule set' }))
	const sources = within(dialog).getByRole('combobox', { name: 'Source' })
	expect(within(sources).getByText('On sale')).toBeInTheDocument()
})

test('offers no rules on a field whose siblings cannot be read', async () => {
	server.use(http.get('/api/groups', () => HttpResponse.json({ items: [{ ...DETAILS, fields: [PHOTO] }] })))
	renderAt('/field-groups')

	const dialog = await openFields()

	const row = within(dialog).getByRole('listitem', { name: 'Photo' })
	expect(within(row).queryByRole('button', { name: 'Rules showing Photo' })).not.toBeInTheDocument()
})

test('stores a rule showing a field while a sibling holds', async () => {
	const held = recordingSettings()
	renderAt('/field-groups')
	const dialog = await openConditions('Sale note')

	await userEvent.click(within(dialog).getByRole('button', { name: 'Add rule set' }))
	await userEvent.click(within(dialog).getByRole('button', { name: 'Save rules' }))

	await waitFor(() =>
		expect(held.sent).toEqual({
			settings: { conditions: [[{ source: 'on-sale', operator: '==', value: 'true' }]] },
			updated_at: '2026-08-01T10:00:00Z',
		}),
	)
})

test('keeps the settings the rules dialog does not touch', async () => {
	const noted = { ...SALE_NOTE, settings: { instructions: 'Fill me in' } }
	server.use(http.get('/api/groups', () => HttpResponse.json({ items: [{ ...DETAILS, fields: [ON_SALE, noted] }] })))
	const held = recordingSettings()
	renderAt('/field-groups')
	const dialog = await openConditions('Sale note')

	await userEvent.click(within(dialog).getByRole('button', { name: 'Add rule set' }))
	await userEvent.click(within(dialog).getByRole('button', { name: 'Save rules' }))

	await waitFor(() =>
		expect(held.sent).toEqual({
			settings: {
				instructions: 'Fill me in',
				conditions: [[{ source: 'on-sale', operator: '==', value: 'true' }]],
			},
			updated_at: '2026-08-01T10:00:00Z',
		}),
	)
})

test('takes a rule row away again', async () => {
	const held = recordingSettings()
	renderAt('/field-groups')
	const dialog = await openConditions('Sale note')

	await userEvent.click(within(dialog).getByRole('button', { name: 'Add rule set' }))
	await userEvent.click(within(dialog).getByRole('button', { name: 'Remove' }))
	await userEvent.click(within(dialog).getByRole('button', { name: 'Save rules' }))

	await waitFor(() => expect(held.sent).toEqual({ settings: {}, updated_at: '2026-08-01T10:00:00Z' }))
})

test('says a field stands on no rules until one is added', async () => {
	renderAt('/field-groups')

	const dialog = await openConditions('Sale note')

	expect(within(dialog).getByText('Without a rule this field always shows.')).toBeInTheDocument()
})

test('says why the rules were turned away', async () => {
	server.use(
		http.patch('/api/groups/3/fields/:key', () =>
			HttpResponse.json({ error: 'loop', code: 'rule_cycle', meta: { field: 'sale-note' } }, { status: 422 }),
		),
	)
	renderAt('/field-groups')
	const dialog = await openConditions('Sale note')

	await userEvent.click(within(dialog).getByRole('button', { name: 'Add rule set' }))
	await userEvent.click(within(dialog).getByRole('button', { name: 'Save rules' }))

	expect(await screen.findByRole('alert')).toHaveTextContent(/sale-note/)
})

test('carries a rule onto another source, starting its condition afresh', async () => {
	const price = { ...ON_SALE, key: 'price', label: 'Price', kind: 'number' }
	server.use(
		http.get('/api/groups', () =>
			HttpResponse.json({ items: [{ ...DETAILS, fields: [ON_SALE, price, SALE_NOTE] }] }),
		),
	)
	const held = recordingSettings()
	renderAt('/field-groups')
	const dialog = await openConditions('Sale note')

	await userEvent.click(within(dialog).getByRole('button', { name: 'Add rule set' }))
	await userEvent.click(within(dialog).getByRole('combobox', { name: 'Source' }))
	await userEvent.click(await screen.findByRole('option', { name: 'Price' }))
	await userEvent.type(within(dialog).getByRole('spinbutton', { name: 'Value' }), '10')
	await userEvent.click(within(dialog).getByRole('button', { name: 'Save rules' }))

	await waitFor(() =>
		expect(held.sent).toEqual({
			settings: { conditions: [[{ source: 'price', operator: '==', value: '10' }]] },
			updated_at: '2026-08-01T10:00:00Z',
		}),
	)
})

test('carries a rule onto another condition and another value', async () => {
	const held = recordingSettings()
	renderAt('/field-groups')
	const dialog = await openConditions('Sale note')

	await userEvent.click(within(dialog).getByRole('button', { name: 'Add rule set' }))
	await userEvent.click(within(dialog).getByRole('combobox', { name: 'Condition' }))
	await userEvent.click(await screen.findByRole('option', { name: 'is not' }))
	await userEvent.click(within(dialog).getByRole('combobox', { name: 'Value' }))
	await userEvent.click(await screen.findByRole('option', { name: 'No' }))
	await userEvent.click(within(dialog).getByRole('button', { name: 'Save rules' }))

	await waitFor(() =>
		expect(held.sent).toEqual({
			settings: { conditions: [[{ source: 'on-sale', operator: '!=', value: 'false' }]] },
			updated_at: '2026-08-01T10:00:00Z',
		}),
	)
})

test('holds a second condition in the same rule set', async () => {
	const kind = { ...ON_SALE, key: 'kind', label: 'Kind', kind: 'choice', settings: { choices: [
		{ value: 'book', label: 'Book' },
	] } }
	server.use(
		http.get('/api/groups', () =>
			HttpResponse.json({ items: [{ ...DETAILS, fields: [ON_SALE, kind, SALE_NOTE] }] }),
		),
	)
	const held = recordingSettings()
	renderAt('/field-groups')
	const dialog = await openConditions('Sale note')

	await userEvent.click(within(dialog).getByRole('button', { name: 'Add rule set' }))
	await userEvent.click(within(dialog).getByRole('button', { name: 'Add condition' }))
	await userEvent.click(within(dialog).getByRole('button', { name: 'Save rules' }))

	await waitFor(() =>
		expect(held.sent).toEqual({
			settings: {
				conditions: [[
					{ source: 'on-sale', operator: '==', value: 'true' },
					{ source: 'on-sale', operator: '==', value: 'true' },
				]],
			},
			updated_at: '2026-08-01T10:00:00Z',
		}),
	)
})

test('offers the choices a choice source holds', async () => {
	const kind = { ...ON_SALE, key: 'kind', label: 'Kind', kind: 'choice', settings: { choices: [
		{ value: 'book', label: 'Book' },
	] } }
	server.use(
		http.get('/api/groups', () => HttpResponse.json({ items: [{ ...DETAILS, fields: [kind, SALE_NOTE] }] })),
	)
	renderAt('/field-groups')
	const dialog = await openConditions('Sale note')

	await userEvent.click(within(dialog).getByRole('button', { name: 'Add rule set' }))

	const values = within(dialog).getByRole('combobox', { name: 'Value' })
	expect(within(values).getByText('Book')).toBeInTheDocument()
})

test('takes a typed value for a source that offers no choices', async () => {
	const held = recordingSettings()
	server.use(
		http.get('/api/groups', () =>
			HttpResponse.json({ items: [{ ...DETAILS, fields: [{ ...ON_SALE, kind: 'text' }, SALE_NOTE] }] }),
		),
	)
	renderAt('/field-groups')
	const dialog = await openConditions('Sale note')

	await userEvent.click(within(dialog).getByRole('button', { name: 'Add rule set' }))
	await userEvent.type(within(dialog).getByRole('textbox', { name: 'Value' }), 'yes')
	await userEvent.click(within(dialog).getByRole('button', { name: 'Save rules' }))

	await waitFor(() =>
		expect(held.sent).toEqual({
			settings: { conditions: [[{ source: 'on-sale', operator: '==', value: 'yes' }]] },
			updated_at: '2026-08-01T10:00:00Z',
		}),
	)
})

test('asks for no value beside an operator that needs none', async () => {
	const held = recordingSettings()
	server.use(
		http.get('/api/groups', () =>
			HttpResponse.json({ items: [{ ...DETAILS, fields: [{ ...ON_SALE, kind: 'text' }, SALE_NOTE] }] }),
		),
	)
	renderAt('/field-groups')
	const dialog = await openConditions('Sale note')

	await userEvent.click(within(dialog).getByRole('button', { name: 'Add rule set' }))
	await userEvent.click(within(dialog).getByRole('combobox', { name: 'Condition' }))
	await userEvent.click(await screen.findByRole('option', { name: 'is empty' }))

	expect(within(dialog).queryByRole('textbox', { name: 'Value' })).not.toBeInTheDocument()
	await userEvent.click(within(dialog).getByRole('button', { name: 'Save rules' }))
	await waitFor(() =>
		expect(held.sent).toEqual({
			settings: { conditions: [[{ source: 'on-sale', operator: 'empty', value: '' }]] },
			updated_at: '2026-08-01T10:00:00Z',
		}),
	)
})

test('edits one rule set without disturbing the other', async () => {
	const held = recordingSettings()
	renderAt('/field-groups')
	const dialog = await openConditions('Sale note')

	await userEvent.click(within(dialog).getByRole('button', { name: 'Add rule set' }))
	await userEvent.click(within(dialog).getByRole('button', { name: 'Add rule set' }))
	const second = within(dialog).getByRole('group', { name: 'Rule set 2' })
	await userEvent.click(within(second).getByRole('button', { name: 'Add condition' }))
	const rows = within(second).getAllByRole('combobox', { name: 'Condition' })
	await userEvent.click(rows[1] as HTMLElement)
	await userEvent.click(await screen.findByRole('option', { name: 'is not' }))
	await userEvent.click(within(dialog).getByRole('button', { name: 'Save rules' }))

	await waitFor(() =>
		expect(held.sent).toEqual({
			settings: {
				conditions: [
					[{ source: 'on-sale', operator: '==', value: 'true' }],
					[
						{ source: 'on-sale', operator: '==', value: 'true' },
						{ source: 'on-sale', operator: '!=', value: 'true' },
					],
				],
			},
			updated_at: '2026-08-01T10:00:00Z',
		}),
	)
})

test('takes one rule set away without disturbing the other', async () => {
	const held = recordingSettings()
	renderAt('/field-groups')
	const dialog = await openConditions('Sale note')

	await userEvent.click(within(dialog).getByRole('button', { name: 'Add rule set' }))
	await userEvent.click(within(dialog).getByRole('button', { name: 'Add rule set' }))
	const first = within(dialog).getByRole('group', { name: 'Rule set 1' })
	await userEvent.click(within(first).getByRole('button', { name: 'Remove' }))
	await userEvent.click(within(dialog).getByRole('button', { name: 'Save rules' }))

	await waitFor(() =>
		expect(held.sent).toEqual({
			settings: { conditions: [[{ source: 'on-sale', operator: '==', value: 'true' }]] },
			updated_at: '2026-08-01T10:00:00Z',
		}),
	)
})

test('leaves the stored rules alone when the dialog is cancelled', async () => {
	renderAt('/field-groups')
	const dialog = await openConditions('Sale note')

	await userEvent.click(within(dialog).getByRole('button', { name: 'Add rule set' }))
	await userEvent.click(within(dialog).getByRole('button', { name: 'Cancel' }))

	expect(screen.queryByRole('dialog', { name: 'Rules showing Sale note' })).not.toBeInTheDocument()
})

test('falls back to a sibling it can read when a stored rule names one it cannot', async () => {
	const noted = {
		...SALE_NOTE,
		settings: { conditions: [[{ source: 'vanished', operator: '~', value: 'x' }]] },
	}
	server.use(
		http.get('/api/groups', () => HttpResponse.json({ items: [{ ...DETAILS, fields: [ON_SALE, noted] }] })),
	)
	renderAt('/field-groups')

	const dialog = await openConditions('Sale note')

	const sources = within(dialog).getByRole('combobox', { name: 'Source' })
	expect(within(sources).getByText('vanished')).toBeInTheDocument()
	const operators = within(dialog).getByRole('combobox', { name: 'Condition' })
	expect(within(operators).getByText('~')).toBeInTheDocument()
})

test('stores the rules of a field standing inside a container', async () => {
	const paid = { ...ON_SALE, key: 'paid', label: 'Paid' }
	const fee = { ...SALE_NOTE, key: 'fee', label: 'Fee' }
	const crew = { ...ON_SALE, key: 'crew', label: 'Crew', kind: 'section', fields: [paid, fee] }
	server.use(http.get('/api/groups', () => HttpResponse.json({ items: [{ ...DETAILS, fields: [crew] }] })))
	const held = recordingSettings()
	renderAt('/field-groups')
	const fields = await openFields()

	const row = within(fields).getByRole('listitem', { name: 'Fee' })
	await userEvent.click(within(row).getByRole('button', { name: 'Rules showing Fee' }))
	const dialog = await screen.findByRole('dialog', { name: 'Rules showing Fee' })
	await userEvent.click(within(dialog).getByRole('button', { name: 'Add rule set' }))
	await userEvent.click(within(dialog).getByRole('button', { name: 'Save rules' }))

	await waitFor(() =>
		expect(held.sent).toEqual({
			settings: { conditions: [[{ source: 'paid', operator: '==', value: 'true' }]] },
			updated_at: '2026-08-01T10:00:00Z',
		}),
	)
})

test('puts a field in the admin list and takes it out again', async () => {
	const held = recordingSettings()
	renderAt('/field-groups')
	const dialog = await openFields()

	const row = within(dialog).getByRole('listitem', { name: 'Sale note' })
	await userEvent.click(within(row).getByRole('button', { name: 'Show Sale note in the list' }))

	await waitFor(() =>
		expect(held.sent).toEqual({ settings: { listed: true }, updated_at: '2026-08-01T10:00:00Z' }),
	)
})

test('takes a field out of the admin list again', async () => {
	const listed = { ...SALE_NOTE, settings: { listed: true } }
	server.use(http.get('/api/groups', () => HttpResponse.json({ items: [{ ...DETAILS, fields: [listed] }] })))
	const held = recordingSettings()
	renderAt('/field-groups')
	const dialog = await openFields()

	const row = within(dialog).getByRole('listitem', { name: 'Sale note' })
	await userEvent.click(within(row).getByRole('button', { name: 'Keep Sale note out of the list' }))

	await waitFor(() =>
		expect(held.sent).toEqual({ settings: { listed: false }, updated_at: '2026-08-01T10:00:00Z' }),
	)
})

test('offers no list flip inside a container, where the list never reaches', async () => {
	const paid = { ...ON_SALE, key: 'paid', label: 'Paid' }
	const crew = { ...ON_SALE, key: 'crew', label: 'Crew', kind: 'section', fields: [paid] }
	server.use(http.get('/api/groups', () => HttpResponse.json({ items: [{ ...DETAILS, fields: [crew] }] })))
	renderAt('/field-groups')

	const dialog = await openFields()

	const row = within(dialog).getByRole('listitem', { name: 'Paid' })
	expect(within(row).queryByRole('button', { name: /in the list/ })).not.toBeInTheDocument()
})

test('offers no list flip on a kind the list cannot show', async () => {
	renderAt('/field-groups')

	const dialog = await openFields()

	const row = within(dialog).getByRole('listitem', { name: 'Photo' })
	expect(within(row).queryByRole('button', { name: /in the list/ })).not.toBeInTheDocument()
})
