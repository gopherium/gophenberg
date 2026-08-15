// SPDX-License-Identifier: Apache-2.0

import { http, HttpResponse, server } from '@gophenberg/frontend-sdk/testing'
import { act, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { afterEach, beforeAll, beforeEach, expect, test, vi } from 'vitest'

import { renderAt } from './render'
import { storedPost } from './postFixture'

const EDITOR_PATH = `/content/post/${storedPost.id}/edit`

const AUTOSAVE_INTERVAL = 60000

const autosaved: Record<string, unknown>[] = []

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
	fields: [{ key: 'color', label: 'Color', kind: 'text', many: false, required: false }],
}

beforeAll(async () => {
	await import('../content/EditorScreen')
}, 120000)

beforeEach(() => {
	autosaved.length = 0
	vi.useFakeTimers({ shouldAdvanceTime: true })
	server.use(
		http.get('/api/types', () => HttpResponse.json({ items: [TYPE_WITH_FIELDS] })),
		http.get(`/api/content/${storedPost.id}`, () =>
			HttpResponse.json({ ...storedPost, fields: { color: 'red' } }),
		),
		http.get(`/api/content/${storedPost.id}/autosave`, () => HttpResponse.json({}, { status: 404 })),
		http.post(`/api/content/${storedPost.id}/autosave`, async ({ request }) => {
			autosaved.push((await request.json()) as Record<string, unknown>)
			return HttpResponse.json({
				target: 'autosave',
				content_id: storedPost.id,
				title: storedPost.title,
				content: storedPost.content,
				excerpt: '',
				fields: { color: 'blue' },
				saved_at: '2026-08-01T12:00:00Z',
			})
		}),
	)
})

afterEach(() => {
	vi.useRealTimers()
})

/**
 * Advances the clock inside a React update.
 * @param ms - The milliseconds to advance.
 */
async function tick(ms: number) {
	await act(async () => {
		await vi.advanceTimersByTimeAsync(ms)
	})
}

test('parks the field values the buffer holds', async () => {
	renderAt(EDITOR_PATH)
	const control = await screen.findByLabelText('Color')

	await userEvent.clear(control)
	await userEvent.type(control, 'blue')
	await tick(AUTOSAVE_INTERVAL)

	await waitFor(() => expect(autosaved).toHaveLength(1))
	expect(autosaved[0]).toMatchObject({ fields: { color: 'blue' } })
})

test('parks a field edit even when no word moved', async () => {
	renderAt(EDITOR_PATH)
	const control = await screen.findByLabelText('Color')

	await userEvent.clear(control)
	await userEvent.type(control, 'green')
	await tick(AUTOSAVE_INTERVAL)

	await waitFor(() => expect(autosaved).toHaveLength(1))
	expect(autosaved[0]).toMatchObject({ title: storedPost.title, fields: { color: 'green' } })
})

test('brings the parked field values back when the buffer is restored', async () => {
	const sent: Record<string, unknown>[] = []
	server.use(
		http.get(`/api/content/${storedPost.id}/autosave`, () =>
			HttpResponse.json({
				target: 'autosave',
				content_id: storedPost.id,
				title: storedPost.title,
				content: storedPost.content,
				excerpt: '',
				fields: { color: 'parked' },
				saved_at: '2026-08-01T12:00:00Z',
			}),
		),
		http.patch(`/api/content/${storedPost.id}`, async ({ request }) => {
			sent.push((await request.json()) as Record<string, unknown>)
			return HttpResponse.json({ ...storedPost, fields: { color: 'parked' } })
		}),
	)
	renderAt(EDITOR_PATH)

	await userEvent.click(await screen.findByRole('button', { name: 'Restore' }))

	await waitFor(() => expect(screen.getByLabelText('Color')).toHaveValue('parked'))
	await userEvent.click(screen.getByRole('button', { name: 'Save draft' }))
	await waitFor(() => expect(sent).toHaveLength(1))
	expect(sent[0]).toMatchObject({ fields: { color: 'parked' } })
})
