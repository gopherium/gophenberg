// SPDX-License-Identifier: Apache-2.0

import { http, HttpResponse, server } from '@gophenberg/frontend-sdk/testing'
import { act, screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { beforeAll, beforeEach, expect, test, vi } from 'vitest'

import { renderAt, renderRoutedAt } from './render'
import { storedPost } from './postFixture'
import { findActionToast } from './typeFixtures'
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
 * Trashes the ticked posts through the bulk bar.
 */
async function trashTheTicked() {
	await userEvent.click(screen.getByRole('button', { name: 'Trash' }))
}

test('trashes every selected post at once, asking nothing first', async () => {
	renderAt('/content/post')
	await selectBoth()

	await userEvent.click(screen.getByRole('button', { name: 'Trash' }))

	await waitFor(() => expect(trashed).toHaveLength(2))
	expect(trashed).toEqual(expect.arrayContaining([FIRST.id, SECOND.id]))
	expect(screen.queryByRole('dialog')).not.toBeInTheDocument()
	expect(screen.queryByRole('alertdialog')).not.toBeInTheDocument()
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

	await findActionToast('2 items moved to the trash.')
	await waitFor(() => expect(screen.queryByRole('button', { name: 'Trash' })).not.toBeInTheDocument())
})

test('counts the posts a bulk trash moved in a toast whose Undo restores both', async () => {
	renderAt('/content/post')
	await selectBoth()
	await trashTheTicked()
	const toast = await findActionToast('2 items moved to the trash.')
	expect(restored).toEqual([])

	await userEvent.click(within(toast).getByRole('button', { name: 'Undo' }))

	expect(await screen.findByText('2 posts have been restored.', { selector: '.godmin-toast' })).toBeInTheDocument()
	expect(restored).toEqual(expect.arrayContaining([FIRST.id, SECOND.id]))
	expect(restored).toHaveLength(2)
})

test('counts only what a partly refused bulk trash could not move, in one toast', async () => {
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

	expect(await screen.findByText('2 items could not be moved to the trash.', { selector: '.godmin-toast' }))
		.toBeInTheDocument()
	expect(document.querySelectorAll('.godmin-toast')).toHaveLength(1)
	expect(screen.queryByText('1 item moved to the trash.')).not.toBeInTheDocument()
	expect(screen.queryByRole('button', { name: 'Undo' })).not.toBeInTheDocument()
	expect(screen.queryByRole('alert')).not.toBeInTheDocument()
	expect(screen.getByRole('checkbox', { name: 'Second Thoughts' })).toBeChecked()
	expect(screen.getByRole('checkbox', { name: 'Third Draft' })).toBeChecked()
})

test('keeps the tick on a row a run did not reach', async () => {
	renderAt('/content/post')
	await screen.findByText('Welcome to Gophenberg')
	await userEvent.click(screen.getByRole('checkbox', { name: 'Second Thoughts' }))
	const first = screen.getAllByRole('row').find((row) => within(row).queryByText('Welcome to Gophenberg') !== null)

	await userEvent.click(within(first as HTMLElement).getByRole('button', { name: 'Actions' }))
	await userEvent.click(await screen.findByRole('menuitem', { name: 'Trash' }))

	await findActionToast('"Welcome to Gophenberg" moved to the trash.')
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
	await screen.findByText('1 item could not be moved to the trash.', { selector: '.godmin-toast' })

	await act(async () => {
		await router.navigate({
			to: '/content/$typeKey/$postId/edit',
			params: { typeKey: 'post', postId: FIRST.id },
		})
	})

	expect(await screen.findByText(/This item is in the trash/)).toBeInTheDocument()
})

test('reports a batch the server partly refused in a toast and reloads the list', async () => {
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

	expect(await screen.findByText('1 item could not be moved to the trash.', { selector: '.godmin-toast' }))
		.toBeInTheDocument()
	await waitFor(() =>
		expect(screen.queryByText('Welcome to Gophenberg')).not.toBeInTheDocument(),
	)
	expect(screen.getByText('Second Thoughts')).toBeInTheDocument()
})
