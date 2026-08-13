// SPDX-License-Identifier: Apache-2.0

import { http, HttpResponse, server } from '@gophenberg/frontend-sdk/testing'
import { screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { beforeAll, beforeEach, expect, test, vi } from 'vitest'

import { renderAt } from './render'
import { storedPostWithId } from './postFixture'

beforeAll(async () => {
	await import('../content/EditorScreen')
}, 120000)

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
		http.get('/api/content', () => {
			listed.push('asked')
			return HttpResponse.json({ items: [PUBLISHED], total: 1 })
		}),
		http.get('/api/content/counts', () => {
			counted.push('asked')
			return HttpResponse.json({ draft: 0, pending: 0, private: 0, published: 1, trash: 0 })
		}),
		http.get('/api/content/:id', ({ params }) =>
			HttpResponse.json(storedPostWithId(String(params.id))),
		),
		http.delete('/api/content/:id', ({ params }) => {
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
	renderAt('/content/post')

	await openRowActions()

	expect(await screen.findByRole('menuitem', { name: 'Edit' })).toBeInTheDocument()
	expect(screen.getByRole('menuitem', { name: 'Move to Trash' })).toBeInTheDocument()
})

test('opens the editor from the row actions', async () => {
	renderAt('/content/post')
	await openRowActions()

	await userEvent.click(await screen.findByRole('menuitem', { name: 'Edit' }))

	expect(await screen.findByTitle('Editor canvas')).toBeInTheDocument()
})

test('asks to confirm before trashing', async () => {
	renderAt('/content/post')
	await openRowActions()

	await userEvent.click(await screen.findByRole('menuitem', { name: 'Move to Trash' }))

	expect(await screen.findByRole('dialog')).toHaveTextContent(/Welcome to Gophenberg/)
	expect(trashed).toEqual([])
})

test('names a post that has no title yet in the confirm', async () => {
	server.use(
		http.get('/api/content', () =>
			HttpResponse.json({ items: [{ ...PUBLISHED, title: '' }], total: 1 }),
		),
	)
	renderAt('/content/post')
	await screen.findByRole('link', { name: '(no title)' })
	await userEvent.click(screen.getByRole('button', { name: 'Actions' }))

	await userEvent.click(await screen.findByRole('menuitem', { name: 'Move to Trash' }))

	expect(await screen.findByRole('dialog')).toHaveTextContent(/\(no title\)/)
})

test('trashes the post once confirmed', async () => {
	renderAt('/content/post')
	await openRowActions()
	await userEvent.click(await screen.findByRole('menuitem', { name: 'Move to Trash' }))

	await userEvent.click(await screen.findByRole('button', { name: 'Move to Trash' }))

	await waitFor(() => expect(trashed).toEqual([PUBLISHED.id]))
})

test('leaves the post alone when the confirm is dismissed', async () => {
	renderAt('/content/post')
	await openRowActions()
	await userEvent.click(await screen.findByRole('menuitem', { name: 'Move to Trash' }))

	await userEvent.click(await screen.findByRole('button', { name: 'Cancel' }))

	await waitFor(() => expect(screen.queryByRole('dialog')).not.toBeInTheDocument())
	expect(trashed).toEqual([])
})

test('refreshes the listing and the counts after trashing', async () => {
	renderAt('/content/post')
	await openRowActions()
	await userEvent.click(await screen.findByRole('menuitem', { name: 'Move to Trash' }))
	const listedBefore = listed.length
	const countedBefore = counted.length

	await userEvent.click(await screen.findByRole('button', { name: 'Move to Trash' }))

	await waitFor(() => expect(listed.length).toBeGreaterThan(listedBefore))
	await waitFor(() => expect(counted.length).toBeGreaterThan(countedBefore))
})

test('offers to undo a post just trashed', async () => {
	renderAt('/content/post')
	await openRowActions()
	await userEvent.click(await screen.findByRole('menuitem', { name: 'Move to Trash' }))

	await userEvent.click(await screen.findByRole('button', { name: 'Move to Trash' }))

	expect(await screen.findByText(/moved to the trash/i)).toBeInTheDocument()
	expect(screen.getByRole('button', { name: 'Undo' })).toBeInTheDocument()
})

test('restores the post when the undo is taken', async () => {
	const restored: string[] = []
	server.use(
		http.post('/api/content/:id/restore', ({ params }) => {
			restored.push(String(params.id))
			return HttpResponse.json({ ...PUBLISHED, status: 'draft' })
		}),
	)
	renderAt('/content/post')
	await openRowActions()
	await userEvent.click(await screen.findByRole('menuitem', { name: 'Move to Trash' }))
	await userEvent.click(await screen.findByRole('button', { name: 'Move to Trash' }))

	await userEvent.click(await screen.findByRole('button', { name: 'Undo' }))

	await waitFor(() => expect(restored).toEqual([PUBLISHED.id]))
	await waitFor(() => expect(screen.queryByText(/moved to the trash/i)).not.toBeInTheDocument())
})

test('reports an undo the server refused', async () => {
	vi.spyOn(console, 'error').mockImplementation(() => {})
	server.use(http.post('/api/content/:id/restore', () => HttpResponse.json({}, { status: 500 })))
	renderAt('/content/post')
	await openRowActions()
	await userEvent.click(await screen.findByRole('menuitem', { name: 'Move to Trash' }))
	await userEvent.click(await screen.findByRole('button', { name: 'Move to Trash' }))

	await userEvent.click(await screen.findByRole('button', { name: 'Undo' }))

	expect(await screen.findByText(/could not restore that post/i)).toBeInTheDocument()
})

test('reports a trash the server refused', async () => {
	vi.spyOn(console, 'error').mockImplementation(() => {})
	server.use(http.delete('/api/content/:id', () => HttpResponse.json({}, { status: 500 })))
	renderAt('/content/post')
	await openRowActions()
	await userEvent.click(await screen.findByRole('menuitem', { name: 'Move to Trash' }))

	await userEvent.click(await screen.findByRole('button', { name: 'Move to Trash' }))

	expect(await screen.findByText(/could not move that post to trash/i)).toBeInTheDocument()
})
