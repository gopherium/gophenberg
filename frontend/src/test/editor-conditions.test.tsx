// SPDX-License-Identifier: Apache-2.0

import { http, HttpResponse, server } from '@gophenberg/frontend-sdk/testing'
import { screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { beforeAll, beforeEach, expect, test } from 'vitest'

import { renderAt } from './render'
import { storedPost } from './postFixture'

const EDITOR_PATH = `/content/post/${storedPost.id}/edit`

const STAMP = '2026-08-01T10:00:00Z'

const ON_SALE = { key: 'on-sale', label: 'On sale', kind: 'boolean', many: false, required: false, updated_at: STAMP }

const SALE_NOTE = {
	key: 'sale-note',
	label: 'Sale note',
	kind: 'text',
	many: false,
	required: false,
	updated_at: STAMP,
	settings: { conditions: [[{ source: 'on-sale', operator: '==', value: 'true' }]] },
}

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
	created_at: STAMP,
	updated_at: STAMP,
	fields: [ON_SALE, SALE_NOTE],
}

/**
 * Serves the type declaring the given fields.
 * @param fields - The fields the type declares.
 */
function declaring(fields: Record<string, unknown>[]) {
	server.use(http.get('/api/types', () => HttpResponse.json({ items: [{ ...POST_TYPE, fields }] })))
}

/**
 * Serves the stored post holding the given values.
 * @param fields - The values the item holds.
 */
function holding(fields: Record<string, unknown>) {
	server.use(http.get(`/api/content/${storedPost.id}`, () => HttpResponse.json({ ...storedPost, fields })))
}

/**
 * Records the body of every save the editor sends.
 * @returns The bodies sent so far.
 */
function recordingSaves(): unknown[] {
	const sent: unknown[] = []
	server.use(
		http.patch(`/api/content/${storedPost.id}`, async ({ request }) => {
			sent.push(await request.json())
			return HttpResponse.json(storedPost)
		}),
	)
	return sent
}

beforeAll(async () => {
	await import('../content/EditorScreen')
}, 120000)

beforeEach(() => {
	declaring([ON_SALE, SALE_NOTE])
	holding({ 'on-sale': true, 'sale-note': 'half price' })
})

test('shows a field while the rules its sibling answers hold', async () => {
	renderAt(EDITOR_PATH)

	expect(await screen.findByLabelText('Sale note')).toBeInTheDocument()
})

test('hides a field once the rules stop holding', async () => {
	holding({ 'on-sale': false, 'sale-note': 'half price' })
	renderAt(EDITOR_PATH)
	await screen.findByLabelText('On sale')

	expect(screen.queryByLabelText('Sale note')).not.toBeInTheDocument()
})

test('hides a field the moment its source is switched off', async () => {
	renderAt(EDITOR_PATH)
	const flip = await screen.findByLabelText('On sale')

	await userEvent.click(flip)

	await waitFor(() => expect(screen.queryByLabelText('Sale note')).not.toBeInTheDocument())
})

test('leaves a value it just hid out of what it saves', async () => {
	const sent = recordingSaves()
	renderAt(EDITOR_PATH)

	await userEvent.type(await screen.findByLabelText('Sale note'), ' today')
	await userEvent.click(screen.getByLabelText('On sale'))
	await userEvent.click(screen.getByRole('button', { name: 'Save draft' }))

	await waitFor(() => expect(sent).toHaveLength(1))
	expect((sent[0] as { fields: Record<string, unknown> }).fields).toEqual({ 'on-sale': false })
})

test('leaves a value it just hid out of what it publishes', async () => {
	const sent = recordingSaves()
	renderAt(EDITOR_PATH)

	await userEvent.type(await screen.findByLabelText('Sale note'), ' today')
	await userEvent.click(screen.getByLabelText('On sale'))
	await userEvent.click(screen.getByRole('button', { name: 'Publish' }))

	await waitFor(() => expect(sent).toHaveLength(1))
	expect((sent[0] as { fields: Record<string, unknown> }).fields).toEqual({ 'on-sale': false })
})

test('sends a value that is still shown', async () => {
	const sent = recordingSaves()
	renderAt(EDITOR_PATH)

	await userEvent.type(await screen.findByLabelText('Sale note'), ' today')
	await userEvent.click(screen.getByRole('button', { name: 'Save draft' }))

	await waitFor(() => expect(sent).toHaveLength(1))
	expect((sent[0] as { fields: Record<string, unknown> }).fields).toEqual({ 'sale-note': 'half price today' })
})

test('hides every field a hidden container holds', async () => {
	const crew = {
		key: 'crew',
		label: 'Crew',
		kind: 'section',
		many: false,
		required: false,
		updated_at: STAMP,
		settings: { conditions: [[{ source: 'on-sale', operator: '==', value: 'true' }]] },
		fields: [{ key: 'fee', label: 'Fee', kind: 'text', many: false, required: false, updated_at: STAMP }],
	}
	declaring([ON_SALE, crew])
	holding({ 'on-sale': false, crew: { fee: 'ten' } })
	renderAt(EDITOR_PATH)
	await screen.findByLabelText('On sale')

	expect(screen.queryByLabelText('Fee')).not.toBeInTheDocument()
})

test('judges a field inside a row against that row alone', async () => {
	const crew = {
		key: 'crew',
		label: 'Crew',
		kind: 'repeater',
		many: false,
		required: false,
		updated_at: STAMP,
		fields: [
			{ key: 'paid', label: 'Paid', kind: 'boolean', many: false, required: false, updated_at: STAMP },
			{
				key: 'fee',
				label: 'Fee',
				kind: 'text',
				many: false,
				required: false,
				updated_at: STAMP,
				settings: { conditions: [[{ source: 'paid', operator: '==', value: 'true' }]] },
			},
		],
	}
	declaring([crew])
	holding({ crew: [{ paid: true, fee: 'ten' }, { paid: false, fee: 'twenty' }] })
	renderAt(EDITOR_PATH)

	expect(await screen.findAllByLabelText('Paid')).toHaveLength(2)
	expect(screen.getAllByLabelText('Fee')).toHaveLength(1)
})

test('sends a hidden value a row still carries, because a row travels whole', async () => {
	const crew = {
		key: 'crew',
		label: 'Crew',
		kind: 'repeater',
		many: false,
		required: false,
		updated_at: STAMP,
		fields: [
			{ key: 'paid', label: 'Paid', kind: 'boolean', many: false, required: false, updated_at: STAMP },
			{
				key: 'fee',
				label: 'Fee',
				kind: 'text',
				many: false,
				required: false,
				updated_at: STAMP,
				settings: { conditions: [[{ source: 'paid', operator: '==', value: 'true' }]] },
			},
		],
	}
	declaring([crew])
	holding({ crew: [{ paid: true, fee: 'ten' }] })
	const sent = recordingSaves()
	renderAt(EDITOR_PATH)

	await userEvent.click(await screen.findByLabelText('Paid'))
	await userEvent.click(screen.getByRole('button', { name: 'Save draft' }))

	await waitFor(() => expect(sent).toHaveLength(1))
	expect((sent[0] as { fields: Record<string, unknown> }).fields).toEqual({
		crew: [{ paid: false, fee: 'ten' }],
	})
})

test('shows no panel at all when every field it holds is hidden', async () => {
	declaring([SALE_NOTE])
	holding({})
	renderAt(EDITOR_PATH)

	await screen.findByRole('tab', { name: 'Document' })
	expect(screen.queryByLabelText('Sale note')).not.toBeInTheDocument()
})
