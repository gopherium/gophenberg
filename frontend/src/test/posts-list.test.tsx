// SPDX-License-Identifier: Apache-2.0

import { http, HttpResponse, server } from '@gophenberg/frontend-sdk/testing'
import { screen, within } from '@testing-library/react'
import { beforeEach, expect, test } from 'vitest'

import { renderAt } from './render'
import { warmPostsScreen } from './warm'

warmPostsScreen()

const PUBLISHED = {
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

const DRAFT = {
	...PUBLISHED,
	id: '019fb000-0000-7000-8000-000000000002',
	slug: 'notes',
	title: 'Notes on the Next Release',
	status: 'draft',
	published_at: null,
	updated_at: '2026-07-28T09:00:00Z',
}

beforeEach(() => {
	server.use(
		http.get('/api/content', () => HttpResponse.json({ items: [PUBLISHED, DRAFT], total: 2 })),
		http.get('/api/content/counts', () =>
			HttpResponse.json({ draft: 1, published: 1, pending: 0, private: 0, trash: 0 }),
		),
	)
})

test('lists every post the API returned', async () => {
	renderAt('/content/post')

	expect(await screen.findByText('Welcome to Gophenberg')).toBeInTheDocument()
	expect(screen.getByText('Notes on the Next Release')).toBeInTheDocument()
})

test('links each title to its editor', async () => {
	renderAt('/content/post')

	const title = await screen.findByRole('link', { name: /Welcome to Gophenberg/ })

	expect(title).toHaveAttribute('href', `/admin/content/post/${PUBLISHED.id}/edit`)
})

test('marks a post that is not published with its status', async () => {
	renderAt('/content/post')

	const row = (await screen.findByText('Notes on the Next Release')).closest('td, div')

	expect(within(row as HTMLElement).getByText('Draft')).toBeInTheDocument()
})

test('leaves a published post unmarked', async () => {
	renderAt('/content/post')

	const row = (await screen.findByText('Welcome to Gophenberg')).closest('td, div')

	expect(within(row as HTMLElement).queryByText('Published')).not.toBeInTheDocument()
})

test('shows the author of each post', async () => {
	renderAt('/content/post')

	expect(await screen.findAllByText('Maria Perez')).toHaveLength(2)
})

test('dates a published post by its publication', async () => {
	renderAt('/content/post')

	expect(await screen.findByText(/Published/)).toBeInTheDocument()
})

test('dates an unpublished post by its last change', async () => {
	renderAt('/content/post')

	expect(await screen.findByText(/Last Modified/)).toBeInTheDocument()
})

test('reports a listing that could not be read', async () => {
	server.use(http.get('/api/content', () => HttpResponse.json({}, { status: 500 })))
	renderAt('/content/post')

	expect(await screen.findByRole('alert')).toHaveTextContent(/could not/i)
})

test('marks every status that is not published', async () => {
	server.use(
		http.get('/api/content', () =>
			HttpResponse.json({
				items: [
					{ ...DRAFT, id: 'a1', title: 'Pending One', status: 'pending' },
					{ ...DRAFT, id: 'a2', title: 'Private One', status: 'private' },
					{ ...DRAFT, id: 'a3', title: 'Scheduled One', status: 'scheduled' },
					{ ...DRAFT, id: 'a4', title: 'Trashed One', status: 'trash' },
				],
				total: 4,
			}),
		),
	)
	renderAt('/content/post')

	expect(await screen.findByText('Pending')).toBeInTheDocument()
	expect(screen.getByText('Private')).toBeInTheDocument()
	expect(screen.getByText('Scheduled')).toBeInTheDocument()
	expect(screen.getByText('Trash')).toBeInTheDocument()
})

test('names a post that has no title yet', async () => {
	server.use(
		http.get('/api/content', () =>
			HttpResponse.json({ items: [{ ...DRAFT, title: '' }], total: 1 }),
		),
	)
	renderAt('/content/post')

	expect(await screen.findByRole('link', { name: '(no title)' })).toBeInTheDocument()
})

test('dates a post that carries no timestamps', async () => {
	server.use(
		http.get('/api/content', () =>
			HttpResponse.json({
				items: [{ ...DRAFT, published_at: null, updated_at: undefined }],
				total: 1,
			}),
		),
	)
	renderAt('/content/post')

	expect(await screen.findByText('Last Modified')).toBeInTheDocument()
})
