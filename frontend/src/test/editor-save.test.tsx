// SPDX-License-Identifier: Apache-2.0

import { http, HttpResponse, server } from '@gophenberg/frontend-sdk/testing'
import { screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { beforeAll, beforeEach, expect, test, vi } from 'vitest'

import { renderAt } from './render'

const POST_ID = '019fb000-0000-7000-8000-000000000001'
const EDITOR_PATH = `/posts/${POST_ID}/edit`

const STORED = {
	id: POST_ID,
	type: 'post',
	slug: 'welcome',
	title: 'Welcome to Gophenberg',
	excerpt: '',
	content: '<!-- wp:paragraph -->\n<p>Stored words.</p>\n<!-- /wp:paragraph -->',
	status: 'draft',
	author_id: '019fb000-0000-7000-8000-0000000000ff',
	author_name: 'Maria Perez',
	published_at: null,
	created_at: '2026-07-19T10:00:00Z',
	updated_at: '2026-07-28T09:00:00Z',
}

const patched: unknown[] = []

beforeAll(async () => {
	await import('../posts/editorRoute.lazy')
}, 120000)

beforeEach(() => {
	patched.length = 0
	server.use(
		http.get(`/api/posts/${POST_ID}`, () => HttpResponse.json(STORED)),
		http.patch(`/api/posts/${POST_ID}`, async ({ request }) => {
			const body = await request.json()
			patched.push(body)
			return HttpResponse.json({ ...STORED, ...(body as object) })
		}),
	)
})

test('shows the stored title in its own field', async () => {
	renderAt(EDITOR_PATH)

	const title = await screen.findByRole('textbox', { name: 'Title' })

	expect(title).toHaveValue('Welcome to Gophenberg')
})

test('holds the save back until something changes', async () => {
	renderAt(EDITOR_PATH)
	await screen.findByRole('textbox', { name: 'Title' })

	expect(screen.getByRole('button', { name: /save/i })).toHaveAttribute('aria-disabled', 'true')
	expect(patched).toEqual([])
})

test('offers the save once the title changes', async () => {
	renderAt(EDITOR_PATH)
	const title = await screen.findByRole('textbox', { name: 'Title' })

	await userEvent.type(title, '!')

	expect(screen.getByRole('button', { name: /save/i })).toHaveAttribute('aria-disabled', 'false')
})

test('sends the edited title and the serialized content', async () => {
	renderAt(EDITOR_PATH)
	const title = await screen.findByRole('textbox', { name: 'Title' })
	await userEvent.type(title, '!')

	await userEvent.click(screen.getByRole('button', { name: /save/i }))

	await waitFor(() => expect(patched).toHaveLength(1))
	expect(patched[0]).toMatchObject({ title: 'Welcome to Gophenberg!' })
	expect((patched[0] as { content: string }).content).toContain('Stored words.')
})

test('publishes what was written, not only the status', async () => {
	renderAt(EDITOR_PATH)
	const title = await screen.findByRole('textbox', { name: 'Title' })
	await userEvent.type(title, '!')

	await userEvent.click(screen.getByRole('button', { name: 'Publish' }))

	await waitFor(() => expect(patched).toHaveLength(1))
	expect(patched[0]).toMatchObject({ status: 'published', title: 'Welcome to Gophenberg!' })
	expect((patched[0] as { content: string }).content).toContain('Stored words.')
})

test('updates a published post with what was written', async () => {
	server.use(
		http.get(`/api/posts/${POST_ID}`, () => HttpResponse.json({ ...STORED, status: 'published' })),
		http.patch(`/api/posts/${POST_ID}`, async ({ request }) => {
			const body = await request.json()
			patched.push(body)
			return HttpResponse.json({ ...STORED, status: 'published', ...(body as object) })
		}),
	)
	renderAt(EDITOR_PATH)
	const title = await screen.findByRole('textbox', { name: 'Title' })
	await userEvent.type(title, '!')

	await userEvent.click(screen.getByRole('button', { name: 'Update' }))

	await waitFor(() => expect(patched).toHaveLength(1))
	expect(patched[0]).toMatchObject({ title: 'Welcome to Gophenberg!' })
	expect((patched[0] as { content: string }).content).toContain('Stored words.')
})

test('refreshes the post the editor reopens without a full reload', async () => {
	const client = renderAt(EDITOR_PATH)
	const title = await screen.findByRole('textbox', { name: 'Title' })
	await userEvent.type(title, '!')

	await userEvent.click(screen.getByRole('button', { name: /save/i }))
	await waitFor(() => expect(patched).toHaveLength(1))

	await waitFor(() => {
		const held = client.getQueryData(['post', POST_ID]) as { title: string } | undefined
		expect(held?.title).toBe('Welcome to Gophenberg!')
	})
})

test('reports the post saved and holds the next save back', async () => {
	renderAt(EDITOR_PATH)
	const title = await screen.findByRole('textbox', { name: 'Title' })
	await userEvent.type(title, '!')

	await userEvent.click(screen.getByRole('button', { name: /save/i }))

	expect(await screen.findByText('Saved')).toBeInTheDocument()
	await waitFor(() =>
		expect(screen.getByRole('button', { name: /save/i })).toHaveAttribute('aria-disabled', 'true'),
	)
})

test('announces a post that saved', async () => {
	renderAt(EDITOR_PATH)
	const title = await screen.findByRole('textbox', { name: 'Title' })
	await userEvent.type(title, '!')

	await userEvent.click(screen.getByRole('button', { name: /save/i }))

	expect(await screen.findByText('Draft saved.')).toBeInTheDocument()
})

test('reports a post that changed under the editor', async () => {
	vi.spyOn(console, 'error').mockImplementation(() => {})
	server.use(
		http.patch(`/api/posts/${POST_ID}`, () =>
			HttpResponse.json({ error: 'post: conflicting update' }, { status: 409 }),
		),
	)
	renderAt(EDITOR_PATH)
	const title = await screen.findByRole('textbox', { name: 'Title' })
	await userEvent.type(title, '!')

	await userEvent.click(screen.getByRole('button', { name: /save/i }))

	expect(await screen.findByText(/changed elsewhere/i)).toBeInTheDocument()
})

test('reports a save the server rejected', async () => {
	vi.spyOn(console, 'error').mockImplementation(() => {})
	server.use(
		http.patch(`/api/posts/${POST_ID}`, () =>
			HttpResponse.json({ error: 'post: invalid status' }, { status: 422 }),
		),
	)
	renderAt(EDITOR_PATH)
	const title = await screen.findByRole('textbox', { name: 'Title' })
	await userEvent.type(title, '!')

	await userEvent.click(screen.getByRole('button', { name: /save/i }))

	expect(await screen.findByText(/invalid status/i)).toBeInTheDocument()
})

test('says it is saving while the write is in flight', async () => {
	let release: () => void = () => {}
	const held = new Promise<void>((resolve) => {
		release = resolve
	})
	server.use(
		http.patch(`/api/posts/${POST_ID}`, async () => {
			await held
			return HttpResponse.json(STORED)
		}),
	)
	renderAt(EDITOR_PATH)
	const title = await screen.findByRole('textbox', { name: 'Title' })
	await userEvent.type(title, '!')

	await userEvent.click(screen.getByRole('button', { name: /save/i }))

	expect(await screen.findByText('Saving')).toBeInTheDocument()
	release()
	await waitFor(() => expect(screen.queryByText('Saving')).not.toBeInTheDocument())
})

test('reports a refusal that named no reason', async () => {
	vi.spyOn(console, 'error').mockImplementation(() => {})
	server.use(
		http.patch(`/api/posts/${POST_ID}`, () => HttpResponse.json({ nope: true }, { status: 422 })),
	)
	renderAt(EDITOR_PATH)
	const title = await screen.findByRole('textbox', { name: 'Title' })
	await userEvent.type(title, '!')

	await userEvent.click(screen.getByRole('button', { name: /save/i }))

	expect(await screen.findByText(/answered 422/i)).toBeInTheDocument()
})

test('reports a refusal whose body was not json at all', async () => {
	vi.spyOn(console, 'error').mockImplementation(() => {})
	server.use(
		http.patch(`/api/posts/${POST_ID}`, () => new HttpResponse('gateway down', { status: 502 })),
	)
	renderAt(EDITOR_PATH)
	const title = await screen.findByRole('textbox', { name: 'Title' })
	await userEvent.type(title, '!')

	await userEvent.click(screen.getByRole('button', { name: /save/i }))

	expect(await screen.findByText(/answered 502/i)).toBeInTheDocument()
})

test('reports a save that never reached the server', async () => {
	vi.spyOn(console, 'error').mockImplementation(() => {})
	server.use(http.patch(`/api/posts/${POST_ID}`, () => HttpResponse.error()))
	renderAt(EDITOR_PATH)
	const title = await screen.findByRole('textbox', { name: 'Title' })
	await userEvent.type(title, '!')

	await userEvent.click(screen.getByRole('button', { name: /save/i }))

	expect(await screen.findByText(/could not save/i)).toBeInTheDocument()
})

test('reports a post it could not load', async () => {
	vi.spyOn(console, 'error').mockImplementation(() => {})
	server.use(http.get(`/api/posts/${POST_ID}`, () => HttpResponse.json({}, { status: 404 })))
	renderAt(EDITOR_PATH)

	expect(await screen.findByRole('alert')).toHaveTextContent(/could not load/i)
})
