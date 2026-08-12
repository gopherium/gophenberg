// SPDX-License-Identifier: Apache-2.0

import { http, HttpResponse, server } from '@gophenberg/frontend-sdk/testing'
import { act, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { afterEach, beforeAll, beforeEach, expect, test, vi } from 'vitest'

import { renderAt } from './render'
import { storedPost } from './postFixture'

const EDITOR_PATH = `/posts/${storedPost.id}/edit`

const WRITTEN_AT = '2026-07-28T11:00:00Z'

const patched: Record<string, unknown>[] = []

const parked: Record<string, unknown>[] = []

beforeAll(async () => {
	await import('../content/EditorScreen')
}, 120000)

beforeEach(() => {
	patched.length = 0
	parked.length = 0
	vi.useFakeTimers({ shouldAdvanceTime: true })
	server.use(
		http.get(`/api/content/${storedPost.id}`, () => HttpResponse.json(storedPost)),
		http.get(`/api/content/${storedPost.id}/autosave`, () => HttpResponse.json({}, { status: 404 })),
		http.patch(`/api/content/${storedPost.id}`, async ({ request }) => {
			const body = (await request.json()) as Record<string, unknown>
			patched.push(body)
			return HttpResponse.json({ ...storedPost, ...body, updated_at: WRITTEN_AT })
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

/**
 * Answers autosaves with the given target and saved time.
 * @param target - Where the server parked the buffer.
 * @param savedAt - The time the server reported.
 */
function autosaveLands(target: string, savedAt: string) {
	server.use(
		http.post(`/api/content/${storedPost.id}/autosave`, async ({ request }) => {
			parked.push((await request.json()) as Record<string, unknown>)
			return HttpResponse.json({
				target,
				content_id: storedPost.id,
				title: 'Welcome to Gophenberg!',
				content: storedPost.content,
				excerpt: '',
				saved_at: savedAt,
			})
		}),
	)
}

/**
 * Types into the title and saves the post.
 */
async function editAndSave() {
	await userEvent.type(await screen.findByRole('textbox', { name: 'Title' }), '!')
	await userEvent.click(screen.getByRole('button', { name: 'Save draft' }))
}

test('states the version it loaded with', async () => {
	renderAt(EDITOR_PATH)

	await editAndSave()

	await waitFor(() => expect(patched).toHaveLength(1))
	expect(patched[0]).toMatchObject({ updated_at: storedPost.updated_at })
})

test('states the version the server wrote back', async () => {
	renderAt(EDITOR_PATH)
	await editAndSave()
	await waitFor(() => expect(patched).toHaveLength(1))

	await editAndSave()

	await waitFor(() => expect(patched).toHaveLength(2))
	expect(patched[1]).toMatchObject({ updated_at: WRITTEN_AT })
})

test('states its version when parking a buffer', async () => {
	autosaveLands('autosave', WRITTEN_AT)
	renderAt(EDITOR_PATH)
	await userEvent.type(await screen.findByRole('textbox', { name: 'Title' }), '!')

	await tick(60000)

	await waitFor(() => expect(parked).toHaveLength(1))
	expect(parked[0]).toMatchObject({ updated_at: storedPost.updated_at })
})

test('takes the version of an autosave that reached the post', async () => {
	autosaveLands('post', WRITTEN_AT)
	renderAt(EDITOR_PATH)
	await userEvent.type(await screen.findByRole('textbox', { name: 'Title' }), '!')
	await tick(60000)

	await userEvent.click(screen.getByRole('button', { name: 'Save draft' }))

	await waitFor(() => expect(patched).toHaveLength(1))
	expect(patched[0]).toMatchObject({ updated_at: WRITTEN_AT })
})

test('keeps its version when an autosave was only parked', async () => {
	autosaveLands('autosave', WRITTEN_AT)
	renderAt(EDITOR_PATH)
	await userEvent.type(await screen.findByRole('textbox', { name: 'Title' }), '!')
	await tick(60000)

	await userEvent.click(screen.getByRole('button', { name: 'Save draft' }))

	await waitFor(() => expect(patched).toHaveLength(1))
	expect(patched[0]).toMatchObject({ updated_at: storedPost.updated_at })
})

test('states the version when publishing, not only when saving', async () => {
	renderAt(EDITOR_PATH)
	await screen.findByRole('textbox', { name: 'Title' })

	await userEvent.click(screen.getByRole('button', { name: 'Publish' }))

	await waitFor(() => expect(patched).toHaveLength(1))
	expect(patched[0]).toMatchObject({ status: 'published', updated_at: storedPost.updated_at })
})
