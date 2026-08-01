// SPDX-License-Identifier: Apache-2.0

import { http, HttpResponse, server } from '@gophenberg/frontend-sdk/testing'
import { screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { beforeAll, beforeEach, expect, test, vi } from 'vitest'

import { renderAt } from './render'
import { storedPost } from './postFixture'

const EDITOR_PATH = `/posts/${storedPost.id}/edit`

const patched: Record<string, unknown>[] = []

beforeAll(async () => {
	await import('../posts/editorRoute.lazy')
}, 120000)

/**
 * Serves the given post and records every write against it.
 * @param post - The post the server holds.
 */
function serve(post: Record<string, unknown>) {
	server.use(
		http.get(`/api/posts/${storedPost.id}`, () => HttpResponse.json(post)),
		http.patch(`/api/posts/${storedPost.id}`, async ({ request }) => {
			const body = (await request.json()) as Record<string, unknown>
			patched.push(body)
			return HttpResponse.json({ ...post, ...body, published_at: '2026-08-01T10:00:00Z' })
		}),
	)
}

beforeEach(() => {
	patched.length = 0
	serve(storedPost)
})

test('offers a way back to the list', async () => {
	renderAt(EDITOR_PATH)

	const back = await screen.findByRole('link', { name: 'Back to posts' })

	expect(back).toHaveAttribute('href', '/posts')
})

test('offers undo and redo, both idle over an untouched post', async () => {
	renderAt(EDITOR_PATH)

	expect(await screen.findByRole('button', { name: 'Undo' })).toHaveAttribute(
		'aria-disabled',
		'true',
	)
	expect(screen.getByRole('button', { name: 'Redo' })).toHaveAttribute('aria-disabled', 'true')
})

test('offers to publish a draft', async () => {
	renderAt(EDITOR_PATH)

	expect(await screen.findByRole('button', { name: 'Publish' })).toBeInTheDocument()
	expect(screen.queryByRole('button', { name: 'Update' })).not.toBeInTheDocument()
})

test('publishes a draft and asks the server for the transition', async () => {
	renderAt(EDITOR_PATH)

	await userEvent.click(await screen.findByRole('button', { name: 'Publish' }))

	await waitFor(() => expect(patched).toHaveLength(1))
	expect(patched[0]).toEqual({ status: 'published' })
})

test('swaps the control to update once the post is published', async () => {
	renderAt(EDITOR_PATH)

	await userEvent.click(await screen.findByRole('button', { name: 'Publish' }))

	expect(await screen.findByRole('button', { name: 'Update' })).toBeInTheDocument()
})

test('announces the publication', async () => {
	renderAt(EDITOR_PATH)

	await userEvent.click(await screen.findByRole('button', { name: 'Publish' }))

	expect(await screen.findByText(/published/i)).toBeInTheDocument()
})

test('offers to update a post that is already published', async () => {
	serve({ ...storedPost, status: 'published', published_at: '2026-07-20T10:00:00Z' })
	renderAt(EDITOR_PATH)

	expect(await screen.findByRole('button', { name: 'Update' })).toBeInTheDocument()
	expect(screen.queryByRole('button', { name: 'Publish' })).not.toBeInTheDocument()
})

test('reports a publication the server refused', async () => {
	vi.spyOn(console, 'error').mockImplementation(() => {})
	server.use(
		http.patch(`/api/posts/${storedPost.id}`, () =>
			HttpResponse.json({ error: 'post: invalid transition' }, { status: 422 }),
		),
	)
	renderAt(EDITOR_PATH)

	await userEvent.click(await screen.findByRole('button', { name: 'Publish' }))

	expect(await screen.findByText(/invalid transition/i)).toBeInTheDocument()
})
