// SPDX-License-Identifier: Apache-2.0

import { http, HttpResponse, server } from '@gophenberg/frontend-sdk/testing'
import { screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { beforeAll, beforeEach, expect, test } from 'vitest'

import { renderAt } from './render'
import { storedPost } from './postFixture'

const EDITOR_PATH = `/content/post/${storedPost.id}/edit`

const TYPE_WITH_FIELDS = {
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
	created_at: '2026-08-01T10:00:00Z',
	updated_at: '2026-08-01T10:00:00Z',
	fields: [
		{ key: 'color', label: 'Color', kind: 'text', many: false, required: false },
		{ key: 'doors', label: 'Doors', kind: 'number', many: false, required: false },
		{ key: 'boxed', label: 'Boxed', kind: 'boolean', many: false, required: false },
	],
}

beforeAll(async () => {
	await import('../content/EditorScreen')
}, 120000)

beforeEach(() => {
	server.use(
		http.get('/api/types', () => HttpResponse.json({ items: [TYPE_WITH_FIELDS] })),
		http.get(`/api/content/${storedPost.id}`, () =>
			HttpResponse.json({ ...storedPost, fields: { color: 'red' } }),
		),
	)
})

test('shows a control for every scalar field the type declares', async () => {
	renderAt(EDITOR_PATH)

	expect(await screen.findByLabelText('Color')).toBeInTheDocument()
	expect(screen.getByLabelText('Doors')).toBeInTheDocument()
	expect(screen.getByLabelText('Boxed')).toBeInTheDocument()
})

test('shows the value the post holds', async () => {
	renderAt(EDITOR_PATH)

	expect(await screen.findByLabelText('Color')).toHaveValue('red')
})

test('shows no panel when the type declares no fields', async () => {
	server.use(
		http.get('/api/types', () =>
			HttpResponse.json({ items: [{ ...TYPE_WITH_FIELDS, fields: [] }] }),
		),
	)
	renderAt(EDITOR_PATH)
	await screen.findByRole('tab', { name: 'Document' })

	expect(screen.queryByLabelText('Color')).not.toBeInTheDocument()
})

test('sends the edited values when the post is saved', async () => {
	const sent: unknown[] = []
	server.use(
		http.patch(`/api/content/${storedPost.id}`, async ({ request }) => {
			sent.push(await request.json())
			return HttpResponse.json({ ...storedPost, fields: { color: 'blue' } })
		}),
	)
	renderAt(EDITOR_PATH)
	const control = await screen.findByLabelText('Color')

	await userEvent.clear(control)
	await userEvent.type(control, 'blue')
	await userEvent.click(screen.getByRole('button', { name: 'Save draft' }))

	await waitFor(() => expect(sent).toHaveLength(1))
	expect(sent[0]).toMatchObject({ fields: { color: 'blue' } })
})

test('keeps the stored values when only the title moved', async () => {
	const sent: unknown[] = []
	server.use(
		http.patch(`/api/content/${storedPost.id}`, async ({ request }) => {
			sent.push(await request.json())
			return HttpResponse.json({ ...storedPost, fields: { color: 'red' } })
		}),
	)
	renderAt(EDITOR_PATH)
	await screen.findByLabelText('Color')

	await userEvent.type(screen.getByRole('textbox', { name: 'Title' }), ' again')
	await userEvent.click(screen.getByRole('button', { name: 'Save draft' }))

	await waitFor(() => expect(sent).toHaveLength(1))
	expect(sent[0]).toMatchObject({ fields: { color: 'red' } })
})

test('clears a number field when its control is emptied', async () => {
	const sent: Record<string, unknown>[] = []
	server.use(
		http.get(`/api/content/${storedPost.id}`, () =>
			HttpResponse.json({ ...storedPost, fields: { doors: 4 } }),
		),
		http.patch(`/api/content/${storedPost.id}`, async ({ request }) => {
			sent.push((await request.json()) as Record<string, unknown>)
			return HttpResponse.json({ ...storedPost, fields: {} })
		}),
	)
	renderAt(EDITOR_PATH)
	const control = await screen.findByLabelText('Doors')

	await userEvent.clear(control)
	await userEvent.click(screen.getByRole('button', { name: 'Save draft' }))

	await waitFor(() => expect(sent).toHaveLength(1))
	expect(sent[0].fields).toEqual({ doors: null })
})
