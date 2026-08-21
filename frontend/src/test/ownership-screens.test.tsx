// SPDX-License-Identifier: Apache-2.0

import { http, HttpResponse, server } from '@gophenberg/frontend-sdk/testing'
import { screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { beforeAll, expect, test } from 'vitest'

import { adminUser, renderAt } from './render'
import { storedPost } from './postFixture'

const FOREIGN = '019fb000-0000-7000-8000-0000000000ff'

const maria = { ...adminUser, rank: 'author' }

const grace = { ...adminUser, rank: 'editor' }

beforeAll(async () => {
	await import('../content/EditorScreen')
	await import('../media/MediaScreen')
}, 120000)

/**
 * Returns a listed post authored by the given account.
 * @param authorId - The account the post belongs to.
 * @returns The post row.
 */
function postBy(authorId: string) {
	return {
		id: '019fb000-0000-7000-8000-000000000001',
		type: 'post',
		slug: 'welcome',
		title: 'Welcome to Gophenberg',
		excerpt: '',
		status: 'published',
		author_id: authorId,
		author_name: 'Maria Perez',
		published_at: '2026-07-20T10:00:00Z',
		created_at: '2026-07-19T10:00:00Z',
		updated_at: '2026-07-20T10:00:00Z',
	}
}

/**
 * Serves a one-post listing authored by the given account.
 * @param authorId - The account the listed post belongs to.
 */
function servePosts(authorId: string) {
	server.use(
		http.get('/api/content', () =>
			HttpResponse.json({ items: [postBy(authorId)], total: 1 }),
		),
		http.get('/api/content/counts', () =>
			HttpResponse.json({ draft: 0, pending: 0, private: 0, published: 1, trash: 0 }),
		),
	)
}

/**
 * Serves a one-item media library authored by the given account.
 * @param authorId - The account the item belongs to.
 */
function serveMedia(authorId: string) {
	server.use(
		http.get('/api/media', () =>
			HttpResponse.json({
				items: [
					{
						id: 7,
						type: 'image',
						file: '2026/08/harbor.jpg',
						title: 'Harbor at dawn',
						alt_text: 'Boats at sunrise',
						caption: '',
						description: '',
						mime_type: 'image/jpeg',
						width: 1600,
						height: 1000,
						filesize: 254000,
						sizes: {},
						author_id: authorId,
						created_at: '2026-08-10T10:00:00Z',
						updated_at: '2026-08-10T10:00:00Z',
					},
				],
				total: 1,
			}),
		),
	)
}

/**
 * Serves the stored post and its empty autosave for the editor.
 * @param authorId - The account the stored post belongs to.
 */
function serveEditor(authorId: string) {
	server.use(
		http.get(`/api/content/${storedPost.id}`, () =>
			HttpResponse.json({ ...storedPost, author_id: authorId }),
		),
		http.get(`/api/content/${storedPost.id}/autosave`, () =>
			HttpResponse.json({}, { status: 404 }),
		),
	)
}

test('offers an author no actions on a post another account wrote', async () => {
	servePosts(FOREIGN)
	renderAt('/content/post', maria)

	await screen.findByText('Welcome to Gophenberg')

	expect(screen.queryByRole('button', { name: 'Actions' })).toBeNull()
})

test('offers an author every action on its own post', async () => {
	servePosts(maria.id)
	renderAt('/content/post', maria)
	await screen.findByText('Welcome to Gophenberg')

	await userEvent.click(screen.getByRole('button', { name: 'Actions' }))

	expect(await screen.findByRole('menuitem', { name: 'Edit' })).toBeInTheDocument()
	expect(screen.getByRole('menuitem', { name: 'Move to Trash' })).toBeInTheDocument()
})

test('offers an editor every action on a post another account wrote', async () => {
	servePosts(FOREIGN)
	renderAt('/content/post', grace)
	await screen.findByText('Welcome to Gophenberg')

	await userEvent.click(screen.getByRole('button', { name: 'Actions' }))

	expect(await screen.findByRole('menuitem', { name: 'Edit' })).toBeInTheDocument()
})

test('opens a post another account wrote as a reading view', async () => {
	serveEditor(FOREIGN)
	renderAt(`/content/post/${storedPost.id}/edit`, maria)

	expect(await screen.findByText(/another account wrote this/i)).toBeInTheDocument()
	expect(screen.getByText('Welcome to Gophenberg')).toBeInTheDocument()
	expect(screen.queryByRole('textbox', { name: 'Title' })).toBeNull()
	expect(screen.queryByRole('button', { name: /publish|update/i })).toBeNull()
})

test('reads an untitled foreign post under a stand in title, its excerpt beside it', async () => {
	server.use(
		http.get(`/api/content/${storedPost.id}`, () =>
			HttpResponse.json({
				...storedPost,
				title: '',
				excerpt: 'A few words about it.',
				author_id: FOREIGN,
			}),
		),
		http.get(`/api/content/${storedPost.id}/autosave`, () =>
			HttpResponse.json({}, { status: 404 }),
		),
	)
	renderAt(`/content/post/${storedPost.id}/edit`, maria)

	expect(await screen.findByText('(no title)')).toBeInTheDocument()
	expect(screen.getByText('A few words about it.')).toBeInTheDocument()
})

test('opens an author s own post as the editor', async () => {
	serveEditor(maria.id)
	renderAt(`/content/post/${storedPost.id}/edit`, maria)

	expect(await screen.findByRole('textbox', { name: 'Title' })).toBeInTheDocument()
})

test('opens a foreign post as the editor for an editor', async () => {
	serveEditor(FOREIGN)
	renderAt(`/content/post/${storedPost.id}/edit`, grace)

	expect(await screen.findByRole('textbox', { name: 'Title' })).toBeInTheDocument()
})

test('offers an author no controls on media another account uploaded', async () => {
	serveMedia(FOREIGN)
	renderAt('/media', maria)
	await screen.findByText('Harbor at dawn')

	await userEvent.click(screen.getByRole('button', { name: 'Actions' }))

	expect(screen.queryByRole('menuitem', { name: 'Describe' })).toBeNull()
	expect(screen.queryByRole('menuitem', { name: 'Delete Permanently' })).toBeNull()
})

test('offers an editor every action on media another account uploaded', async () => {
	serveMedia(FOREIGN)
	renderAt('/media', grace)
	await screen.findByText('Harbor at dawn')

	await userEvent.click(screen.getByRole('button', { name: 'Actions' }))

	expect(await screen.findByRole('menuitem', { name: 'Describe' })).toBeInTheDocument()
	expect(screen.getByRole('menuitem', { name: 'Delete Permanently' })).toBeInTheDocument()
})

test('offers an author every action on its own upload', async () => {
	serveMedia(maria.id)
	renderAt('/media', maria)
	await screen.findByText('Harbor at dawn')

	await userEvent.click(screen.getByRole('button', { name: 'Actions' }))

	expect(await screen.findByRole('menuitem', { name: 'Describe' })).toBeInTheDocument()
	expect(screen.getByRole('menuitem', { name: 'Delete Permanently' })).toBeInTheDocument()
})
