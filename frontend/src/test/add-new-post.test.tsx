// SPDX-License-Identifier: Apache-2.0

import { http, HttpResponse, server } from '@gophenberg/frontend-sdk/testing'
import { screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { beforeAll, beforeEach, expect, test, vi } from 'vitest'

import { renderAt } from './render'
import { storedPostWithId } from './postFixture'

beforeAll(async () => {
	await import('../posts/EditorScreen')
}, 120000)

beforeEach(() => {
	server.use(
		http.get('/api/posts/:id', ({ params }) =>
			HttpResponse.json(storedPostWithId(String(params.id))),
		),
	)
})

const NEW_POST_ID = '019fb000-0000-7000-8000-0000000000aa'

/**
 * Serves a created draft and records the request body.
 * @returns The recorded bodies, filled as requests arrive.
 */
function captureCreate(): { bodies: unknown[] } {
	const bodies: unknown[] = []
	server.use(
		http.post('/api/posts', async ({ request }) => {
			bodies.push(await request.json())
			return HttpResponse.json(
				{ id: NEW_POST_ID, type: 'post', slug: 'untitled', title: '', status: 'draft' },
				{ status: 201 },
			)
		}),
	)
	return { bodies }
}

test('add new creates a draft and opens it in the editor', async () => {
	captureCreate()
	renderAt('/posts')

	await userEvent.click(await screen.findByRole('button', { name: 'Add New' }))

	expect(await screen.findByTitle('Editor canvas')).toBeInTheDocument()
})

test('add new asks for a post of the default type', async () => {
	const recorded = captureCreate()
	renderAt('/posts')

	await userEvent.click(await screen.findByRole('button', { name: 'Add New' }))
	await screen.findByTitle('Editor canvas')

	expect(recorded.bodies).toEqual([{ type: 'post', title: '' }])
})

test('add new reports a failure without navigating away', async () => {
	vi.spyOn(console, 'error').mockImplementation(() => {})
	server.use(
		http.get('/api/posts', () => HttpResponse.json({ items: [], total: 0 })),
		http.post('/api/posts', () =>
			HttpResponse.json({ error: 'nope' }, { status: 500 }),
		),
	)
	renderAt('/posts')

	await userEvent.click(await screen.findByRole('button', { name: 'Add New' }))

	expect(await screen.findByText(/could not create a draft/i)).toBeInTheDocument()
	expect(screen.queryByTitle('Editor canvas')).not.toBeInTheDocument()
})
