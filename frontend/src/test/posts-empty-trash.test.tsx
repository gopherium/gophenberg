// SPDX-License-Identifier: Apache-2.0

import { http, HttpResponse, server } from '@gophenberg/frontend-sdk/testing'
import { act, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { beforeAll, beforeEach, expect, test, vi } from 'vitest'

import { renderAt, renderRoutedAt } from './render'
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

let bin: { id: string, title: string }[] = []
const deleted: URL[] = []

beforeEach(() => {
	bin = [TRASHED, SECOND]
	deleted.length = 0
	server.use(
		http.get('/api/content', ({ request }) => {
			const query = new URL(request.url).searchParams
			const matching = query.get('status') === 'trash' ? bin : []
			const perPage = Number(query.get('per_page') ?? '20')
			return HttpResponse.json({ items: matching.slice(0, perPage), total: matching.length })
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
		http.delete('/api/content/:id', ({ request, params }) => {
			deleted.push(new URL(request.url))
			bin = bin.filter((post) => post.id !== String(params.id))
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

test('offers to empty the trash only in the trash view', async () => {
	renderAt('/content/post')
	await screen.findByRole('button', { name: 'All (2)' })

	expect(screen.queryByRole('button', { name: 'Empty Trash' })).not.toBeInTheDocument()
	await openTrashView()

	expect(screen.getByRole('button', { name: 'Empty Trash' })).toBeInTheDocument()
})

test('asks to confirm before emptying the trash', async () => {
	renderAt('/content/post')
	await openTrashView()

	await userEvent.click(screen.getByRole('button', { name: 'Empty Trash' }))

	expect(await screen.findByRole('alertdialog')).toBeInTheDocument()
	expect(deleted).toEqual([])
})

test('warns that every item in the trash of the type on screen goes for good', async () => {
	renderAt('/content/post')
	await openTrashView()

	await userEvent.click(screen.getByRole('button', { name: 'Empty Trash' }))

	expect(await screen.findByRole('alertdialog')).toHaveTextContent(
		'Every item in the posts trash is removed for good. This cannot be undone.',
	)
})

test('deletes every trashed post once confirmed', async () => {
	renderAt('/content/post')
	await openTrashView()
	await userEvent.click(screen.getByRole('button', { name: 'Empty Trash' }))

	await userEvent.click(await screen.findByRole('button', { name: 'Delete All' }))

	await waitFor(() => expect(deleted).toHaveLength(2))
	expect(deleted.map((url) => url.searchParams.get('force'))).toEqual(['true', 'true'])
	expect(bin).toEqual([])
})

test('empties a trash holding more than one page', async () => {
	bin = Array.from({ length: 25 }, (_, index) => ({
		...TRASHED,
		id: `019fb000-0000-7000-8000-0000000001${String(index).padStart(2, '0')}`,
		title: `Old Notes ${index}`,
	}))
	renderAt('/content/post')
	await userEvent.click(await screen.findByRole('button', { name: 'Trash (25)' }))
	await screen.findByText('Old Notes 0')
	await userEvent.click(screen.getByRole('button', { name: 'Empty Trash' }))

	await userEvent.click(await screen.findByRole('button', { name: 'Delete All' }))

	await waitFor(() => expect(bin).toEqual([]))
	expect(deleted).toHaveLength(25)
})

test('empties only the trash of the type on screen', async () => {
	const postType = {
		key: 'post',
		singular_label: 'Post',
		plural_label: 'Posts',
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
	const pageType = {
		...postType,
		key: 'page',
		singular_label: 'Page',
		plural_label: 'Pages',
		route_word: 'pages',
		default: false,
	}
	const trashedPage = { ...TRASHED, id: '019fb000-0000-7000-8000-000000000005', type: 'page', title: 'Old Page' }
	const trashes: Record<string, { id: string, title: string }[]> = { post: [TRASHED], page: [trashedPage] }
	server.use(
		http.get('/api/types', () => HttpResponse.json({ items: [postType, pageType] })),
		http.get('/api/content', ({ request }) => {
			const query = new URL(request.url).searchParams
			const matching = query.get('status') === 'trash' ? trashes[query.get('type') ?? 'post'] : []
			return HttpResponse.json({ items: matching, total: matching.length })
		}),
		http.get('/api/content/counts', ({ request }) => {
			const held = trashes[new URL(request.url).searchParams.get('type') ?? 'post']
			return HttpResponse.json({ draft: 0, pending: 0, private: 0, published: 0, trash: held.length })
		}),
		http.delete('/api/content/:id', ({ params }) => {
			for (const key of Object.keys(trashes)) {
				trashes[key] = trashes[key].filter((item) => item.id !== String(params.id))
			}
			return new HttpResponse(null, { status: 204 })
		}),
	)
	renderAt('/content/page')
	await userEvent.click(await screen.findByRole('button', { name: 'Trash (1)' }))
	await screen.findByText('Old Page')
	await userEvent.click(screen.getByRole('button', { name: 'Empty Trash' }))
	expect(await screen.findByRole('alertdialog')).toHaveTextContent('Every item in the pages trash is removed for good.')

	await userEvent.click(await screen.findByRole('button', { name: 'Delete All' }))

	await waitFor(() => expect(trashes.page).toEqual([]))
	expect(trashes.post).toEqual([TRASHED])
})

test('keeps the trash when the confirm is dismissed', async () => {
	renderAt('/content/post')
	await openTrashView()
	await userEvent.click(screen.getByRole('button', { name: 'Empty Trash' }))

	await userEvent.click(await screen.findByRole('button', { name: 'Cancel' }))

	await waitFor(() => expect(screen.queryByRole('alertdialog')).not.toBeInTheDocument())
	expect(deleted).toEqual([])
})

test('forgets the posts an interrupted empty trash still removed', async () => {
	vi.spyOn(console, 'error').mockImplementation(() => {})
	server.use(
		http.get(`/api/content/${TRASHED.id}`, () =>
			bin.some((post) => post.id === TRASHED.id)
				? HttpResponse.json({ ...storedPost, ...TRASHED })
				: HttpResponse.json({}, { status: 404 }),
		),
		http.delete('/api/content/:id', ({ request, params }) => {
			if (String(params.id) === SECOND.id) {
				return HttpResponse.json({}, { status: 500 })
			}
			deleted.push(new URL(request.url))
			bin = bin.filter((post) => post.id !== String(params.id))
			return new HttpResponse(null, { status: 204 })
		}),
	)
	const { router } = renderRoutedAt(`/content/post/${TRASHED.id}/edit`)
	await screen.findByText(/This item is in the trash/)
	await act(async () => {
		await router.navigate({ to: '/content/$typeKey', params: { typeKey: 'post' } })
	})
	await openTrashView()
	await userEvent.click(screen.getByRole('button', { name: 'Empty Trash' }))
	await userEvent.click(await screen.findByRole('button', { name: 'Delete All' }))
	await screen.findByText(/could not empty the trash/i)

	await act(async () => {
		await router.navigate({
			to: '/content/$typeKey/$postId/edit',
			params: { typeKey: 'post', postId: TRASHED.id },
		})
	})

	expect(await screen.findByText('Could not load that post.')).toBeInTheDocument()
})

test('reports an empty trash the server refused', async () => {
	vi.spyOn(console, 'error').mockImplementation(() => {})
	server.use(http.delete('/api/content/:id', () => HttpResponse.json({}, { status: 500 })))
	renderAt('/content/post')
	await openTrashView()
	await userEvent.click(screen.getByRole('button', { name: 'Empty Trash' }))

	await userEvent.click(await screen.findByRole('button', { name: 'Delete All' }))

	expect(await screen.findByText(/could not empty the trash/i)).toBeInTheDocument()
})
