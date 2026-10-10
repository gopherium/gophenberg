// SPDX-License-Identifier: Apache-2.0

import { http, HttpResponse, server } from '@gophenberg/frontend-sdk/testing'
import { act, screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { resetLocaleData, setLocaleData } from '@wordpress/i18n'
import { beforeAll, beforeEach, expect, onTestFinished, test, vi } from 'vitest'

import { catalogFor } from '../i18n/catalog'
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

const SPARE = { ...TRASHED, id: '019fb000-0000-7000-8000-000000000007', title: 'Spare Draft' }

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
const asked: URLSearchParams[] = []
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
	asked.length = 0
	emptied.length = 0
	deletedOneByOne.length = 0
	server.use(
		http.get('/api/content', ({ request }) => {
			const query = new URL(request.url).searchParams
			asked.push(query)
			const search = (query.get('search') ?? '').toLowerCase()
			const trashed = query.get('status') === 'trash' ? bin : []
			const matching = trashed.filter((post) => post.title.toLowerCase().includes(search))
			const perPage = Number(query.get('per_page') ?? matching.length)
			return HttpResponse.json({ items: matching.slice(0, perPage), total: matching.length })
		}),
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
 * Picks the Trash tab of the status filter once the list shows it.
 */
async function pickTrashTab() {
	const tabs = await screen.findByRole('navigation', { name: 'Filter by status' })
	await userEvent.click(within(tabs).getByRole('link', { name: 'Trash' }))
}

/**
 * Switches the list to the trash view.
 */
async function openTrashView() {
	await pickTrashTab()
	await screen.findByText('Old Notes')
}

/**
 * Opens the empty trash confirm from the trash view and returns it.
 * @returns The confirm dialog.
 */
async function openEmptyTrash(): Promise<HTMLElement> {
	await openTrashView()
	await userEvent.click(screen.getByRole('button', { name: 'Empty Trash' }))
	return screen.findByRole('alertdialog')
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

	await pickTrashTab()

	expect(await screen.findByText('No items found.')).toBeInTheDocument()
	expect(screen.queryByRole('button', { name: 'Empty Trash' })).not.toBeInTheDocument()
})

test('asks in an alert dialog titled with the question and a red Empty trash, before emptying the trash', async () => {
	renderAt('/content/post')

	const dialog = await openEmptyTrash()

	expect(dialog).toHaveAccessibleName('Empty the trash?')
	expect(within(dialog).getByRole('heading', { name: 'Empty the trash?', level: 2 }))
		.not.toHaveAttribute('data-visually-hidden')
	expect(within(dialog).queryByRole('button', { name: 'Close' })).not.toBeInTheDocument()
	expect(within(dialog).getByRole('button', { name: 'Empty trash' }).className).toMatch(/__irreversible-action\b/)
	expect(screen.queryByRole('dialog')).not.toBeInTheDocument()
	expect(emptied).toEqual([])
})

test('opens the empty trash confirm on Cancel, as the WordPress design system confirm does', async () => {
	renderAt('/content/post')

	const dialog = await openEmptyTrash()

	await waitFor(() => expect(within(dialog).getByRole('button', { name: 'Cancel' })).toHaveFocus())
})

test('describes the empty trash confirm by its warning, so a screen reader reads it', async () => {
	renderAt('/content/post')

	const dialog = await openEmptyTrash()

	expect(dialog).toHaveAccessibleDescription('The 2 items in the trash are deleted for good. This cannot be undone.')
})

test('names Empty Trash and its warning in WordPress es_ES words, and counts exact millions with "de"', async () => {
	serveEmptied({ deleted: 0, kept: 1000000 })
	renderAt('/content/post')
	await openTrashView()
	setLocaleData(await catalogFor('es-ES'), 'gophenberg')
	onTestFinished(() => resetLocaleData({}, 'gophenberg'))

	await userEvent.click(screen.getByRole('button', { name: 'Empty Trash' }))
	const dialog = await screen.findByRole('alertdialog', { name: '¿Vaciar la papelera?' })
	expect(dialog).toHaveTextContent(
		'Los 2 elementos de la papelera se borran permanentemente. Esto no se puede deshacer.',
	)
	await userEvent.click(within(dialog).getByRole('button', { name: 'Vaciar papelera' }))

	expect(await screen.findByText('1.000.000 de elementos siguen en la papelera.', { selector: '.godmin-toast' }))
		.toBeInTheDocument()
})

test('counts the items the trash holds in the warning', async () => {
	renderAt('/content/post')

	const dialog = await openEmptyTrash()

	expect(dialog).toHaveTextContent('The 2 items in the trash are deleted for good. This cannot be undone.')
})

test('counts every trashed item of the type in the warning while a search narrows the list', async () => {
	bin = [TRASHED, SECOND, SPARE]
	renderAt('/content/post?status=trash&search=spare')
	await screen.findByText('Spare Draft')
	expect(screen.queryByText('Old Notes')).not.toBeInTheDocument()

	await userEvent.click(screen.getByRole('button', { name: 'Empty Trash' }))

	const dialog = await screen.findByRole('alertdialog')
	expect(dialog).toHaveTextContent('The 3 items in the trash are deleted for good. This cannot be undone.')
	const counted = asked.find((query) => query.get('per_page') === '1')
	expect(counted?.get('status')).toBe('trash')
	expect(counted?.has('search')).toBe(false)
})

test('offers Empty Trash only while the narrowed list shows rows, its dialog counting the whole trash', async () => {
	bin = [TRASHED, SECOND, SPARE]
	renderAt('/content/post?status=trash&search=nothing')
	expect(await screen.findByText('No items found.')).toBeInTheDocument()
	expect(screen.queryByRole('button', { name: 'Empty Trash' })).not.toBeInTheDocument()

	const search = screen.getByRole('searchbox')
	await userEvent.clear(search)
	await userEvent.type(search, 'spare')

	await screen.findByText('Spare Draft')
	await userEvent.click(await screen.findByRole('button', { name: 'Empty Trash' }))
	expect(await screen.findByRole('alertdialog')).toHaveTextContent(
		'The 3 items in the trash are deleted for good. This cannot be undone.',
	)
})

test('takes the empty trash control away once the trash empties', async () => {
	renderAt('/content/post')
	const dialog = await openEmptyTrash()

	await userEvent.click(within(dialog).getByRole('button', { name: 'Empty trash' }))

	await screen.findByText('2 items permanently deleted.', { selector: '.godmin-toast' })
	await waitFor(() => expect(screen.queryByRole('button', { name: 'Empty Trash' })).not.toBeInTheDocument())
})

test('counts the one item the trash holds in the singular', async () => {
	bin = [TRASHED]
	renderAt('/content/post')

	const dialog = await openEmptyTrash()

	expect(dialog).toHaveTextContent('The 1 item in the trash is deleted for good. This cannot be undone.')
})

test('shows the emptying on the confirm button and holds both buttons until it settles', async () => {
	const answer = gate()
	server.use(
		http.delete('/api/content/trash', async () => {
			await answer.held
			return HttpResponse.json({ deleted: 2, kept: 0 })
		}),
	)
	renderAt('/content/post')
	const dialog = await openEmptyTrash()

	await userEvent.click(within(dialog).getByRole('button', { name: 'Empty trash' }))

	const confirm = within(dialog).getByRole('button', { name: 'Empty trash' })
	await waitFor(() => expect(confirm.className).toMatch(/__is-loading\b/))
	expect(confirm).toHaveAttribute('aria-disabled', 'true')
	expect(within(dialog).getByRole('button', { name: 'Cancel' })).toHaveAttribute('aria-disabled', 'true')
	answer.release()
	await waitFor(() => expect(screen.queryByRole('alertdialog')).not.toBeInTheDocument())
})

test('empties the trash in one call once confirmed', async () => {
	renderAt('/content/post')
	const dialog = await openEmptyTrash()

	await userEvent.click(within(dialog).getByRole('button', { name: 'Empty trash' }))

	await waitFor(() => expect(emptied).toHaveLength(1))
	expect(emptied[0].searchParams.get('type')).toBe('post')
	expect(deletedOneByOne).toEqual([])
	await waitFor(() => expect(screen.queryByRole('alertdialog')).not.toBeInTheDocument())
})

test('counts the items it deleted for good in a toast', async () => {
	renderAt('/content/post')
	const dialog = await openEmptyTrash()

	await userEvent.click(within(dialog).getByRole('button', { name: 'Empty trash' }))

	expect(await screen.findByText('2 items permanently deleted.', { selector: '.godmin-toast' })).toBeInTheDocument()
	await waitFor(() => expect(screen.queryByText('Old Notes')).not.toBeInTheDocument())
})

test('moves the focus to the list once the trash empties', async () => {
	renderAt('/content/post')
	const dialog = await openEmptyTrash()

	await userEvent.click(within(dialog).getByRole('button', { name: 'Empty trash' }))

	await screen.findByText('2 items permanently deleted.', { selector: '.godmin-toast' })
	await waitFor(() => expect(screen.getByRole('region', { name: 'Posts' })).toHaveFocus())
})

test('counts the items it deleted and the items that stay in one toast', async () => {
	serveEmptied({ deleted: 1, kept: 1 })
	renderAt('/content/post')
	const dialog = await openEmptyTrash()

	await userEvent.click(within(dialog).getByRole('button', { name: 'Empty trash' }))

	expect(
		await screen.findByText('1 item permanently deleted. 1 item stays in the trash.', { selector: '.godmin-toast' }),
	).toBeInTheDocument()
	expect(screen.queryByRole('alert')).not.toBeInTheDocument()
	await waitFor(() => expect(screen.queryByRole('alertdialog')).not.toBeInTheDocument())
})

test('says only what stays when nothing went', async () => {
	serveEmptied({ deleted: 0, kept: 2 })
	renderAt('/content/post')
	const dialog = await openEmptyTrash()

	await userEvent.click(within(dialog).getByRole('button', { name: 'Empty trash' }))

	expect(await screen.findByText('2 items stay in the trash.', { selector: '.godmin-toast' })).toBeInTheDocument()
})

test('returns the focus to the empty trash control when nothing went', async () => {
	serveEmptied({ deleted: 0, kept: 2 })
	renderAt('/content/post')
	const dialog = await openEmptyTrash()

	await userEvent.click(within(dialog).getByRole('button', { name: 'Empty trash' }))

	await screen.findByText('2 items stay in the trash.', { selector: '.godmin-toast' })
	await waitFor(() => expect(screen.getByRole('button', { name: 'Empty Trash' })).toHaveFocus())
})

test('shows no toast when nothing went and nothing stays', async () => {
	serveEmptied({ deleted: 0, kept: 0 })
	renderAt('/content/post')
	const dialog = await openEmptyTrash()

	await userEvent.click(within(dialog).getByRole('button', { name: 'Empty trash' }))

	await waitFor(() => expect(emptied).toHaveLength(1))
	await waitFor(() => expect(screen.queryByRole('alertdialog')).not.toBeInTheDocument())
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
	await userEvent.click(within(await screen.findByRole('alertdialog')).getByRole('button', { name: 'Empty trash' }))

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
	await userEvent.click(within(dialog).getByRole('button', { name: 'Empty trash' }))

	await userEvent.click(within(dialog).getByRole('button', { name: 'Cancel' }))

	expect(dialog).toBeInTheDocument()
	answer.release()
	await waitFor(() => expect(screen.queryByRole('alertdialog')).not.toBeInTheDocument())
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
		http.delete('/api/content/trash', ({ request }) => {
			const type = new URL(request.url).searchParams.get('type') ?? ''
			const deleted = trashes[type].length
			trashes[type] = []
			return HttpResponse.json({ deleted, kept: 0 })
		}),
	)
	renderAt('/content/page')
	await pickTrashTab()
	await screen.findByText('Old Page')
	await userEvent.click(screen.getByRole('button', { name: 'Empty Trash' }))
	const dialog = await screen.findByRole('alertdialog')
	expect(dialog).toHaveTextContent('The 1 item in the trash is deleted for good.')

	await userEvent.click(within(dialog).getByRole('button', { name: 'Empty trash' }))

	await waitFor(() => expect(trashes.page).toEqual([]))
	expect(trashes.post).toEqual([TRASHED])
})

test('keeps the trash when the confirm is dismissed', async () => {
	renderAt('/content/post')
	await openEmptyTrash()

	await userEvent.click(await screen.findByRole('button', { name: 'Cancel' }))

	await waitFor(() => expect(screen.queryByRole('alertdialog')).not.toBeInTheDocument())
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
	await userEvent.click(within(dialog).getByRole('button', { name: 'Empty trash' }))
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
	await userEvent.click(within(dialog).getByRole('button', { name: 'Empty trash' }))
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

test('reports an empty trash the server refused under the buttons, keeping the confirm open', async () => {
	server.use(http.delete('/api/content/trash', () => HttpResponse.json({}, { status: 500 })))
	renderAt('/content/post')
	const dialog = await openEmptyTrash()

	await userEvent.click(within(dialog).getByRole('button', { name: 'Empty trash' }))

	const line = await within(dialog).findByText('The trash could not be emptied.')
	expect(line.className).toMatch(/__error-message\b/)
	expect(line.compareDocumentPosition(within(dialog).getByRole('button', { name: 'Cancel' })))
		.toBe(Node.DOCUMENT_POSITION_PRECEDING)
	expect(within(dialog).getByRole('button', { name: 'Empty trash' })).toHaveAttribute('aria-disabled', 'false')
	expect(bin).toHaveLength(2)
})

test('opens the confirm again without the failure of the last try', async () => {
	server.use(http.delete('/api/content/trash', () => HttpResponse.json({}, { status: 500 })))
	renderAt('/content/post')
	const dialog = await openEmptyTrash()
	await userEvent.click(within(dialog).getByRole('button', { name: 'Empty trash' }))
	await within(dialog).findByText('The trash could not be emptied.')
	await userEvent.click(within(dialog).getByRole('button', { name: 'Cancel' }))
	await waitFor(() => expect(screen.queryByRole('alertdialog')).not.toBeInTheDocument())

	await userEvent.click(screen.getByRole('button', { name: 'Empty Trash' }))

	expect(await screen.findByRole('alertdialog')).not.toHaveTextContent('The trash could not be emptied.')
})
