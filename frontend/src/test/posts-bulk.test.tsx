// SPDX-License-Identifier: Apache-2.0

import { http, HttpResponse, server } from '@gophenberg/frontend-sdk/testing'
import { screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { beforeEach, expect, test, vi } from 'vitest'

import { renderAt } from './render'
import { warmPostsScreen } from './warm'

warmPostsScreen()

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

test('trashes every selected post behind one confirm', async () => {
	renderAt('/content/post')
	await selectBoth()

	await userEvent.click(screen.getByRole('button', { name: 'Move to Trash' }))
	const dialog = await screen.findByRole('dialog')
	expect(dialog).toHaveTextContent(/2 posts/)
	expect(trashed).toEqual([])
	await userEvent.click(within(dialog).getByRole('button', { name: 'Move to Trash' }))

	await waitFor(() => expect(trashed).toHaveLength(2))
	expect(trashed).toEqual(expect.arrayContaining([FIRST.id, SECOND.id]))
})

test('does not hold restored posts selected from before their trashing', async () => {
	renderAt('/content/post')
	await selectBoth()
	await userEvent.click(screen.getByRole('button', { name: 'Move to Trash' }))
	const dialog = await screen.findByRole('dialog')
	await userEvent.click(within(dialog).getByRole('button', { name: 'Move to Trash' }))
	await screen.findByText(/2 posts moved to the trash/i)
	listed = [FIRST, SECOND]

	await userEvent.click(screen.getByRole('button', { name: 'Undo' }))

	expect(await screen.findByText('Welcome to Gophenberg')).toBeInTheDocument()
	expect(screen.queryByRole('checkbox', { name: 'Deselect all' })).not.toBeInTheDocument()
	expect(screen.getByRole('checkbox', { name: 'Welcome to Gophenberg' })).not.toBeChecked()
})

test('offers one undo covering the whole batch', async () => {
	renderAt('/content/post')
	await selectBoth()
	await userEvent.click(screen.getByRole('button', { name: 'Move to Trash' }))
	const dialog = await screen.findByRole('dialog')
	await userEvent.click(within(dialog).getByRole('button', { name: 'Move to Trash' }))
	await screen.findByText(/2 posts moved to the trash/i)

	await userEvent.click(screen.getByRole('button', { name: 'Undo' }))

	await waitFor(() => expect(restored).toHaveLength(2))
	expect(restored).toEqual(expect.arrayContaining([FIRST.id, SECOND.id]))
})

test('reports a batch undo the server refused', async () => {
	vi.spyOn(console, 'error').mockImplementation(() => {})
	server.use(http.post('/api/content/:id/restore', () => HttpResponse.json({}, { status: 500 })))
	renderAt('/content/post')
	await selectBoth()
	await userEvent.click(screen.getByRole('button', { name: 'Move to Trash' }))
	const dialog = await screen.findByRole('dialog')
	await userEvent.click(within(dialog).getByRole('button', { name: 'Move to Trash' }))
	await screen.findByText(/2 posts moved to the trash/i)

	await userEvent.click(screen.getByRole('button', { name: 'Undo' }))

	expect(await screen.findByText(/could not restore those posts/i)).toBeInTheDocument()
})

test('reloads the list when an undo is only partly refused', async () => {
	vi.spyOn(console, 'error').mockImplementation(() => {})
	server.use(
		http.post('/api/content/:id/restore', ({ params }) => {
			if (String(params.id) === SECOND.id) {
				return HttpResponse.json({}, { status: 500 })
			}
			restored.push(String(params.id))
			listed = [...listed, FIRST]
			return HttpResponse.json({ ...FIRST, status: 'draft' })
		}),
	)
	renderAt('/content/post')
	await selectBoth()
	await userEvent.click(screen.getByRole('button', { name: 'Move to Trash' }))
	const dialog = await screen.findByRole('dialog')
	await userEvent.click(within(dialog).getByRole('button', { name: 'Move to Trash' }))
	await screen.findByText(/2 posts moved to the trash/i)

	await userEvent.click(screen.getByRole('button', { name: 'Undo' }))

	expect(await screen.findByText(/could not restore that post/i)).toBeInTheDocument()
	await waitFor(() => expect(restored).toEqual([FIRST.id]))
	await waitFor(() => expect(screen.getByText('Welcome to Gophenberg')).toBeInTheDocument())
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
	await userEvent.click(screen.getByRole('button', { name: 'Move to Trash' }))
	const dialog = await screen.findByRole('dialog')

	await userEvent.click(within(dialog).getByRole('button', { name: 'Move to Trash' }))

	expect(await screen.findByText(/could not move/i)).toBeInTheDocument()
	await waitFor(() =>
		expect(screen.queryByText('Welcome to Gophenberg')).not.toBeInTheDocument(),
	)
	expect(screen.getByText('Second Thoughts')).toBeInTheDocument()
})
