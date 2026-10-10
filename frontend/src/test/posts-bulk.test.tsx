// SPDX-License-Identifier: Apache-2.0

import { http, HttpResponse, server } from '@gophenberg/frontend-sdk/testing'
import { act, screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { beforeAll, beforeEach, expect, test, vi } from 'vitest'

import { renderAt, renderRoutedAt } from './render'
import { storedPost } from './postFixture'
import { siteSettings } from './siteSettings'
import { warmPostsScreen } from './warm'

warmPostsScreen()

beforeAll(async () => {
	await import('../content/EditorScreen')
}, 120000)

const FIRST = {
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

const SECOND = { ...FIRST, id: '019fb000-0000-7000-8000-000000000002', title: 'Second Thoughts' }

let listed: { id: string, title: string }[] = []
const trashed: string[] = []
const restored: string[] = []

beforeEach(() => {
	listed = [FIRST, SECOND]
	trashed.length = 0
	restored.length = 0
	server.use(
		http.get('/api/content', () => HttpResponse.json({ items: listed, total: listed.length })),
		http.get('/api/content/counts', () =>
			HttpResponse.json({ draft: 0, pending: 0, private: 0, published: listed.length, trash: 0 }),
		),
		http.delete('/api/content/:id', ({ params }) => {
			trashed.push(String(params.id))
			listed = listed.filter((post) => post.id !== String(params.id))
			return HttpResponse.json({ ...FIRST, id: String(params.id), status: 'trash' })
		}),
		http.post('/api/content/:id/restore', ({ params }) => {
			restored.push(String(params.id))
			return HttpResponse.json({ ...FIRST, id: String(params.id), status: 'draft' })
		}),
	)
})

/**
 * Selects both listed posts by their row checkboxes.
 */
async function selectBoth() {
	await screen.findByText('Welcome to Gophenberg')
	await userEvent.click(screen.getByRole('checkbox', { name: 'Welcome to Gophenberg' }))
	await userEvent.click(screen.getByRole('checkbox', { name: 'Second Thoughts' }))
}

/**
 * Trashes the ticked posts through the bulk bar and its confirm.
 */
async function trashTheTicked() {
	await userEvent.click(screen.getByRole('button', { name: 'Trash…' }))
	const dialog = await screen.findByRole('dialog')
	await userEvent.click(within(dialog).getByRole('button', { name: 'Trash' }))
}

test('trashes every selected post behind one confirm', async () => {
	renderAt('/content/post')
	await selectBoth()

	await userEvent.click(screen.getByRole('button', { name: 'Trash…' }))
	const dialog = await screen.findByRole('dialog')
	expect(dialog).toHaveTextContent('Move 2 items to the trash?')
	expect(trashed).toEqual([])
	await userEvent.click(within(dialog).getByRole('button', { name: 'Trash' }))

	await waitFor(() => expect(trashed).toHaveLength(2))
	expect(trashed).toEqual(expect.arrayContaining([FIRST.id, SECOND.id]))
})

test('quotes the one ticked post in the bulk trash confirm, cut at the length the site serves', async () => {
	server.use(http.get('/api/settings', () => HttpResponse.json({ ...siteSettings, toast_name_length: 10 })))
	renderAt('/content/post')
	await screen.findByText('Welcome to Gophenberg')
	await userEvent.click(screen.getByRole('checkbox', { name: 'Welcome to Gophenberg' }))

	await userEvent.click(screen.getByRole('button', { name: 'Trash…' }))

	expect(await screen.findByRole('dialog')).toHaveTextContent('Move "Welcome to…" to the trash?')
})

test('stops offering the bulk trash once the ticked posts sit in the trash', async () => {
	server.use(
		http.delete('/api/content/:id', ({ params }) => {
			trashed.push(String(params.id))
			listed = listed.map((post) => (post.id === String(params.id) ? { ...post, status: 'trash' } : post))
			return HttpResponse.json({ ...FIRST, id: String(params.id), status: 'trash' })
		}),
	)
	renderAt('/content/post')
	await selectBoth()

	await trashTheTicked()

	await screen.findByText('2 items moved to the trash.', { selector: '.godmin-toast' })
	await waitFor(() => expect(screen.queryByRole('button', { name: 'Trash…' })).not.toBeInTheDocument())
})

test('counts the posts a bulk trash moved in a toast with no undo', async () => {
	renderAt('/content/post')
	await selectBoth()

	await trashTheTicked()

	expect(await screen.findByText('2 items moved to the trash.', { selector: '.godmin-toast' })).toBeInTheDocument()
	expect(screen.queryByRole('button', { name: 'Undo' })).not.toBeInTheDocument()
	expect(restored).toEqual([])
})

test('counts what a partly refused bulk trash moved and what it could not', async () => {
	vi.spyOn(console, 'error').mockImplementation(() => {})
	const THIRD = { ...FIRST, id: '019fb000-0000-7000-8000-000000000003', title: 'Third Draft' }
	listed = [FIRST, SECOND, THIRD]
	server.use(
		http.delete('/api/content/:id', ({ params }) => {
			if (String(params.id) !== FIRST.id) {
				return HttpResponse.json({}, { status: 500 })
			}
			listed = listed.filter((post) => post.id !== FIRST.id)
			return HttpResponse.json({ ...FIRST, status: 'trash' })
		}),
	)
	renderAt('/content/post')
	await selectBoth()
	await userEvent.click(screen.getByRole('checkbox', { name: 'Third Draft' }))

	await trashTheTicked()

	expect(await screen.findByText('1 item moved to the trash.', { selector: '.godmin-toast' })).toBeInTheDocument()
	expect(await screen.findByText('2 items could not be moved to the trash.')).toBeInTheDocument()
	expect(screen.getByRole('checkbox', { name: 'Second Thoughts' })).toBeChecked()
	expect(screen.getByRole('checkbox', { name: 'Third Draft' })).toBeChecked()
})

test('keeps the tick on a row a run did not reach', async () => {
	renderAt('/content/post')
	await screen.findByText('Welcome to Gophenberg')
	await userEvent.click(screen.getByRole('checkbox', { name: 'Second Thoughts' }))
	const first = screen.getAllByRole('row').find((row) => within(row).queryByText('Welcome to Gophenberg') !== null)

	await userEvent.click(within(first as HTMLElement).getByRole('button', { name: 'Actions' }))
	await userEvent.click(await screen.findByRole('menuitem', { name: 'Trash…' }))
	await userEvent.click(within(await screen.findByRole('dialog')).getByRole('button', { name: 'Trash' }))

	await screen.findByText('"Welcome to Gophenberg" moved to the trash.', { selector: '.godmin-toast' })
	expect(screen.getByRole('checkbox', { name: 'Second Thoughts' })).toBeChecked()
})

test('forgets the cached copy of a post a partly refused batch still moved', async () => {
	vi.spyOn(console, 'error').mockImplementation(() => {})
	server.use(
		http.get(`/api/content/${FIRST.id}`, () =>
			HttpResponse.json({
				...storedPost,
				...FIRST,
				status: listed.some((post) => post.id === FIRST.id) ? 'published' : 'trash',
			}),
		),
		http.delete('/api/content/:id', ({ params }) => {
			if (String(params.id) === SECOND.id) {
				return HttpResponse.json({}, { status: 500 })
			}
			listed = listed.filter((post) => post.id !== String(params.id))
			return HttpResponse.json({ ...FIRST, status: 'trash' })
		}),
	)
	const { router } = renderRoutedAt(`/content/post/${FIRST.id}/edit`)
	await screen.findByRole('textbox', { name: 'Title' })
	await act(async () => {
		await router.navigate({ to: '/content/$typeKey', params: { typeKey: 'post' } })
	})
	await selectBoth()
	await trashTheTicked()
	await screen.findByText('1 item could not be moved to the trash.')

	await act(async () => {
		await router.navigate({
			to: '/content/$typeKey/$postId/edit',
			params: { typeKey: 'post', postId: FIRST.id },
		})
	})

	expect(await screen.findByText(/This item is in the trash/)).toBeInTheDocument()
})

test('reports a batch the server partly refused and reloads the list', async () => {
	vi.spyOn(console, 'error').mockImplementation(() => {})
	server.use(
		http.delete('/api/content/:id', ({ params }) => {
			if (String(params.id) === SECOND.id) {
				return HttpResponse.json({}, { status: 500 })
			}
			trashed.push(String(params.id))
			listed = listed.filter((post) => post.id !== String(params.id))
			return HttpResponse.json({ ...FIRST, status: 'trash' })
		}),
	)
	renderAt('/content/post')
	await selectBoth()

	await trashTheTicked()

	expect(await screen.findByText('1 item could not be moved to the trash.')).toBeInTheDocument()
	await waitFor(() =>
		expect(screen.queryByText('Welcome to Gophenberg')).not.toBeInTheDocument(),
	)
	expect(screen.getByText('Second Thoughts')).toBeInTheDocument()
})
