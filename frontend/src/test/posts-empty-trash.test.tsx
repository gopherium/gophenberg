// SPDX-License-Identifier: Apache-2.0

import { http, HttpResponse, server } from '@gophenberg/frontend-sdk/testing'
import { screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { beforeEach, expect, test, vi } from 'vitest'

import { renderAt } from './render'

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

test('keeps the trash when the confirm is dismissed', async () => {
	renderAt('/content/post')
	await openTrashView()
	await userEvent.click(screen.getByRole('button', { name: 'Empty Trash' }))

	await userEvent.click(await screen.findByRole('button', { name: 'Cancel' }))

	await waitFor(() => expect(screen.queryByRole('alertdialog')).not.toBeInTheDocument())
	expect(deleted).toEqual([])
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
