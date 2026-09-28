// SPDX-License-Identifier: Apache-2.0

import { http, HttpResponse, server } from '@gophenberg/frontend-sdk/testing'
import { act, cleanup, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { afterEach, beforeAll, beforeEach, expect, test, vi } from 'vitest'

import { collectParks } from './parks'
import type { Park } from './parks'
import { renderAt } from './render'
import { storedPost } from './postFixture'

const EDITOR_PATH = `/content/post/${storedPost.id}/edit`

let parks: Park[] = []

beforeAll(async () => {
	await import('../content/EditorScreen')
}, 120000)

beforeEach(() => {
	vi.useFakeTimers({ shouldAdvanceTime: true })
	server.use(http.get(`/api/content/${storedPost.id}`, () => HttpResponse.json(storedPost)))
	parks = collectParks()
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

	expect(parks).toEqual([])
})

test('saves a dirty post once its interval comes round', async () => {
	renderAt(EDITOR_PATH)
	await userEvent.type(await screen.findByRole('textbox', { name: 'Title' }), '!')

	await tick(60000)

	await waitFor(() => expect(parks).toHaveLength(1))
	expect(parks[0].body).toMatchObject({ title: 'Welcome to Gophenberg!' })
	expect(parks[0].keepalive).toBe(false)
})

test('stops its timer once the editor has closed', async () => {
	renderAt(EDITOR_PATH)
	await userEvent.type(await screen.findByRole('textbox', { name: 'Title' }), '!')
	cleanup()
	await waitFor(() => expect(parks).toHaveLength(1))

	await tick(120000)

	expect(parks).toHaveLength(1)
})

test('holds off until the interval comes round', async () => {
	renderAt(EDITOR_PATH)
	await userEvent.type(await screen.findByRole('textbox', { name: 'Title' }), '!')

	await tick(59000)

	expect(parks).toEqual([])
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

	expect(parks).toEqual([])
})

test('flushes the words when the page is leaving', async () => {
	renderAt(EDITOR_PATH)
	await userEvent.type(await screen.findByRole('textbox', { name: 'Title' }), '!')

	await act(async () => {
		window.dispatchEvent(new Event('beforeunload'))
		await Promise.resolve()
	})

	await waitFor(() => expect(parks).toHaveLength(1))
	expect(parks[0].body).toMatchObject({ title: 'Welcome to Gophenberg!' })
	expect(parks[0].keepalive).toBe(true)
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
	await waitFor(() => expect(parks).toHaveLength(1))
	await tick(60000)

	await waitFor(() => expect(parks).toHaveLength(2))
})
