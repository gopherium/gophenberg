// SPDX-License-Identifier: Apache-2.0

import { http, HttpResponse, server } from '@gophenberg/frontend-sdk/testing'
import { screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { beforeAll, beforeEach, expect, test } from 'vitest'

import { renderAt } from './render'
import { storedPost } from './postFixture'

const EDITOR_PATH = `/content/post/${storedPost.id}/edit`

const SOURCE = {
	key: 'source',
	label: 'Source',
	kind: 'link',
	relates_to: '',
	many: false,
	required: false,
	updated_at: '2026-08-01T10:00:00Z',
}

/**
 * Returns the post type declaring the given fields.
 * @param fields - The field definitions the type declares.
 * @returns The type as the registry answers it.
 */
function typeDeclaring(fields: unknown[]) {
	return {
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
		fields,
	}
}

/**
 * Serves the type declaring the fields and a stored post holding the values, recording what is saved.
 * @param fields - The field definitions the type declares.
 * @param values - The values the stored post holds.
 * @returns The bodies the editor sent back.
 */
function editing(fields: unknown[], values: Record<string, unknown>) {
	const sent: Record<string, unknown>[] = []
	server.use(
		http.get('/api/types', () => HttpResponse.json({ items: [typeDeclaring(fields)] })),
		http.get(`/api/content/${storedPost.id}`, () => HttpResponse.json({ ...storedPost, fields: values })),
		http.patch(`/api/content/${storedPost.id}`, async ({ request }) => {
			sent.push((await request.json()) as Record<string, unknown>)
			return HttpResponse.json({ ...storedPost, fields: values })
		}),
	)
	return sent
}

beforeAll(async () => {
	await import('../content/EditorScreen')
}, 120000)

beforeEach(() => {
	server.use(http.get('/api/content', () => HttpResponse.json({ items: [], total: 0 })))
})

test('shows the three parts a link holds', async () => {
	editing([SOURCE], { source: { url: 'https://example.com/a', title: 'A page', new_tab: true } })
	renderAt(EDITOR_PATH)

	expect(await screen.findByLabelText('Source address')).toHaveValue('https://example.com/a')
	expect(screen.getByLabelText('Source title')).toHaveValue('A page')
	expect(screen.getByLabelText('Open Source in a new tab')).toBeChecked()
})

test('sends a link back whole once any part of it is written', async () => {
	const sent = editing([SOURCE], {})
	renderAt(EDITOR_PATH)

	await userEvent.type(await screen.findByLabelText('Source address'), '/about')
	await userEvent.type(screen.getByLabelText('Source title'), 'About')
	await userEvent.click(screen.getByLabelText('Open Source in a new tab'))
	await userEvent.click(screen.getByRole('button', { name: 'Save draft' }))

	await waitFor(() => expect(sent).toHaveLength(1))
	expect(sent[0]).toMatchObject({ fields: { source: { url: '/about', title: 'About', new_tab: true } } })
})

test('clears a link whose address is wiped', async () => {
	const sent = editing([SOURCE], { source: { url: '/about', title: 'About', new_tab: false } })
	renderAt(EDITOR_PATH)

	await userEvent.clear(await screen.findByLabelText('Source address'))
	await userEvent.clear(screen.getByLabelText('Source title'))
	await userEvent.click(screen.getByRole('button', { name: 'Save draft' }))

	await waitFor(() => expect(sent).toHaveLength(1))
	expect(sent[0]).toMatchObject({ fields: { source: null } })
})

test('edits a link standing inside a repeater row', async () => {
	const sent = editing(
		[{ ...SOURCE, key: 'sources', label: 'Sources', kind: 'repeater', fields: [SOURCE] }],
		{ sources: [{ source: { url: '/a', title: 'A', new_tab: false } }] },
	)
	renderAt(EDITOR_PATH)

	await userEvent.type(await screen.findByLabelText('Source title'), ' page')
	await userEvent.click(screen.getByRole('button', { name: 'Save draft' }))

	await waitFor(() => expect(sent).toHaveLength(1))
	expect(sent[0]).toMatchObject({
		fields: { sources: [{ source: { url: '/a', title: 'A page', new_tab: false } }] },
	})
})
