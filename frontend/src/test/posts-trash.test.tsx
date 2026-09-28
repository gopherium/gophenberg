// SPDX-License-Identifier: Apache-2.0

import { http, HttpResponse, server } from '@gophenberg/frontend-sdk/testing'
import { act, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { beforeAll, beforeEach, expect, test, vi } from 'vitest'

import { fetchPost } from '../content/api'
import type { Router } from './anotherPost'
import { renderAt, renderRoutedAt } from './render'
import { storedPost } from './postFixture'
import { warmPostsScreen } from './warm'

warmPostsScreen()

beforeAll(async () => {
	await import('../content/EditorScreen')
}, 120000)

const RESTORED_AT = '2026-07-29T09:00:00Z'

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

const EDITOR_PATH = `/content/post/${TRASHED.id}/edit`

/**
 * Serves the post in the given status until the list trashes or restores it.
 * @param status - The status the post starts in.
 */
function serveMovable(status: 'draft' | 'trash') {
	let held: Record<string, unknown> | null = { ...storedPost, ...TRASHED, status }
	server.use(
		http.get('/api/content', ({ request }) => {
			listed.push(new URL(request.url).searchParams.get('status') ?? '')
			return HttpResponse.json(held === null ? { items: [], total: 0 } : { items: [held], total: 1 })
		}),
		http.get('/api/content/counts', () => {
			counted.push('asked')
			const inTrash = held?.status === 'trash'
			return HttpResponse.json({
				draft: held === null || inTrash ? 0 : 1, pending: 0, private: 0, published: 0, trash: inTrash ? 1 : 0,
			})
		}),
		http.get(`/api/content/${TRASHED.id}`, () =>
			held === null ? HttpResponse.json({}, { status: 404 }) : HttpResponse.json(held),
		),
		http.delete(`/api/content/${TRASHED.id}`, ({ request }) => {
			deleted.push(new URL(request.url))
			if (new URL(request.url).searchParams.get('force') === 'true') {
				held = null
				return new HttpResponse(null, { status: 204 })
			}
			held = { ...held, status: 'trash' }
			return HttpResponse.json(held)
		}),
		http.post(`/api/content/${TRASHED.id}/restore`, () => {
			restored.push(TRASHED.id)
			held = { ...held, status: 'draft', updated_at: RESTORED_AT }
			return HttpResponse.json(held)
		}),
	)
}

/**
 * Moves the admin to the posts list.
 * @param router - The router the admin runs on.
 */
async function goToList(router: Router) {
	await act(async () => {
		await router.navigate({ to: '/content/$typeKey', params: { typeKey: 'post' } })
	})
}

/**
 * Moves the admin to the editor of the post the trash tests move.
 * @param router - The router the admin runs on.
 */
async function goToEditor(router: Router) {
	await act(async () => {
		await router.navigate({
			to: '/content/$typeKey/$postId/edit',
			params: { typeKey: 'post', postId: TRASHED.id },
		})
	})
}

/**
 * Trashes the only listed post through its row actions and waits for the notice.
 */
async function trashTheListedPost() {
	await userEvent.click(await screen.findByRole('button', { name: 'Actions' }))
	await userEvent.click(await screen.findByRole('menuitem', { name: 'Move to Trash' }))
	await userEvent.click(await screen.findByRole('button', { name: 'Move to Trash' }))
	await screen.findByText('Moved to the trash.')
}

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

test('opens a post trashed from the list as a reading view', async () => {
	serveMovable('draft')
	const { router } = renderRoutedAt(EDITOR_PATH)
	await screen.findByRole('textbox', { name: 'Title' })
	await goToList(router)
	await trashTheListedPost()

	await goToEditor(router)

	expect(await screen.findByText(/This item is in the trash/)).toBeInTheDocument()
	expect(screen.queryByRole('textbox', { name: 'Title' })).not.toBeInTheDocument()
})

test('opens a post restored from the trash view as the editor', async () => {
	serveMovable('trash')
	const { router } = renderRoutedAt(EDITOR_PATH)
	await screen.findByText(/This item is in the trash/)
	await goToList(router)
	await openTrashedRowActions()
	await userEvent.click(await screen.findByRole('menuitem', { name: 'Restore' }))
	await waitFor(() => expect(restored).toEqual([TRASHED.id]))

	await goToEditor(router)

	expect(await screen.findByRole('textbox', { name: 'Title' })).toBeInTheDocument()
})

test('opens a post restored by undo as the editor', async () => {
	serveMovable('draft')
	const { client, router } = renderRoutedAt('/content/post')
	await trashTheListedPost()
	client.setQueryData(['post', TRASHED.id], await fetchPost(TRASHED.id))
	await userEvent.click(screen.getByRole('button', { name: 'Undo' }))
	await waitFor(() => expect(restored).toEqual([TRASHED.id]))

	await goToEditor(router)

	expect(await screen.findByRole('textbox', { name: 'Title' })).toBeInTheDocument()
})

test('opens a post deleted for good as a missing post', async () => {
	serveMovable('trash')
	const { router } = renderRoutedAt(EDITOR_PATH)
	await screen.findByText(/This item is in the trash/)
	await goToList(router)
	await openTrashedRowActions()
	await userEvent.click(await screen.findByRole('menuitem', { name: 'Delete Permanently' }))
	await userEvent.click(await screen.findByRole('button', { name: 'Delete Permanently' }))
	await waitFor(() => expect(deleted).toHaveLength(1))

	await goToEditor(router)

	expect(await screen.findByText('Could not load that post.')).toBeInTheDocument()
})

test('opens a post removed by emptying the trash as a missing post', async () => {
	serveMovable('trash')
	const { router } = renderRoutedAt(EDITOR_PATH)
	await screen.findByText(/This item is in the trash/)
	await goToList(router)
	await userEvent.click(await screen.findByRole('button', { name: 'Trash (1)' }))
	await userEvent.click(await screen.findByRole('button', { name: 'Empty Trash' }))
	await userEvent.click(await screen.findByRole('button', { name: 'Delete All' }))
	await waitFor(() => expect(deleted).toHaveLength(1))

	await goToEditor(router)

	expect(await screen.findByText('Could not load that post.')).toBeInTheDocument()
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
