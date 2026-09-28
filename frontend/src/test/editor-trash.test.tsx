// SPDX-License-Identifier: Apache-2.0

import { http, HttpResponse, server } from '@gophenberg/frontend-sdk/testing'
import { act, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { beforeAll, beforeEach, expect, test, vi } from 'vitest'

import { renderAt, renderRoutedAt } from './render'
import { storedPost } from './postFixture'

const EDITOR_PATH = `/content/post/${storedPost.id}/edit`

const RESTORED_AT = '2026-07-29T09:00:00Z'

const trashed: string[] = []
const restored: string[] = []

/**
 * Serves the post in the trash until a restore returns it to draft at a newer version.
 * @returns The bodies of the saves made and the reads of the status counts.
 */
function serveRestorable(): { patched: Record<string, unknown>[], countsAsked: string[] } {
	let held: Record<string, unknown> = { ...storedPost, status: 'trash' }
	const patched: Record<string, unknown>[] = []
	const countsAsked: string[] = []
	server.use(
		http.get(`/api/content/${storedPost.id}`, () => HttpResponse.json(held)),
		http.get('/api/content/counts', () => {
			countsAsked.push('read')
			return HttpResponse.json({
				draft: 0, pending: 0, private: 0, published: 0, trash: held.status === 'trash' ? 1 : 0,
			})
		}),
		http.post(`/api/content/${storedPost.id}/restore`, () => {
			restored.push(storedPost.id)
			held = { ...storedPost, updated_at: RESTORED_AT }
			return HttpResponse.json(held)
		}),
		http.patch(`/api/content/${storedPost.id}`, async ({ request }) => {
			const body = (await request.json()) as Record<string, unknown>
			patched.push(body)
			return HttpResponse.json({ ...held, ...body })
		}),
	)
	return { patched, countsAsked }
}

beforeAll(async () => {
	await Promise.all([
		import('../content/EditorScreen'),
		import('../content/PostsScreen'),
	])
}, 120000)

beforeEach(() => {
	trashed.length = 0
	restored.length = 0
	server.use(
		http.get(`/api/content/${storedPost.id}`, () => HttpResponse.json(storedPost)),
		http.get('/api/content', () => HttpResponse.json({ items: [], total: 0 })),
		http.get('/api/content/counts', () =>
			HttpResponse.json({ draft: 0, pending: 0, private: 0, published: 0, trash: 1 }),
		),
		http.delete(`/api/content/${storedPost.id}`, ({ params }) => {
			trashed.push(String(params.id ?? storedPost.id))
			return HttpResponse.json({ ...storedPost, status: 'trash' })
		}),
		http.post(`/api/content/${storedPost.id}/restore`, () => {
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

test('lands on the list of the type it left', async () => {
	renderAt(EDITOR_PATH)
	await userEvent.click(await screen.findByRole('button', { name: 'Move to trash' }))

	await userEvent.click(await screen.findByRole('button', { name: 'Move to trash' }))

	expect(await screen.findByRole('heading', { name: 'Posts', level: 1 })).toBeInTheDocument()
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
		http.get(`/api/content/${storedPost.id}`, () =>
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
		http.post(`/api/content/${storedPost.id}/restore`, () =>
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

test('opens a trashed post as a reading view', async () => {
	server.use(
		http.get(`/api/content/${storedPost.id}`, () =>
			HttpResponse.json({ ...storedPost, status: 'trash' }),
		),
	)
	renderAt(EDITOR_PATH)

	expect(await screen.findByText(/This item is in the trash/)).toBeInTheDocument()
	expect(screen.queryByRole('textbox', { name: 'Title' })).not.toBeInTheDocument()
	expect(screen.queryByRole('button', { name: 'Move to trash' })).not.toBeInTheDocument()
	expect(screen.queryByRole('button', { name: /save draft|publish/i })).not.toBeInTheDocument()
})

test('restores a trashed post and opens it in the editor', async () => {
	serveRestorable()
	renderAt(EDITOR_PATH)

	await userEvent.click(await screen.findByRole('button', { name: 'Restore' }))

	expect(await screen.findByRole('textbox', { name: 'Title' })).toHaveValue(storedPost.title)
	expect(restored).toEqual([storedPost.id])
})

test('saves a restored post against the version the restore stamped', async () => {
	const { patched } = serveRestorable()
	renderAt(EDITOR_PATH)
	await userEvent.click(await screen.findByRole('button', { name: 'Restore' }))
	await userEvent.type(await screen.findByRole('textbox', { name: 'Title' }), '!')

	await userEvent.click(screen.getByRole('button', { name: 'Save draft' }))

	await waitFor(() => expect(patched).toHaveLength(1))
	expect(patched[0].updated_at).toBe(RESTORED_AT)
})

test('refreshes the status counts after a restore from the reading view', async () => {
	const { countsAsked } = serveRestorable()
	const { router } = renderRoutedAt('/content/post')
	await screen.findByRole('button', { name: 'Trash (1)' })
	await act(async () => {
		await router.navigate({
			to: '/content/$typeKey/$postId/edit',
			params: { typeKey: 'post', postId: storedPost.id },
		})
	})
	await userEvent.click(await screen.findByRole('button', { name: 'Restore' }))
	await screen.findByRole('textbox', { name: 'Title' })

	await act(async () => {
		await router.navigate({ to: '/content/$typeKey', params: { typeKey: 'post' } })
	})

	await waitFor(() => expect(countsAsked).toHaveLength(2))
})

test('reports a restore the reading view could not make', async () => {
	vi.spyOn(console, 'error').mockImplementation(() => {})
	server.use(
		http.get(`/api/content/${storedPost.id}`, () =>
			HttpResponse.json({ ...storedPost, status: 'trash' }),
		),
		http.post(`/api/content/${storedPost.id}/restore`, () =>
			HttpResponse.json({}, { status: 500 }),
		),
	)
	renderAt(EDITOR_PATH)

	await userEvent.click(await screen.findByRole('button', { name: 'Restore' }))

	expect(await screen.findByText(/could not restore that post/i)).toBeInTheDocument()
})

test('reports a trash the server refused', async () => {
	vi.spyOn(console, 'error').mockImplementation(() => {})
	server.use(
		http.delete(`/api/content/${storedPost.id}`, () => HttpResponse.json({}, { status: 500 })),
	)
	renderAt(EDITOR_PATH)
	await userEvent.click(await screen.findByRole('button', { name: 'Move to trash' }))

	await userEvent.click(await screen.findByRole('button', { name: 'Move to trash' }))

	expect(await screen.findByText(/could not move that post to trash/i)).toBeInTheDocument()
})
