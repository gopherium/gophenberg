// SPDX-License-Identifier: Apache-2.0

import { http, HttpResponse, server } from '@gophenberg/frontend-sdk/testing'
import { act, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { afterEach, beforeAll, beforeEach, expect, test, vi } from 'vitest'

import { renderAt } from './render'
import { storedPost } from './postFixture'

const EDITOR_PATH = `/content/post/${storedPost.id}/edit`

const autosaved: Record<string, unknown>[] = []

beforeAll(async () => {
	await import('../content/EditorScreen')
}, 120000)

beforeEach(() => {
	autosaved.length = 0
	vi.useFakeTimers({ shouldAdvanceTime: true })
	server.use(
		http.get(`/api/content/${storedPost.id}`, () => HttpResponse.json(storedPost)),
		http.get(`/api/content/${storedPost.id}/autosave`, () => HttpResponse.json({}, { status: 404 })),
		http.post(`/api/content/${storedPost.id}/autosave`, async ({ request }) => {
			autosaved.push((await request.json()) as Record<string, unknown>)
			return HttpResponse.json({
				target: 'autosave',
				content_id: storedPost.id,
				title: storedPost.title,
				content: storedPost.content,
				excerpt: '',
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

test('leaves an untouched post alone', async () => {
	renderAt(EDITOR_PATH)
	await screen.findByRole('textbox', { name: 'Title' })

	await tick(60000)

	expect(autosaved).toEqual([])
})

test('saves a dirty post once its interval comes round', async () => {
	renderAt(EDITOR_PATH)
	await userEvent.type(await screen.findByRole('textbox', { name: 'Title' }), '!')

	await tick(60000)

	await waitFor(() => expect(autosaved).toHaveLength(1))
	expect(autosaved[0]).toMatchObject({ title: 'Welcome to Gophenberg!' })
})

test('holds off until the interval comes round', async () => {
	renderAt(EDITOR_PATH)
	await userEvent.type(await screen.findByRole('textbox', { name: 'Title' }), '!')

	await tick(59000)

	expect(autosaved).toEqual([])
})

test('stops saving a post that was written by hand', async () => {
	server.use(
		http.patch(`/api/content/${storedPost.id}`, () =>
			HttpResponse.json({ ...storedPost, title: 'Welcome to Gophenberg!' }),
		),
	)
	renderAt(EDITOR_PATH)
	await userEvent.type(await screen.findByRole('textbox', { name: 'Title' }), '!')
	await userEvent.click(screen.getByRole('button', { name: 'Save draft' }))
	await screen.findByText('Draft saved.')

	await tick(60000)

	expect(autosaved).toEqual([])
})

test('flushes the words when the page is leaving', async () => {
	renderAt(EDITOR_PATH)
	await userEvent.type(await screen.findByRole('textbox', { name: 'Title' }), '!')

	await act(async () => {
		window.dispatchEvent(new Event('beforeunload'))
		await Promise.resolve()
	})

	await waitFor(() => expect(autosaved).toHaveLength(1))
	expect(autosaved[0]).toMatchObject({ title: 'Welcome to Gophenberg!' })
})

test('leaves the editor standing when the server refuses an autosave', async () => {
	vi.spyOn(console, 'error').mockImplementation(() => {})
	server.use(
		http.post(`/api/content/${storedPost.id}/autosave`, () =>
			HttpResponse.json({}, { status: 500 }),
		),
	)
	renderAt(EDITOR_PATH)
	await userEvent.type(await screen.findByRole('textbox', { name: 'Title' }), '!')

	await tick(60000)

	expect(screen.getByRole('textbox', { name: 'Title' })).toHaveValue('Welcome to Gophenberg!')
})

test('keeps saving while the post stays dirty', async () => {
	renderAt(EDITOR_PATH)
	await userEvent.type(await screen.findByRole('textbox', { name: 'Title' }), '!')

	await tick(60000)
	await waitFor(() => expect(autosaved).toHaveLength(1))
	await tick(60000)

	await waitFor(() => expect(autosaved).toHaveLength(2))
})
