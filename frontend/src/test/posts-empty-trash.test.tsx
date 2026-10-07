// SPDX-License-Identifier: Apache-2.0

import { http, HttpResponse, server } from '@gophenberg/frontend-sdk/testing'
import { act, screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { beforeAll, beforeEach, expect, test, vi } from 'vitest'

import { gate } from './gate'
import { adminUser, renderAt, renderRoutedAt } from './render'
import { storedPost } from './postFixture'
import { warmPostsScreen } from './warm'

warmPostsScreen()

beforeAll(async () => {
	await import('../content/EditorScreen')
}, 120000)

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

const SECOND = { ...TRASHED, id: '019fb000-0000-7000-8000-000000000004', title: 'Older Notes' }

const TRASHED_PAGE = { ...TRASHED, id: '019fb000-0000-7000-8000-000000000005', type: 'page', title: 'Old Page' }

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

let bin: { id: string, title: string }[] = []
const emptied: URL[] = []
const deletedOneByOne: string[] = []

/**
 * Serves the trash emptying answer with the given counts, in place of emptying the whole bin.
 * @param counts - What the server reports deleted and kept.
 */
function serveEmptied(counts: { deleted: number, kept: number }) {
	server.use(
		http.delete('/api/content/trash', ({ request }) => {
			emptied.push(new URL(request.url))
			return HttpResponse.json(counts)
		}),
	)
}

beforeEach(() => {
	bin = [TRASHED, SECOND]
	emptied.length = 0
	deletedOneByOne.length = 0
	server.use(
		http.get('/api/content', ({ request }) => {
			const query = new URL(request.url).searchParams
			const matching = query.get('status') === 'trash' ? bin : []
			return HttpResponse.json({ items: matching, total: matching.length })
		}),
		http.get('/api/content/counts', () =>
			HttpResponse.json({
				draft: 0,
				pending: 0,
				private: 0,
				published: 0,
				trash: bin.length,
			}),
		),
		http.delete('/api/content/trash', ({ request }) => {
			emptied.push(new URL(request.url))
			const deleted = bin.length
			bin = []
			return HttpResponse.json({ deleted, kept: 0 })
		}),
		http.delete('/api/content/:id', ({ params }) => {
			deletedOneByOne.push(String(params.id))
			return new HttpResponse(null, { status: 204 })
		}),
	)
})

/**
 * Switches the list to the trash view.
 */
async function openTrashView() {
	await userEvent.click(await screen.findByRole('button', { name: 'Trash (2)' }))
	await screen.findByText('Old Notes')
}

/**
 * Opens the empty trash confirm from the trash view and returns it.
 * @returns The confirm dialog.
 */
async function openEmptyTrash(): Promise<HTMLElement> {
	await openTrashView()
	await userEvent.click(screen.getByRole('button', { name: 'Empty Trash' }))
	return screen.findByRole('dialog')
}

test('offers to empty the trash only in the trash view', async () => {
	server.use(http.get('/api/content', () => HttpResponse.json({ items: bin, total: bin.length })))
	renderAt('/content/post')
	await screen.findByText('Old Notes')

	expect(screen.queryByRole('button', { name: 'Empty Trash' })).not.toBeInTheDocument()
	await openTrashView()

	expect(screen.getByRole('button', { name: 'Empty Trash' })).toBeInTheDocument()
})

test('offers no empty trash to a role that may change only its own work', async () => {
	renderAt('/content/post', { ...adminUser, role: 'author' })

	await openTrashView()

	expect(screen.getByText('Older Notes')).toBeInTheDocument()
	expect(screen.queryByRole('button', { name: 'Empty Trash' })).not.toBeInTheDocument()
})

test('hides the empty trash control while the trash view lists nothing', async () => {
	bin = []
	renderAt('/content/post')

	await userEvent.click(await screen.findByRole('button', { name: 'Trash (0)' }))

	expect(await screen.findByText('No results')).toBeInTheDocument()
	expect(screen.queryByRole('button', { name: 'Empty Trash' })).not.toBeInTheDocument()
})

test('asks to confirm in a dialog headed by the action before emptying the trash', async () => {
	renderAt('/content/post')

	const dialog = await openEmptyTrash()

	expect(dialog).toHaveAccessibleName('Empty Trash')
	expect(within(dialog).getByRole('heading', { name: 'Empty Trash' })).toBeInTheDocument()
	expect(within(dialog).getByRole('button', { name: 'Empty Trash' })).toBeInTheDocument()
	expect(screen.queryByRole('alertdialog')).not.toBeInTheDocument()
	expect(emptied).toEqual([])
})

test('warns that every item in the trash of the type on screen goes for good', async () => {
	renderAt('/content/post')

	const dialog = await openEmptyTrash()

	expect(dialog).toHaveTextContent('Every item in the posts trash is removed for good. This cannot be undone.')
})

test('empties the trash in one call once confirmed', async () => {
	renderAt('/content/post')
	const dialog = await openEmptyTrash()

	await userEvent.click(within(dialog).getByRole('button', { name: 'Empty Trash' }))

	await waitFor(() => expect(emptied).toHaveLength(1))
	expect(emptied[0].searchParams.get('type')).toBe('post')
	expect(deletedOneByOne).toEqual([])
	await waitFor(() => expect(screen.queryByRole('dialog')).not.toBeInTheDocument())
})

test('counts the items it deleted for good in a toast', async () => {
	renderAt('/content/post')
	const dialog = await openEmptyTrash()

	await userEvent.click(within(dialog).getByRole('button', { name: 'Empty Trash' }))

	expect(await screen.findByText('2 items permanently deleted.', { selector: '.godmin-toast' })).toBeInTheDocument()
	await waitFor(() => expect(screen.queryByText('Old Notes')).not.toBeInTheDocument())
})

test('moves the focus to the list once the trash empties', async () => {
	renderAt('/content/post')
	const dialog = await openEmptyTrash()

	await userEvent.click(within(dialog).getByRole('button', { name: 'Empty Trash' }))

	await screen.findByText('2 items permanently deleted.', { selector: '.godmin-toast' })
	await waitFor(() => expect(screen.getByRole('region', { name: 'Posts' })).toHaveFocus())
})

test('counts the items it deleted and the items that stay in one toast', async () => {
	serveEmptied({ deleted: 1, kept: 1 })
	renderAt('/content/post')
	const dialog = await openEmptyTrash()

	await userEvent.click(within(dialog).getByRole('button', { name: 'Empty Trash' }))

	expect(
		await screen.findByText('1 item permanently deleted. 1 item stays in the trash.', { selector: '.godmin-toast' }),
	).toBeInTheDocument()
	expect(screen.queryByRole('alert')).not.toBeInTheDocument()
	await waitFor(() => expect(screen.queryByRole('dialog')).not.toBeInTheDocument())
})

test('says only what stays when nothing went', async () => {
	serveEmptied({ deleted: 0, kept: 2 })
	renderAt('/content/post')
	const dialog = await openEmptyTrash()

	await userEvent.click(within(dialog).getByRole('button', { name: 'Empty Trash' }))

	expect(await screen.findByText('2 items stay in the trash.', { selector: '.godmin-toast' })).toBeInTheDocument()
})

test('returns the focus to the empty trash control when nothing went', async () => {
	serveEmptied({ deleted: 0, kept: 2 })
	renderAt('/content/post')
	const dialog = await openEmptyTrash()

	await userEvent.click(within(dialog).getByRole('button', { name: 'Empty Trash' }))

	await screen.findByText('2 items stay in the trash.', { selector: '.godmin-toast' })
	await waitFor(() => expect(screen.getByRole('button', { name: 'Empty Trash' })).toHaveFocus())
})

test('shows no toast when nothing went and nothing stays', async () => {
	serveEmptied({ deleted: 0, kept: 0 })
	renderAt('/content/post')
	const dialog = await openEmptyTrash()

	await userEvent.click(within(dialog).getByRole('button', { name: 'Empty Trash' }))

	await waitFor(() => expect(emptied).toHaveLength(1))
	await waitFor(() => expect(screen.queryByRole('dialog')).not.toBeInTheDocument())
	expect(document.querySelector('.godmin-toast')).toBeNull()
})

test('clears the failure above the list once the trash empties', async () => {
	vi.spyOn(console, 'error').mockImplementation(() => {})
	server.use(http.post('/api/content/:id/restore', () => HttpResponse.json({}, { status: 500 })))
	renderAt('/content/post')
	await openTrashView()
	const first = screen.getAllByRole('row').find((row) => within(row).queryByText('Old Notes') !== null)
	await userEvent.click(within(first as HTMLElement).getByRole('button', { name: 'Actions' }))
	await userEvent.click(await screen.findByRole('menuitem', { name: 'Restore' }))
	await screen.findByText('The item could not be restored.')

	await userEvent.click(screen.getByRole('button', { name: 'Empty Trash' }))
	await userEvent.click(within(await screen.findByRole('dialog')).getByRole('button', { name: 'Empty Trash' }))

	await screen.findByText('2 items permanently deleted.', { selector: '.godmin-toast' })
	expect(screen.queryByText('The item could not be restored.')).not.toBeInTheDocument()
})

test('keeps the confirm open when Cancel is pressed while the trash empties', async () => {
	const answer = gate()
	server.use(
		http.delete('/api/content/trash', async () => {
			await answer.held
			return HttpResponse.json({ deleted: 2, kept: 0 })
		}),
	)
	renderAt('/content/post')
	const dialog = await openEmptyTrash()
	await userEvent.click(within(dialog).getByRole('button', { name: 'Empty Trash' }))

	await userEvent.click(within(dialog).getByRole('button', { name: 'Cancel' }))

	expect(dialog).toBeInTheDocument()
	answer.release()
	await waitFor(() => expect(screen.queryByRole('dialog')).not.toBeInTheDocument())
})

test('empties only the trash of the type on screen', async () => {
	const trashes: Record<string, { id: string, title: string }[]> = { post: [TRASHED], page: [TRASHED_PAGE] }
	server.use(
		http.get('/api/types', () => HttpResponse.json({ items: [POST_TYPE, PAGE_TYPE] })),
		http.get('/api/content', ({ request }) => {
			const query = new URL(request.url).searchParams
			const matching = query.get('status') === 'trash' ? trashes[query.get('type') ?? 'post'] : []
			return HttpResponse.json({ items: matching, total: matching.length })
		}),
		http.get('/api/content/counts', ({ request }) => {
			const held = trashes[new URL(request.url).searchParams.get('type') ?? 'post']
			return HttpResponse.json({ draft: 0, pending: 0, private: 0, published: 0, trash: held.length })
		}),
		http.delete('/api/content/trash', ({ request }) => {
			const type = new URL(request.url).searchParams.get('type') ?? ''
			const deleted = trashes[type].length
			trashes[type] = []
			return HttpResponse.json({ deleted, kept: 0 })
		}),
	)
	renderAt('/content/page')
	await userEvent.click(await screen.findByRole('button', { name: 'Trash (1)' }))
	await screen.findByText('Old Page')
	await userEvent.click(screen.getByRole('button', { name: 'Empty Trash' }))
	const dialog = await screen.findByRole('dialog')
	expect(dialog).toHaveTextContent('Every item in the pages trash is removed for good.')

	await userEvent.click(within(dialog).getByRole('button', { name: 'Empty Trash' }))

	await waitFor(() => expect(trashes.page).toEqual([]))
	expect(trashes.post).toEqual([TRASHED])
})

test('keeps the trash when the confirm is dismissed', async () => {
	renderAt('/content/post')
	await openEmptyTrash()

	await userEvent.click(await screen.findByRole('button', { name: 'Cancel' }))

	await waitFor(() => expect(screen.queryByRole('dialog')).not.toBeInTheDocument())
	expect(emptied).toEqual([])
})

test('forgets the cached copy of every item of the type it emptied', async () => {
	server.use(
		http.get(`/api/content/${TRASHED.id}`, () =>
			bin.some((post) => post.id === TRASHED.id)
				? HttpResponse.json({ ...storedPost, ...TRASHED })
				: HttpResponse.json({}, { status: 404 }),
		),
	)
	const { router } = renderRoutedAt(`/content/post/${TRASHED.id}/edit`)
	await screen.findByText(/This item is in the trash/)
	await act(async () => {
		await router.navigate({ to: '/content/$typeKey', params: { typeKey: 'post' } })
	})
	const dialog = await openEmptyTrash()
	await userEvent.click(within(dialog).getByRole('button', { name: 'Empty Trash' }))
	await screen.findByText('2 items permanently deleted.', { selector: '.godmin-toast' })

	await act(async () => {
		await router.navigate({
			to: '/content/$typeKey/$postId/edit',
			params: { typeKey: 'post', postId: TRASHED.id },
		})
	})

	expect(await screen.findByText('Could not load that post.')).toBeInTheDocument()
})

test('keeps the cached copy of an item of another type and of an item that never loaded', async () => {
	const missing = '019fb000-0000-7000-8000-000000000006'
	const later = gate()
	let pageReads = 0
	server.use(
		http.get('/api/types', () => HttpResponse.json({ items: [POST_TYPE, PAGE_TYPE] })),
		http.get(`/api/content/${TRASHED_PAGE.id}`, async () => {
			pageReads += 1
			if (pageReads > 1) {
				await later.held
			}
			return HttpResponse.json({ ...storedPost, ...TRASHED_PAGE })
		}),
		http.get(`/api/content/${missing}`, () => HttpResponse.json({}, { status: 404 })),
	)
	const { router } = renderRoutedAt(`/content/page/${TRASHED_PAGE.id}/edit`)
	await screen.findByText(/This item is in the trash/)
	await act(async () => {
		await router.navigate({ to: '/content/$typeKey/$postId/edit', params: { typeKey: 'post', postId: missing } })
	})
	await screen.findByText('Could not load that post.')
	await act(async () => {
		await router.navigate({ to: '/content/$typeKey', params: { typeKey: 'post' } })
	})
	const dialog = await openEmptyTrash()
	await userEvent.click(within(dialog).getByRole('button', { name: 'Empty Trash' }))
	await screen.findByText('2 items permanently deleted.', { selector: '.godmin-toast' })

	await act(async () => {
		await router.navigate({
			to: '/content/$typeKey/$postId/edit',
			params: { typeKey: 'page', postId: TRASHED_PAGE.id },
		})
	})

	expect(await screen.findByText(/This item is in the trash/)).toBeInTheDocument()
	later.release()
})

test('reports an empty trash the server refused inside the confirm', async () => {
	vi.spyOn(console, 'error').mockImplementation(() => {})
	server.use(http.delete('/api/content/trash', () => HttpResponse.json({}, { status: 500 })))
	renderAt('/content/post')
	const dialog = await openEmptyTrash()

	await userEvent.click(within(dialog).getByRole('button', { name: 'Empty Trash' }))

	expect(await within(dialog).findByText('The trash could not be emptied.')).toBeInTheDocument()
	expect(bin).toHaveLength(2)
})

test('opens the confirm again without the failure of the last try', async () => {
	vi.spyOn(console, 'error').mockImplementation(() => {})
	server.use(http.delete('/api/content/trash', () => HttpResponse.json({}, { status: 500 })))
	renderAt('/content/post')
	const dialog = await openEmptyTrash()
	await userEvent.click(within(dialog).getByRole('button', { name: 'Empty Trash' }))
	await within(dialog).findByText('The trash could not be emptied.')
	await userEvent.click(within(dialog).getByRole('button', { name: 'Cancel' }))
	await waitFor(() => expect(screen.queryByRole('dialog')).not.toBeInTheDocument())

	await userEvent.click(screen.getByRole('button', { name: 'Empty Trash' }))

	expect(await screen.findByRole('dialog')).not.toHaveTextContent('The trash could not be emptied.')
})
