// SPDX-License-Identifier: Apache-2.0

import { http, HttpResponse, server } from '@gophenberg/frontend-sdk/testing'
import { act, screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { resetLocaleData, setLocaleData } from '@wordpress/i18n'
import { beforeAll, beforeEach, expect, onTestFinished, test, vi } from 'vitest'

import { deleteQuestion } from '../content/actionWords'
import type { Post } from '../content/api'
import { catalogFor } from '../i18n/catalog'
import type { Router } from './anotherPost'
import { gate } from './gate'
import { renderAt, renderRoutedAt } from './render'
import { storedPost } from './postFixture'
import { siteSettings } from './siteSettings'
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

const SECOND_ID = '019fb000-0000-7000-8000-000000000004'

const FRESH = { ...TRASHED, id: '019fb000-0000-7000-8000-000000000005', title: 'Fresh Notes', status: 'draft' }

const OLD_NOTES: Post = {
	id: TRASHED.id,
	type: 'post',
	parentId: null,
	path: TRASHED.slug,
	slug: TRASHED.slug,
	title: 'Old Notes',
	status: 'trash',
	excerpt: '',
	authorId: TRASHED.author_id,
	authorName: 'Maria Perez',
	publishedAt: null,
	createdAt: TRASHED.created_at,
	updatedAt: TRASHED.updated_at,
}

const POST_TYPE = {
	key: 'post',
	singular_label: 'Post',
	plural_label: 'Posts',
	description: '',
	route_word: '',
	hierarchical: false,
	revisions: true,
	revision_cap: 100,
	page_kind: 'single',
	default: true,
	active: true,
	created_at: '2026-08-01T10:00:00Z',
	updated_at: '2026-08-01T10:00:00Z',
	fields: [],
}

const PAGE_TYPE = {
	...POST_TYPE,
	key: 'page',
	singular_label: 'Page',
	plural_label: 'Pages',
	route_word: 'pages',
	default: false,
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
 * Serves one post in the trash that leaves the listing once restored or deleted for good.
 */
function serveLeaving() {
	let left = false
	server.use(
		http.get('/api/content', ({ request }) => {
			listed.push(new URL(request.url).searchParams.get('status') ?? '')
			return HttpResponse.json(left ? { items: [], total: 0 } : { items: [TRASHED], total: 1 })
		}),
		http.post('/api/content/:id/restore', () => {
			left = true
			return HttpResponse.json({ ...TRASHED, status: 'draft' })
		}),
		http.delete('/api/content/:id', () => {
			left = true
			return new HttpResponse(null, { status: 204 })
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
 * Trashes the only listed post through its row actions and waits for the toast.
 */
async function trashTheListedPost() {
	await userEvent.click(await screen.findByRole('button', { name: 'Actions' }))
	await userEvent.click(await screen.findByRole('menuitem', { name: 'Trash' }))
	await userEvent.click(within(await screen.findByRole('dialog')).getByRole('button', { name: 'Trash' }))
	await screen.findByText('"Old Notes" moved to the trash.', { selector: '.godmin-toast' })
}

/**
 * Serves two posts in the trash, recording each restore and each permanent delete.
 * @param refused - The post whose calls the server refuses, none to accept every call.
 * @param held - What each restore waits on before the server answers it.
 */
function serveTwoTrashed(refused?: string, held: Promise<void> = Promise.resolve()) {
	const second = { ...TRASHED, id: '019fb000-0000-7000-8000-000000000004', title: 'Older Notes' }
	server.use(
		http.get('/api/content', ({ request }) => {
			listed.push(new URL(request.url).searchParams.get('status') ?? '')
			return HttpResponse.json({ items: [TRASHED, second], total: 2 })
		}),
		http.get('/api/content/counts', () =>
			HttpResponse.json({ draft: 0, pending: 0, private: 0, published: 0, trash: 2 }),
		),
		http.post('/api/content/:id/restore', async ({ params }) => {
			await held
			if (String(params.id) === refused) {
				return HttpResponse.json({}, { status: 500 })
			}
			restored.push(String(params.id))
			return HttpResponse.json({ ...TRASHED, id: String(params.id), status: 'draft' })
		}),
		http.delete('/api/content/:id', ({ request }) => {
			deleted.push(new URL(request.url))
			return new HttpResponse(null, { status: 204 })
		}),
	)
}

/**
 * Opens the trash view holding two posts and ticks both.
 */
async function tickBothTrashed() {
	await userEvent.click(await screen.findByRole('button', { name: 'Trash (2)' }))
	await screen.findByText('Older Notes')
	await userEvent.click(screen.getByRole('checkbox', { name: 'Old Notes' }))
	await userEvent.click(screen.getByRole('checkbox', { name: 'Older Notes' }))
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

test('swaps the row actions in the trash view, the restore first', async () => {
	renderAt('/content/post')

	await openTrashedRowActions()

	await screen.findByRole('menuitem', { name: 'Restore' })
	expect(screen.getAllByRole('menuitem').map((item) => item.textContent)).toEqual(['Restore', 'Permanently delete'])
	expect(screen.queryByRole('menuitem', { name: 'Trash' })).not.toBeInTheDocument()
	expect(screen.queryByRole('menuitem', { name: 'Edit' })).not.toBeInTheDocument()
})

test('restores a post out of the trash', async () => {
	renderAt('/content/post')
	await openTrashedRowActions()

	await userEvent.click(await screen.findByRole('menuitem', { name: 'Restore' }))

	await waitFor(() => expect(restored).toEqual([TRASHED.id]))
})

test('names the post it restored in a toast', async () => {
	renderAt('/content/post')
	await openTrashedRowActions()

	await userEvent.click(await screen.findByRole('menuitem', { name: 'Restore' }))

	expect(await screen.findByText('"Old Notes" has been restored.', { selector: '.godmin-toast' })).toBeInTheDocument()
})

test('moves the focus to the list once a restore finishes', async () => {
	serveLeaving()
	renderAt('/content/post')
	await openTrashedRowActions()

	await userEvent.click(await screen.findByRole('menuitem', { name: 'Restore' }))

	await screen.findByText('"Old Notes" has been restored.', { selector: '.godmin-toast' })
	await waitFor(() => expect(screen.getByRole('region', { name: 'Posts' })).toHaveFocus())
})

test('restores every ticked post at once and counts them', async () => {
	serveTwoTrashed()
	renderAt('/content/post')
	await tickBothTrashed()

	await userEvent.click(screen.getByRole('button', { name: 'Restore' }))

	expect(await screen.findByText('2 posts have been restored.', { selector: '.godmin-toast' })).toBeInTheDocument()
	expect(restored).toEqual(expect.arrayContaining([TRASHED.id, '019fb000-0000-7000-8000-000000000004']))
})

test('counts what a partly refused bulk restore brought back and what it could not', async () => {
	vi.spyOn(console, 'error').mockImplementation(() => {})
	serveTwoTrashed('019fb000-0000-7000-8000-000000000004')
	renderAt('/content/post')
	await tickBothTrashed()

	await userEvent.click(screen.getByRole('button', { name: 'Restore' }))

	expect(await screen.findByText('1 post has been restored.', { selector: '.godmin-toast' })).toBeInTheDocument()
	expect(await screen.findByText('1 item could not be restored.')).toBeInTheDocument()
})

test('shows the bulk restore as busy until it settles', async () => {
	const answer = gate()
	serveTwoTrashed(undefined, answer.held)
	renderAt('/content/post')
	await tickBothTrashed()

	await userEvent.click(screen.getByRole('button', { name: 'Restore' }))

	expect(screen.getByRole('button', { name: 'Restore' })).toHaveAttribute('aria-disabled', 'true')
	answer.release()
	await screen.findByText('2 posts have been restored.', { selector: '.godmin-toast' })
	expect(screen.getByRole('button', { name: 'Restore' })).not.toHaveAttribute('aria-disabled')
})

/**
 * Starts a bulk restore the server partly refuses and holds, then moves the list to All and ticks a post there.
 * @param held - What each restore waits on before the server answers it.
 */
async function tickInAllWhileRestoring(held: Promise<void>) {
	serveTwoTrashed(SECOND_ID, held)
	server.use(
		http.get('/api/content', ({ request }) => {
			const status = new URL(request.url).searchParams.get('status') ?? ''
			listed.push(status)
			const items = status === 'trash' ? [TRASHED, { ...TRASHED, id: SECOND_ID, title: 'Older Notes' }] : [FRESH]
			return HttpResponse.json({ items, total: items.length })
		}),
	)
	renderAt('/content/post')
	await tickBothTrashed()
	await userEvent.click(screen.getByRole('button', { name: 'Restore' }))
	await userEvent.click(screen.getByRole('button', { name: /^All/ }))
	await waitFor(() => expect(listed).toContain(''))
	await userEvent.click(await screen.findByRole('checkbox', { name: 'Fresh Notes' }))
}

test('keeps a failure off the status the list moved to before the run settled', async () => {
	vi.spyOn(console, 'error').mockImplementation(() => {})
	const answer = gate()
	await tickInAllWhileRestoring(answer.held)

	answer.release()

	await screen.findByText('1 post has been restored.', { selector: '.godmin-toast' })
	expect(screen.queryByText('1 item could not be restored.')).not.toBeInTheDocument()
	expect(screen.getByRole('checkbox', { name: 'Fresh Notes' })).toBeChecked()
})

test('leaves the focus alone on the status the list moved to before the run settled', async () => {
	vi.spyOn(console, 'error').mockImplementation(() => {})
	const answer = gate()
	await tickInAllWhileRestoring(answer.held)

	answer.release()

	await screen.findByText('1 post has been restored.', { selector: '.godmin-toast' })
	expect(screen.getByRole('checkbox', { name: 'Fresh Notes' })).toHaveFocus()
})

/**
 * Serves two posts in the trash, holding the first two restores on the gate and refusing the restores asked after.
 * @param held - What the first two restores wait on before the server answers them.
 */
function serveTwoTrashedRefusingLater(held: Promise<void>) {
	serveTwoTrashed(undefined, held)
	let asked = 0
	server.use(
		http.post('/api/content/:id/restore', async ({ params }) => {
			asked += 1
			if (asked > 2) {
				return HttpResponse.json({}, { status: 500 })
			}
			await held
			restored.push(String(params.id))
			return HttpResponse.json({ ...TRASHED, id: String(params.id), status: 'draft' })
		}),
	)
}

test('keeps the newer failure and the focus once a run from an earlier visit of the status settles', async () => {
	vi.spyOn(console, 'error').mockImplementation(() => {})
	const answer = gate()
	serveTwoTrashedRefusingLater(answer.held)
	renderAt('/content/post')
	await tickBothTrashed()
	await userEvent.click(screen.getByRole('button', { name: 'Restore' }))
	await userEvent.click(screen.getByRole('button', { name: /^All/ }))
	await waitFor(() => expect(listed).toContain(''))
	await userEvent.click(screen.getByRole('button', { name: 'Trash (2)' }))
	await screen.findByText('Older Notes')
	expect(screen.getByRole('checkbox', { name: 'Old Notes' })).not.toBeChecked()
	await userEvent.click(screen.getAllByRole('button', { name: 'Actions' })[0])
	await userEvent.click(await screen.findByRole('menuitem', { name: 'Restore' }))
	await screen.findByText('The item could not be restored.')
	await waitFor(() => expect(screen.getAllByRole('button', { name: 'Actions' })[0]).toHaveFocus())

	answer.release()

	await screen.findByText('2 posts have been restored.', { selector: '.godmin-toast' })
	expect(screen.getByText('The item could not be restored.')).toBeInTheDocument()
	expect(screen.getAllByRole('button', { name: 'Actions' })[0]).toHaveFocus()
	expect(screen.getByRole('region', { name: 'Posts' })).not.toHaveFocus()
})

test('keeps a failure off the type the list moved to before the run settled', async () => {
	vi.spyOn(console, 'error').mockImplementation(() => {})
	const answer = gate()
	serveTwoTrashed('019fb000-0000-7000-8000-000000000004', answer.held)
	server.use(http.get('/api/types', () => HttpResponse.json({ items: [POST_TYPE, PAGE_TYPE] })))
	const { router } = renderRoutedAt('/content/post')
	await tickBothTrashed()
	await userEvent.click(screen.getByRole('button', { name: 'Restore' }))

	await act(async () => {
		await router.navigate({ to: '/content/$typeKey', params: { typeKey: 'page' } })
	})
	await screen.findByRole('heading', { name: 'Pages', level: 1 })
	answer.release()

	await screen.findByText('1 post has been restored.', { selector: '.godmin-toast' })
	expect(screen.queryByText('1 item could not be restored.')).not.toBeInTheDocument()
})

test('counts the pages a bulk restore brought back as pages', async () => {
	serveTwoTrashed()
	server.use(http.get('/api/types', () => HttpResponse.json({ items: [POST_TYPE, PAGE_TYPE] })))
	renderAt('/content/page')
	await tickBothTrashed()

	await userEvent.click(screen.getByRole('button', { name: 'Restore' }))

	expect(await screen.findByText('2 pages have been restored.', { selector: '.godmin-toast' })).toBeInTheDocument()
})

test('deletes every ticked post for good behind one confirm and counts them', async () => {
	serveTwoTrashed()
	renderAt('/content/post')
	await tickBothTrashed()

	await userEvent.click(screen.getByRole('button', { name: 'Permanently delete' }))
	const dialog = await screen.findByRole('dialog', { name: 'Permanently delete' })
	expect(dialog).toHaveTextContent('Delete 2 items for good? This cannot be undone.')
	await userEvent.click(within(dialog).getByRole('button', { name: 'Permanently delete' }))

	expect(await screen.findByText('2 items permanently deleted.', { selector: '.godmin-toast' })).toBeInTheDocument()
	expect(deleted).toHaveLength(2)
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

test('opens a post deleted for good as a missing post', async () => {
	serveMovable('trash')
	const { router } = renderRoutedAt(EDITOR_PATH)
	await screen.findByText(/This item is in the trash/)
	await goToList(router)
	await openTrashedRowActions()
	await userEvent.click(await screen.findByRole('menuitem', { name: 'Permanently delete' }))
	await userEvent.click(within(await screen.findByRole('dialog')).getByRole('button', { name: 'Permanently delete' }))
	await waitFor(() => expect(deleted).toHaveLength(1))

	await goToEditor(router)

	expect(await screen.findByText('Could not load that post.')).toBeInTheDocument()
})

test('opens a post removed by emptying the trash as a missing post', async () => {
	serveMovable('trash')
	let emptied = false
	server.use(
		http.delete('/api/content/trash', () => {
			emptied = true
			return HttpResponse.json({ deleted: 1, kept: 0 })
		}),
		http.get(`/api/content/${TRASHED.id}`, () =>
			emptied ? HttpResponse.json({}, { status: 404 }) : HttpResponse.json({ ...storedPost, ...TRASHED }),
		),
	)
	const { router } = renderRoutedAt(EDITOR_PATH)
	await screen.findByText(/This item is in the trash/)
	await goToList(router)
	await userEvent.click(await screen.findByRole('button', { name: 'Trash (1)' }))
	await userEvent.click(await screen.findByRole('button', { name: 'Empty Trash' }))
	await userEvent.click(within(await screen.findByRole('dialog')).getByRole('button', { name: 'Empty Trash' }))
	await waitFor(() => expect(emptied).toBe(true))

	await goToEditor(router)

	expect(await screen.findByText('Could not load that post.')).toBeInTheDocument()
})

test('asks to confirm before deleting permanently, headed by the action', async () => {
	renderAt('/content/post')
	await openTrashedRowActions()

	await userEvent.click(await screen.findByRole('menuitem', { name: 'Permanently delete' }))

	const dialog = await screen.findByRole('dialog', { name: 'Permanently delete' })
	expect(dialog).toHaveTextContent('Delete "Old Notes" for good? This cannot be undone.')
	expect(within(dialog).getByRole('heading', { name: 'Permanently delete' })).toBeInTheDocument()
	expect(deleted).toEqual([])
})

test('cuts a long title in the permanent delete confirm at the length the site serves', async () => {
	server.use(http.get('/api/settings', () => HttpResponse.json({ ...siteSettings, toast_name_length: 5 })))
	renderAt('/content/post')
	await openTrashedRowActions()

	await userEvent.click(await screen.findByRole('menuitem', { name: 'Permanently delete' }))

	expect(await screen.findByRole('dialog')).toHaveTextContent('Delete "Old N…" for good? This cannot be undone.')
})

test('asks the Spanish permanent delete in the words of its title, button and toast', async () => {
	setLocaleData(await catalogFor('es-ES'), 'gophenberg')
	onTestFinished(() => resetLocaleData({}, 'gophenberg'))
	const named = (title: string) => title

	expect(deleteQuestion([OLD_NOTES], named)).toBe('¿Borrar permanentemente «Old Notes»? Esto no se puede deshacer.')
	expect(deleteQuestion([OLD_NOTES, { ...OLD_NOTES, id: SECOND_ID }], named))
		.toBe('¿Borrar permanentemente 2 elementos? Esto no se puede deshacer.')
})

test('deletes for good once confirmed', async () => {
	renderAt('/content/post')
	await openTrashedRowActions()
	await userEvent.click(await screen.findByRole('menuitem', { name: 'Permanently delete' }))

	await userEvent.click(within(await screen.findByRole('dialog')).getByRole('button', { name: 'Permanently delete' }))

	await waitFor(() => expect(deleted).toHaveLength(1))
	expect(deleted[0].pathname).toBe(`/api/content/${TRASHED.id}`)
	expect(deleted[0].searchParams.get('force')).toBe('true')
})

test('names the post it deleted for good in a toast', async () => {
	renderAt('/content/post')
	await openTrashedRowActions()
	await userEvent.click(await screen.findByRole('menuitem', { name: 'Permanently delete' }))

	await userEvent.click(within(await screen.findByRole('dialog')).getByRole('button', { name: 'Permanently delete' }))

	expect(await screen.findByText('"Old Notes" permanently deleted.', { selector: '.godmin-toast' })).toBeInTheDocument()
})

test('moves the focus to the list once a permanent delete finishes', async () => {
	serveLeaving()
	renderAt('/content/post')
	await openTrashedRowActions()
	await userEvent.click(await screen.findByRole('menuitem', { name: 'Permanently delete' }))

	await userEvent.click(within(await screen.findByRole('dialog')).getByRole('button', { name: 'Permanently delete' }))

	await screen.findByText('"Old Notes" permanently deleted.', { selector: '.godmin-toast' })
	await waitFor(() => expect(screen.getByRole('region', { name: 'Posts' })).toHaveFocus())
})

test('keeps the post when the permanent delete is dismissed', async () => {
	renderAt('/content/post')
	await openTrashedRowActions()
	await userEvent.click(await screen.findByRole('menuitem', { name: 'Permanently delete' }))

	await userEvent.click(await screen.findByRole('button', { name: 'Cancel' }))

	await waitFor(() => expect(screen.queryByRole('dialog')).not.toBeInTheDocument())
	expect(deleted).toEqual([])
})

test('reports a restore the server refused above the list', async () => {
	vi.spyOn(console, 'error').mockImplementation(() => {})
	server.use(http.post('/api/content/:id/restore', () => HttpResponse.json({}, { status: 500 })))
	renderAt('/content/post')
	await openTrashedRowActions()

	await userEvent.click(await screen.findByRole('menuitem', { name: 'Restore' }))

	expect(await screen.findByText('The item could not be restored.')).toBeInTheDocument()
})

test('reports a permanent delete the server refused above the list', async () => {
	vi.spyOn(console, 'error').mockImplementation(() => {})
	server.use(http.delete('/api/content/:id', () => HttpResponse.json({}, { status: 500 })))
	renderAt('/content/post')
	await openTrashedRowActions()
	await userEvent.click(await screen.findByRole('menuitem', { name: 'Permanently delete' }))

	await userEvent.click(within(await screen.findByRole('dialog')).getByRole('button', { name: 'Permanently delete' }))

	expect(await screen.findByText('The item could not be permanently deleted.')).toBeInTheDocument()
	expect(screen.queryByRole('dialog')).not.toBeInTheDocument()
})

test('counts the posts a partly refused bulk permanent delete could not remove', async () => {
	vi.spyOn(console, 'error').mockImplementation(() => {})
	serveTwoTrashed()
	server.use(
		http.delete('/api/content/:id', ({ params, request }) => {
			if (String(params.id) === TRASHED.id) {
				return HttpResponse.json({}, { status: 500 })
			}
			deleted.push(new URL(request.url))
			return new HttpResponse(null, { status: 204 })
		}),
	)
	renderAt('/content/post')
	await tickBothTrashed()
	await userEvent.click(screen.getByRole('button', { name: 'Permanently delete' }))

	await userEvent.click(within(await screen.findByRole('dialog')).getByRole('button', { name: 'Permanently delete' }))

	expect(await screen.findByText('1 item permanently deleted.', { selector: '.godmin-toast' })).toBeInTheDocument()
	expect(await screen.findByText('1 item could not be permanently deleted.')).toBeInTheDocument()
})
