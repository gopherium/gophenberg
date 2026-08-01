// SPDX-License-Identifier: Apache-2.0

import { http, HttpResponse, server } from '@gophenberg/frontend-sdk/testing'
import { screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { beforeAll, beforeEach, expect, test, vi } from 'vitest'

import { renderAt } from './render'
import { storedPost } from './postFixture'

const EDITOR_PATH = `/posts/${storedPost.id}/edit`

const trashed: string[] = []
const restored: string[] = []

beforeAll(async () => {
	await Promise.all([
		import('../posts/editorRoute.lazy'),
		import('../posts/postsRoutes.lazy'),
	])
}, 120000)

beforeEach(() => {
	trashed.length = 0
	restored.length = 0
	server.use(
		http.get(`/api/posts/${storedPost.id}`, () => HttpResponse.json(storedPost)),
		http.get('/api/posts', () => HttpResponse.json({ items: [], total: 0 })),
		http.get('/api/posts/counts', () =>
			HttpResponse.json({ draft: 0, pending: 0, private: 0, published: 0, trash: 1 }),
		),
		http.delete(`/api/posts/${storedPost.id}`, ({ params }) => {
			trashed.push(String(params.id ?? storedPost.id))
			return HttpResponse.json({ ...storedPost, status: 'trash' })
		}),
		http.post(`/api/posts/${storedPost.id}/restore`, () => {
			restored.push(storedPost.id)
			return HttpResponse.json({ ...storedPost, status: 'draft' })
		}),
	)
})

test('offers to trash the post from the document tab', async () => {
	renderAt(EDITOR_PATH)

	expect(await screen.findByRole('button', { name: 'Move to trash' })).toBeInTheDocument()
})

test('asks to confirm before trashing', async () => {
	renderAt(EDITOR_PATH)

	await userEvent.click(await screen.findByRole('button', { name: 'Move to trash' }))

	expect(await screen.findByRole('alertdialog')).toBeInTheDocument()
	expect(trashed).toEqual([])
})

test('keeps the post when the confirm is dismissed', async () => {
	renderAt(EDITOR_PATH)
	await userEvent.click(await screen.findByRole('button', { name: 'Move to trash' }))

	await userEvent.click(await screen.findByRole('button', { name: 'Cancel' }))

	await waitFor(() => expect(screen.queryByRole('alertdialog')).not.toBeInTheDocument())
	expect(trashed).toEqual([])
})

test('trashes the post once confirmed', async () => {
	renderAt(EDITOR_PATH)
	await userEvent.click(await screen.findByRole('button', { name: 'Move to trash' }))

	await userEvent.click(await screen.findByRole('button', { name: 'Move to trash' }))

	await waitFor(() => expect(trashed).toHaveLength(1))
})

test('leaves the editor for the list once the post is trashed', async () => {
	renderAt(EDITOR_PATH)
	await userEvent.click(await screen.findByRole('button', { name: 'Move to trash' }))

	await userEvent.click(await screen.findByRole('button', { name: 'Move to trash' }))

	await waitFor(() => expect(screen.queryByTitle('Editor canvas')).not.toBeInTheDocument())
})

test('carries the undo across the move to the list', async () => {
	renderAt(EDITOR_PATH)
	await userEvent.click(await screen.findByRole('button', { name: 'Move to trash' }))
	await userEvent.click(await screen.findByRole('button', { name: 'Move to trash' }))

	await waitFor(() => expect(screen.queryByTitle('Editor canvas')).not.toBeInTheDocument())

	expect(await screen.findByText('Moved to the trash.')).toBeInTheDocument()
	await userEvent.click(screen.getByRole('button', { name: 'Undo' }))
	await waitFor(() => expect(restored).toEqual([storedPost.id]))
})

test('names a post that has no title yet in the confirm', async () => {
	server.use(
		http.get(`/api/posts/${storedPost.id}`, () =>
			HttpResponse.json({ ...storedPost, title: '' }),
		),
	)
	renderAt(EDITOR_PATH)

	await userEvent.click(await screen.findByRole('button', { name: 'Move to trash' }))

	expect(await screen.findByRole('alertdialog')).toHaveTextContent(/This post goes to the trash/)
})

test('reports an undo the server refused', async () => {
	vi.spyOn(console, 'error').mockImplementation(() => {})
	server.use(
		http.post(`/api/posts/${storedPost.id}/restore`, () =>
			HttpResponse.json({}, { status: 500 }),
		),
	)
	renderAt(EDITOR_PATH)
	await userEvent.click(await screen.findByRole('button', { name: 'Move to trash' }))
	await userEvent.click(await screen.findByRole('button', { name: 'Move to trash' }))
	await screen.findByText('Moved to the trash.')

	await userEvent.click(screen.getByRole('button', { name: 'Undo' }))

	expect(await screen.findByText(/could not restore that post/i)).toBeInTheDocument()
})

test('reports a trash the server refused', async () => {
	vi.spyOn(console, 'error').mockImplementation(() => {})
	server.use(
		http.delete(`/api/posts/${storedPost.id}`, () => HttpResponse.json({}, { status: 500 })),
	)
	renderAt(EDITOR_PATH)
	await userEvent.click(await screen.findByRole('button', { name: 'Move to trash' }))

	await userEvent.click(await screen.findByRole('button', { name: 'Move to trash' }))

	expect(await screen.findByText(/could not move that post to trash/i)).toBeInTheDocument()
})
