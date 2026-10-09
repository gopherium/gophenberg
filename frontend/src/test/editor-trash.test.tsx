// SPDX-License-Identifier: Apache-2.0

import { http, HttpResponse, server } from '@gophenberg/frontend-sdk/testing'
import { act, screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { beforeAll, beforeEach, expect, test, vi } from 'vitest'

import { gate } from './gate'
import { renderAt, renderRoutedAt } from './render'
import { storedPost } from './postFixture'
import { siteSettings } from './siteSettings'

const EDITOR_PATH = `/content/post/${storedPost.id}/edit`

const RESTORED_AT = '2026-07-29T09:00:00Z'

const trashed: string[] = []
const restored: string[] = []

/**
 * Serves the post in the trash until a restore returns it to draft at a newer version.
 * @returns The bodies of the saves made.
 */
function serveRestorable(): { patched: Record<string, unknown>[] } {
	let held: Record<string, unknown> = { ...storedPost, status: 'trash' }
	const patched: Record<string, unknown>[] = []
	server.use(
		http.get(`/api/content/${storedPost.id}`, () => HttpResponse.json(held)),
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
	return { patched }
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

test('draws Move to trash as an outline button of the default size, as WordPress draws its trash button', async () => {
	renderAt(EDITOR_PATH)

	const trigger = await screen.findByRole('button', { name: 'Move to trash' })

	expect(trigger.className).toMatch(/__is-outline\b/)
	expect(trigger.className).not.toMatch(/__is-(compact|small)\b/)
})

test('describes the trash confirm by its question, so a screen reader reads it', async () => {
	renderAt(EDITOR_PATH)

	await userEvent.click(await screen.findByRole('button', { name: 'Move to trash' }))

	expect(await screen.findByRole('dialog', { name: 'Move to trash' }))
		.toHaveAccessibleDescription('Move "Welcome to Gophenberg" to the trash?')
})

test('asks to confirm before trashing in a dialog under no header, as WordPress does', async () => {
	renderAt(EDITOR_PATH)

	await userEvent.click(await screen.findByRole('button', { name: 'Move to trash' }))

	const dialog = await screen.findByRole('dialog', { name: 'Move to trash' })
	expect(dialog).toHaveTextContent('Move "Welcome to Gophenberg" to the trash?')
	expect(within(dialog).getByRole('heading', { name: 'Move to trash' })).toHaveAttribute('data-visually-hidden')
	expect(within(dialog).queryByRole('button', { name: 'Close' })).not.toBeInTheDocument()
	expect(screen.queryByRole('alertdialog')).not.toBeInTheDocument()
	expect(trashed).toEqual([])
})

test('cuts a long title in the trash confirm at the length the site serves', async () => {
	server.use(http.get('/api/settings', () => HttpResponse.json({ ...siteSettings, toast_name_length: 10 })))
	renderAt(EDITOR_PATH)

	await userEvent.click(await screen.findByRole('button', { name: 'Move to trash' }))

	expect(await screen.findByRole('dialog')).toHaveTextContent('Move "Welcome to…" to the trash?')
})

test('keeps the post when the confirm is dismissed', async () => {
	renderAt(EDITOR_PATH)
	await userEvent.click(await screen.findByRole('button', { name: 'Move to trash' }))

	await userEvent.click(await screen.findByRole('button', { name: 'Cancel' }))

	await waitFor(() => expect(screen.queryByRole('dialog')).not.toBeInTheDocument())
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

test('names the post it trashed in a toast carried to the list, with no undo', async () => {
	renderAt(EDITOR_PATH)
	await userEvent.click(await screen.findByRole('button', { name: 'Move to trash' }))
	await userEvent.click(await screen.findByRole('button', { name: 'Move to trash' }))

	await waitFor(() => expect(screen.queryByTitle('Editor canvas')).not.toBeInTheDocument())

	expect(await screen.findByText('"Welcome to Gophenberg" moved to the trash.', { selector: '.godmin-toast' }))
		.toBeInTheDocument()
	expect(screen.queryByRole('button', { name: 'Undo' })).not.toBeInTheDocument()
	expect(restored).toEqual([])
})

test('names a post that has no title yet in the confirm and the toast', async () => {
	server.use(
		http.get(`/api/content/${storedPost.id}`, () =>
			HttpResponse.json({ ...storedPost, title: '' }),
		),
	)
	renderAt(EDITOR_PATH)
	await userEvent.click(await screen.findByRole('button', { name: 'Move to trash' }))

	expect(await screen.findByRole('dialog')).toHaveTextContent('Move "(no title)" to the trash?')
	await userEvent.click(await screen.findByRole('button', { name: 'Move to trash' }))

	expect(await screen.findByText('"(no title)" moved to the trash.', { selector: '.godmin-toast' })).toBeInTheDocument()
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
	expect(await screen.findByText('"Welcome to Gophenberg" has been restored.', { selector: '.godmin-toast' }))
		.toBeInTheDocument()
})

test('names an untitled post it restored from the reading view in the toast', async () => {
	let held: Record<string, unknown> = { ...storedPost, title: '', status: 'trash' }
	server.use(
		http.get(`/api/content/${storedPost.id}`, () => HttpResponse.json(held)),
		http.post(`/api/content/${storedPost.id}/restore`, () => {
			held = { ...storedPost, title: '' }
			return HttpResponse.json(held)
		}),
	)
	renderAt(EDITOR_PATH)

	await userEvent.click(await screen.findByRole('button', { name: 'Restore' }))

	expect(await screen.findByText('"(no title)" has been restored.', { selector: '.godmin-toast' })).toBeInTheDocument()
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

test('opens the editor when the restore finds the post already out of the trash', async () => {
	vi.spyOn(console, 'error').mockImplementation(() => {})
	let held: Record<string, unknown> = { ...storedPost, status: 'trash' }
	server.use(
		http.get(`/api/content/${storedPost.id}`, () => HttpResponse.json(held)),
		http.post(`/api/content/${storedPost.id}/restore`, () => {
			held = { ...storedPost, updated_at: RESTORED_AT }
			return HttpResponse.json(
				{ error: 'content: only a trashed item can be restored', code: 'restore_not_trashed' },
				{ status: 422 },
			)
		}),
	)
	renderAt(EDITOR_PATH)

	await userEvent.click(await screen.findByRole('button', { name: 'Restore' }))

	expect(await screen.findByRole('textbox', { name: 'Title' })).toBeInTheDocument()
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

	expect(await screen.findByText('The item could not be restored.')).toBeInTheDocument()
})

test('reports the reason a refused restore gave in the reading view', async () => {
	vi.spyOn(console, 'error').mockImplementation(() => {})
	server.use(
		http.get(`/api/content/${storedPost.id}`, () =>
			HttpResponse.json({ ...storedPost, status: 'trash' }),
		),
		http.post(`/api/content/${storedPost.id}/restore`, () =>
			HttpResponse.json({ error: 'content: parent is in the trash', code: 'parent_trashed' }, { status: 422 }),
		),
	)
	renderAt(EDITOR_PATH)

	await userEvent.click(await screen.findByRole('button', { name: 'Restore' }))

	expect(
		await screen.findByText('The item you picked as parent is in the trash. Restore it, or pick another parent.'),
	).toBeInTheDocument()
})

test('opens a post trashed from the editor as a reading view', async () => {
	let held: Record<string, unknown> = storedPost
	server.use(
		http.get(`/api/content/${storedPost.id}`, () => HttpResponse.json(held)),
		http.delete(`/api/content/${storedPost.id}`, () => {
			held = { ...storedPost, status: 'trash' }
			return HttpResponse.json(held)
		}),
	)
	const { router } = renderRoutedAt(EDITOR_PATH)
	await userEvent.click(await screen.findByRole('button', { name: 'Move to trash' }))
	await userEvent.click(await screen.findByRole('button', { name: 'Move to trash' }))
	await screen.findByRole('heading', { name: 'Posts', level: 1 })

	await act(async () => {
		await router.navigate({
			to: '/content/$typeKey/$postId/edit',
			params: { typeKey: 'post', postId: storedPost.id },
		})
	})

	expect(await screen.findByText(/This item is in the trash/)).toBeInTheDocument()
	expect(screen.queryByRole('textbox', { name: 'Title' })).not.toBeInTheDocument()
})

test('reports a trash the server refused inside the confirm, worded as the list words it', async () => {
	vi.spyOn(console, 'error').mockImplementation(() => {})
	server.use(
		http.delete(`/api/content/${storedPost.id}`, () => HttpResponse.json({}, { status: 500 })),
	)
	renderAt(EDITOR_PATH)
	await userEvent.click(await screen.findByRole('button', { name: 'Move to trash' }))

	await userEvent.click(await screen.findByRole('button', { name: 'Move to trash' }))

	const dialog = await screen.findByRole('dialog')
	expect(await within(dialog).findByText('The item could not be moved to the trash.')).toBeInTheDocument()
	expect(screen.queryByText('The item could not be moved to the trash.', { selector: '.godmin-toast' }))
		.not.toBeInTheDocument()
})

test('opens the trash confirm again without the failure of the last try', async () => {
	vi.spyOn(console, 'error').mockImplementation(() => {})
	server.use(
		http.delete(`/api/content/${storedPost.id}`, () => HttpResponse.json({}, { status: 500 })),
	)
	renderAt(EDITOR_PATH)
	await userEvent.click(await screen.findByRole('button', { name: 'Move to trash' }))
	await userEvent.click(await screen.findByRole('button', { name: 'Move to trash' }))
	await within(await screen.findByRole('dialog')).findByText('The item could not be moved to the trash.')
	await userEvent.click(screen.getByRole('button', { name: 'Cancel' }))
	await waitFor(() => expect(screen.queryByRole('dialog')).not.toBeInTheDocument())

	await userEvent.click(screen.getByRole('button', { name: 'Move to trash' }))

	expect(await screen.findByRole('dialog')).not.toHaveTextContent('The item could not be moved to the trash.')
})

test('reports the reason a refused trash gave inside the confirm', async () => {
	vi.spyOn(console, 'error').mockImplementation(() => {})
	server.use(
		http.delete(`/api/content/${storedPost.id}`, () =>
			HttpResponse.json({ error: 'content: item holds children', code: 'content_holds_children' }, { status: 422 }),
		),
	)
	renderAt(EDITOR_PATH)
	await userEvent.click(await screen.findByRole('button', { name: 'Move to trash' }))

	await userEvent.click(await screen.findByRole('button', { name: 'Move to trash' }))

	expect(
		await within(await screen.findByRole('dialog')).findByText(
			'This item still holds items nested inside it. Move or delete those first.',
		),
	).toBeInTheDocument()
})

test('keeps the confirm open when Cancel is pressed while the trash runs', async () => {
	const answer = gate()
	server.use(
		http.delete(`/api/content/${storedPost.id}`, async () => {
			await answer.held
			return HttpResponse.json({ ...storedPost, status: 'trash' })
		}),
	)
	renderAt(EDITOR_PATH)
	await userEvent.click(await screen.findByRole('button', { name: 'Move to trash' }))
	const dialog = await screen.findByRole('dialog')
	await userEvent.click(within(dialog).getByRole('button', { name: 'Move to trash' }))

	await userEvent.click(within(dialog).getByRole('button', { name: 'Cancel' }))

	expect(dialog).toBeInTheDocument()
	answer.release()
	expect(await screen.findByRole('heading', { name: 'Posts', level: 1 })).toBeInTheDocument()
})
