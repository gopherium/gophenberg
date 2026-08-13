// SPDX-License-Identifier: Apache-2.0

import { http, HttpResponse, server } from '@gophenberg/frontend-sdk/testing'
import { screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { beforeEach, expect, test, vi } from 'vitest'

import { renderAt } from './render'

const TRASHED = {
	id: '019fb000-0000-7000-8000-000000000003',
	type: 'post',
	slug: 'old-notes-trashed-a1b2c3d4',
	title: 'Old Notes',
	excerpt: '',
	status: 'trash',
	author_id: '019fb000-0000-7000-8000-0000000000ff',
	author_name: 'Maria Perez',
	published_at: null,
	created_at: '2026-07-19T10:00:00Z',
	updated_at: '2026-07-28T09:00:00Z',
}

const listed: string[] = []
const counted: string[] = []
const restored: string[] = []
const deleted: URL[] = []

beforeEach(() => {
	listed.length = 0
	counted.length = 0
	restored.length = 0
	deleted.length = 0
	server.use(
		http.get('/api/content', ({ request }) => {
			listed.push(new URL(request.url).searchParams.get('status') ?? '')
			return HttpResponse.json({ items: [TRASHED], total: 1 })
		}),
		http.get('/api/content/counts', () => {
			counted.push('asked')
			return HttpResponse.json({ draft: 0, pending: 0, private: 0, published: 0, trash: 1 })
		}),
		http.post('/api/content/:id/restore', ({ params }) => {
			restored.push(String(params.id))
			return HttpResponse.json({ ...TRASHED, status: 'draft' })
		}),
		http.delete('/api/content/:id', ({ request }) => {
			deleted.push(new URL(request.url))
			return new HttpResponse(null, { status: 204 })
		}),
	)
})

/**
 * Opens the row actions of the only post listed in the trash view.
 */
async function openTrashedRowActions() {
	await screen.findByRole('button', { name: 'Trash (1)' })
	await userEvent.click(screen.getByRole('button', { name: 'Trash (1)' }))
	await waitFor(() => expect(listed).toContain('trash'))
	await userEvent.click(await screen.findByRole('button', { name: 'Actions' }))
}

test('swaps the row actions in the trash view', async () => {
	renderAt('/content/post')

	await openTrashedRowActions()

	expect(await screen.findByRole('menuitem', { name: 'Restore' })).toBeInTheDocument()
	expect(screen.getByRole('menuitem', { name: 'Delete Permanently' })).toBeInTheDocument()
	expect(screen.queryByRole('menuitem', { name: 'Move to Trash' })).not.toBeInTheDocument()
	expect(screen.queryByRole('menuitem', { name: 'Edit' })).not.toBeInTheDocument()
})

test('restores a post out of the trash', async () => {
	renderAt('/content/post')
	await openTrashedRowActions()

	await userEvent.click(await screen.findByRole('menuitem', { name: 'Restore' }))

	await waitFor(() => expect(restored).toEqual([TRASHED.id]))
})

test('refreshes the listing and the counts after a restore', async () => {
	renderAt('/content/post')
	await openTrashedRowActions()
	const listedBefore = listed.length
	const countedBefore = counted.length

	await userEvent.click(await screen.findByRole('menuitem', { name: 'Restore' }))

	await waitFor(() => expect(listed.length).toBeGreaterThan(listedBefore))
	await waitFor(() => expect(counted.length).toBeGreaterThan(countedBefore))
})

test('asks to confirm before deleting permanently', async () => {
	renderAt('/content/post')
	await openTrashedRowActions()

	await userEvent.click(await screen.findByRole('menuitem', { name: 'Delete Permanently' }))

	expect(await screen.findByRole('dialog')).toHaveTextContent(/Old Notes/)
	expect(deleted).toEqual([])
})

test('deletes for good once confirmed', async () => {
	renderAt('/content/post')
	await openTrashedRowActions()
	await userEvent.click(await screen.findByRole('menuitem', { name: 'Delete Permanently' }))

	await userEvent.click(await screen.findByRole('button', { name: 'Delete Permanently' }))

	await waitFor(() => expect(deleted).toHaveLength(1))
	expect(deleted[0].pathname).toBe(`/api/content/${TRASHED.id}`)
	expect(deleted[0].searchParams.get('force')).toBe('true')
})

test('keeps the post when the permanent delete is dismissed', async () => {
	renderAt('/content/post')
	await openTrashedRowActions()
	await userEvent.click(await screen.findByRole('menuitem', { name: 'Delete Permanently' }))

	await userEvent.click(await screen.findByRole('button', { name: 'Cancel' }))

	await waitFor(() => expect(screen.queryByRole('dialog')).not.toBeInTheDocument())
	expect(deleted).toEqual([])
})

test('reports a restore the server refused', async () => {
	vi.spyOn(console, 'error').mockImplementation(() => {})
	server.use(http.post('/api/content/:id/restore', () => HttpResponse.json({}, { status: 500 })))
	renderAt('/content/post')
	await openTrashedRowActions()

	await userEvent.click(await screen.findByRole('menuitem', { name: 'Restore' }))

	expect(await screen.findByText(/could not restore that post/i)).toBeInTheDocument()
})

test('reports a permanent delete the server refused', async () => {
	vi.spyOn(console, 'error').mockImplementation(() => {})
	server.use(http.delete('/api/content/:id', () => HttpResponse.json({}, { status: 500 })))
	renderAt('/content/post')
	await openTrashedRowActions()
	await userEvent.click(await screen.findByRole('menuitem', { name: 'Delete Permanently' }))

	await userEvent.click(await screen.findByRole('button', { name: 'Delete Permanently' }))

	expect(await screen.findByText(/could not delete that post/i)).toBeInTheDocument()
})
