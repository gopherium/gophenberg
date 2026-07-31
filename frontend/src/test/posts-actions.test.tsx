// SPDX-License-Identifier: Apache-2.0

import { http, HttpResponse, server } from '@gophenberg/frontend-sdk/testing'
import { screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { beforeEach, expect, test, vi } from 'vitest'

import { renderAt } from './render'

const PUBLISHED = {
	id: '019fb000-0000-7000-8000-000000000001',
	type: 'post',
	slug: 'welcome',
	title: 'Welcome to Gophenberg',
	excerpt: '',
	status: 'published',
	author_id: '019fb000-0000-7000-8000-0000000000ff',
	author_name: 'Maria Perez',
	published_at: '2026-07-20T10:00:00Z',
	created_at: '2026-07-19T10:00:00Z',
	updated_at: '2026-07-20T10:00:00Z',
}

const listed: string[] = []
const counted: string[] = []
const trashed: string[] = []

beforeEach(() => {
	listed.length = 0
	counted.length = 0
	trashed.length = 0
	server.use(
		http.get('/api/posts', () => {
			listed.push('asked')
			return HttpResponse.json({ items: [PUBLISHED], total: 1 })
		}),
		http.get('/api/posts/counts', () => {
			counted.push('asked')
			return HttpResponse.json({ draft: 0, pending: 0, private: 0, published: 1, trash: 0 })
		}),
		http.delete('/api/posts/:id', ({ params }) => {
			trashed.push(String(params.id))
			return HttpResponse.json({ ...PUBLISHED, status: 'trash' })
		}),
	)
})

/**
 * Opens the row actions menu of the only listed post.
 */
async function openRowActions() {
	await screen.findByText('Welcome to Gophenberg')
	await userEvent.click(screen.getByRole('button', { name: 'Actions' }))
}

test('offers edit and trash on a row', async () => {
	renderAt('/posts')

	await openRowActions()

	expect(await screen.findByRole('menuitem', { name: 'Edit' })).toBeInTheDocument()
	expect(screen.getByRole('menuitem', { name: 'Move to Trash' })).toBeInTheDocument()
})

test('opens the editor from the row actions', async () => {
	renderAt('/posts')
	await openRowActions()

	await userEvent.click(await screen.findByRole('menuitem', { name: 'Edit' }))

	expect(await screen.findByRole('heading', { name: /editor/i })).toBeInTheDocument()
})

test('asks to confirm before trashing', async () => {
	renderAt('/posts')
	await openRowActions()

	await userEvent.click(await screen.findByRole('menuitem', { name: 'Move to Trash' }))

	expect(await screen.findByRole('dialog')).toHaveTextContent(/Welcome to Gophenberg/)
	expect(trashed).toEqual([])
})

test('names a post that has no title yet in the confirm', async () => {
	server.use(
		http.get('/api/posts', () =>
			HttpResponse.json({ items: [{ ...PUBLISHED, title: '' }], total: 1 }),
		),
	)
	renderAt('/posts')
	await screen.findByRole('link', { name: '(no title)' })
	await userEvent.click(screen.getByRole('button', { name: 'Actions' }))

	await userEvent.click(await screen.findByRole('menuitem', { name: 'Move to Trash' }))

	expect(await screen.findByRole('dialog')).toHaveTextContent(/\(no title\)/)
})

test('trashes the post once confirmed', async () => {
	renderAt('/posts')
	await openRowActions()
	await userEvent.click(await screen.findByRole('menuitem', { name: 'Move to Trash' }))

	await userEvent.click(await screen.findByRole('button', { name: 'Move to Trash' }))

	await waitFor(() => expect(trashed).toEqual([PUBLISHED.id]))
})

test('leaves the post alone when the confirm is dismissed', async () => {
	renderAt('/posts')
	await openRowActions()
	await userEvent.click(await screen.findByRole('menuitem', { name: 'Move to Trash' }))

	await userEvent.click(await screen.findByRole('button', { name: 'Cancel' }))

	await waitFor(() => expect(screen.queryByRole('dialog')).not.toBeInTheDocument())
	expect(trashed).toEqual([])
})

test('refreshes the listing and the counts after trashing', async () => {
	renderAt('/posts')
	await openRowActions()
	await userEvent.click(await screen.findByRole('menuitem', { name: 'Move to Trash' }))
	const listedBefore = listed.length
	const countedBefore = counted.length

	await userEvent.click(await screen.findByRole('button', { name: 'Move to Trash' }))

	await waitFor(() => expect(listed.length).toBeGreaterThan(listedBefore))
	await waitFor(() => expect(counted.length).toBeGreaterThan(countedBefore))
})

test('reports a trash the server refused', async () => {
	vi.spyOn(console, 'error').mockImplementation(() => {})
	server.use(http.delete('/api/posts/:id', () => HttpResponse.json({}, { status: 500 })))
	renderAt('/posts')
	await openRowActions()
	await userEvent.click(await screen.findByRole('menuitem', { name: 'Move to Trash' }))

	await userEvent.click(await screen.findByRole('button', { name: 'Move to Trash' }))

	expect(await screen.findByText(/could not move that post to trash/i)).toBeInTheDocument()
})
