// SPDX-License-Identifier: Apache-2.0

import { http, HttpResponse, server } from '@gophenberg/frontend-sdk/testing'
import { screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { beforeAll, beforeEach, expect, test } from 'vitest'

import { renderAt } from './render'
import { storedPost } from './postFixture'

const EDITOR_PATH = `/posts/${storedPost.id}/edit`

const NEWER = {
	target: 'autosave',
	post_id: storedPost.id,
	title: 'Words the browser kept',
	content: '<!-- wp:paragraph -->\n<p>Kept words.</p>\n<!-- /wp:paragraph -->',
	excerpt: '',
	saved_at: '2026-07-30T09:00:00Z',
}

const OLDER = { ...NEWER, saved_at: '2026-07-20T09:00:00Z' }

beforeAll(async () => {
	await import('../posts/EditorScreen')
}, 120000)

beforeEach(() => {
	server.use(http.get(`/api/posts/${storedPost.id}`, () => HttpResponse.json(storedPost)))
})

/**
 * Serves the given autosave for the post under edit.
 * @param autosave - The autosave the server holds, or nothing.
 */
function serveAutosave(autosave: object | null) {
	server.use(
		http.get(`/api/posts/${storedPost.id}/autosave`, () =>
			autosave === null
				? HttpResponse.json({}, { status: 404 })
				: HttpResponse.json(autosave),
		),
	)
}

test('says nothing when the server kept no words', async () => {
	serveAutosave(null)
	renderAt(EDITOR_PATH)
	await screen.findByRole('textbox', { name: 'Title' })

	expect(screen.queryByText(/unsaved version/i)).not.toBeInTheDocument()
})

test('says nothing when the kept words are older than the post', async () => {
	const asked: string[] = []
	server.use(
		http.get(`/api/posts/${storedPost.id}/autosave`, () => {
			asked.push('read')
			return HttpResponse.json(OLDER)
		}),
	)
	renderAt(EDITOR_PATH)
	await screen.findByRole('textbox', { name: 'Title' })

	await waitFor(() => expect(asked).toHaveLength(1))

	expect(screen.queryByText(/unsaved version/i)).not.toBeInTheDocument()
})

test('says nothing when the post already holds the kept words', async () => {
	const asked: string[] = []
	server.use(
		http.get(`/api/posts/${storedPost.id}/autosave`, () => {
			asked.push('read')
			return HttpResponse.json({
				...NEWER,
				title: storedPost.title,
				content: storedPost.content,
				excerpt: storedPost.excerpt,
			})
		}),
	)
	renderAt(EDITOR_PATH)
	await screen.findByRole('textbox', { name: 'Title' })

	await waitFor(() => expect(asked).toHaveLength(1))

	expect(screen.queryByRole('button', { name: 'Restore' })).not.toBeInTheDocument()
})

test('says nothing when the kept words trail the post inside one second', async () => {
	const asked: string[] = []
	server.use(
		http.get(`/api/posts/${storedPost.id}`, () =>
			HttpResponse.json({ ...storedPost, updated_at: '2026-07-28T09:00:00.1045Z' }),
		),
		http.get(`/api/posts/${storedPost.id}/autosave`, () => {
			asked.push('read')
			return HttpResponse.json({ ...NEWER, saved_at: '2026-07-28T09:00:00.104Z' })
		}),
	)
	renderAt(EDITOR_PATH)
	await screen.findByRole('textbox', { name: 'Title' })

	await waitFor(() => expect(asked).toHaveLength(1))

	expect(screen.queryByRole('button', { name: 'Restore' })).not.toBeInTheDocument()
})

test('offers the kept words when they are newer than the post', async () => {
	serveAutosave(NEWER)
	renderAt(EDITOR_PATH)

	expect(await screen.findByText(/unsaved version/i)).toBeInTheDocument()
	expect(screen.getByRole('button', { name: 'Restore' })).toBeInTheDocument()
})

test('takes the kept words into the editor when restored', async () => {
	serveAutosave(NEWER)
	renderAt(EDITOR_PATH)
	await screen.findByText(/unsaved version/i)

	await userEvent.click(screen.getByRole('button', { name: 'Restore' }))

	await waitFor(() =>
		expect(screen.getByRole('textbox', { name: 'Title' })).toHaveValue('Words the browser kept'),
	)
})

test('leaves the post unsaved after a restore so the author can keep it', async () => {
	serveAutosave(NEWER)
	renderAt(EDITOR_PATH)
	await screen.findByText(/unsaved version/i)

	await userEvent.click(screen.getByRole('button', { name: 'Restore' }))

	await waitFor(() =>
		expect(screen.getByRole('button', { name: 'Save draft' })).toHaveAttribute(
			'aria-disabled',
			'false',
		),
	)
})

test('drops the offer once it is taken', async () => {
	serveAutosave(NEWER)
	renderAt(EDITOR_PATH)
	await screen.findByText(/unsaved version/i)

	await userEvent.click(screen.getByRole('button', { name: 'Restore' }))

	await waitFor(() => expect(screen.queryByText(/unsaved version/i)).not.toBeInTheDocument())
})
