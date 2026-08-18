// SPDX-License-Identifier: Apache-2.0

import { http, HttpResponse, server } from '@gophenberg/frontend-sdk/testing'
import { expect, test } from 'vitest'

import {
	deleteMedia,
	describeMedia,
	listMedia,
	mediaSrc,
	uploadMedia,
} from '../media/api'

const HARBOR = {
	id: 7,
	type: 'image',
	file: '2026/08/harbor.jpg',
	title: 'Harbor at dawn',
	alt_text: 'Boats at sunrise',
	caption: 'Before the market opens',
	description: 'From the eastern pier',
	mime_type: 'image/jpeg',
	width: 1600,
	height: 1000,
	filesize: 254000,
	sizes: {
		thumbnail: {
			file: '2026/08/harbor-150x150.jpg',
			width: 150,
			height: 150,
			mime_type: 'image/jpeg',
			filesize: 9000,
		},
		full: {
			file: '2026/08/harbor.jpg',
			width: 1600,
			height: 1000,
			mime_type: 'image/jpeg',
			filesize: 254000,
		},
	},
	author_id: '019fb000-0000-7000-8000-0000000000ff',
	created_at: '2026-08-10T10:00:00Z',
	updated_at: '2026-08-10T10:00:00Z',
}

test('lists media with the query it was given', async () => {
	let asked = ''
	server.use(
		http.get('/api/media', ({ request }) => {
			asked = new URL(request.url).search
			return HttpResponse.json({ items: [HARBOR], total: 1 })
		}),
	)

	const page = await listMedia({ type: 'image', search: 'harbor', page: 2 })

	expect(page.total).toBe(1)
	expect(page.items[0].title).toBe('Harbor at dawn')
	expect(asked).toContain('type=image')
	expect(asked).toContain('search=harbor')
	expect(asked).toContain('page=2')
})

test('asks for the first page without naming it', async () => {
	let asked = ''
	server.use(
		http.get('/api/media', ({ request }) => {
			asked = new URL(request.url).search
			return HttpResponse.json({ items: [], total: 0 })
		}),
	)

	await listMedia({})

	expect(asked).toBe('?per_page=20')
})

test('asks for the first page without naming it', async () => {
	let asked = ''
	server.use(
		http.get('/api/media', ({ request }) => {
			asked = new URL(request.url).search
			return HttpResponse.json({ items: [], total: 0 })
		}),
	)

	await listMedia({ page: 1 })

	expect(asked).toBe('?per_page=20')
})

test('fills in what a sparse media row leaves out', async () => {
	server.use(
		http.get('/api/media', () =>
			HttpResponse.json({
				items: [{ id: 8, type: 'file', file: 'manual.pdf', mime_type: 'application/pdf' }],
				total: 1,
			}),
		),
	)

	const page = await listMedia({})

	expect(page.items[0]).toMatchObject({
		title: '',
		altText: '',
		caption: '',
		description: '',
		width: 0,
		height: 0,
		filesize: 0,
		sizes: {},
		authorId: '',
		createdAt: '',
		updatedAt: '',
	})
})

test('reports a listing that could not be read', async () => {
	server.use(http.get('/api/media', () => HttpResponse.json({}, { status: 500 })))

	await expect(listMedia({})).rejects.toThrow(/500/)
})

test('uploads a file as multipart under the field the server reads', async () => {
	let contentType: string | null = null
	let body = ''
	server.use(
		http.post('/api/media', async ({ request }) => {
			contentType = request.headers.get('content-type')
			body = await request.text()
			return HttpResponse.json(HARBOR, { status: 201 })
		}),
	)

	const outcome = await uploadMedia(new File(['the bytes'], 'harbor.jpg', { type: 'image/jpeg' }))

	expect(outcome).toEqual({ kind: 'stored', item: expect.objectContaining({ id: 7 }) })
	expect(contentType).toMatch(/^multipart\/form-data; boundary=/)
	expect(body).toMatch(/name="file"; filename=/)
	expect(body).toContain('Content-Type: image/jpeg')
})

test('carries the reason an upload was refused', async () => {
	server.use(
		http.post('/api/media', () =>
			HttpResponse.json({ error: 'the file type is not allowed' }, { status: 422 }),
		),
	)

	const outcome = await uploadMedia(new File(['x'], 'notes.txt', { type: 'text/plain' }))

	expect(outcome).toEqual({ kind: 'refused', reason: 'the file type is not allowed' })
})

test('stands in for a refusal that carries no message', async () => {
	server.use(http.post('/api/media', () => new HttpResponse('', { status: 413 })))

	const outcome = await uploadMedia(new File(['x'], 'huge.jpg', { type: 'image/jpeg' }))

	expect(outcome).toEqual({ kind: 'refused', reason: 'Something went wrong. Try again.' })
})

test('saves descriptions against the version it read', async () => {
	let sent: Record<string, unknown> = {}
	server.use(
		http.patch('/api/media/7', async ({ request }) => {
			sent = (await request.json()) as Record<string, unknown>
			return HttpResponse.json({ ...HARBOR, title: 'Harbor at noon' })
		}),
	)

	const outcome = await describeMedia(7, { title: 'Harbor at noon' }, HARBOR.updated_at)

	expect(outcome).toEqual({ kind: 'saved', item: expect.objectContaining({ title: 'Harbor at noon' }) })
	expect(sent).toEqual({ title: 'Harbor at noon', updated_at: HARBOR.updated_at })
})

test('reports an edit the server found stale', async () => {
	server.use(http.patch('/api/media/7', () => HttpResponse.json({}, { status: 409 })))

	const outcome = await describeMedia(7, { title: 'Harbor at noon' }, HARBOR.updated_at)

	expect(outcome.kind).toBe('stale')
})

test('carries the reason an edit was refused', async () => {
	server.use(
		http.patch('/api/media/7', () =>
			HttpResponse.json({ error: 'that is not a media item' }, { status: 404 }),
		),
	)

	const outcome = await describeMedia(7, { title: 'x' }, HARBOR.updated_at)

	expect(outcome).toEqual({ kind: 'rejected', message: 'that is not a media item' })
})

test('deletes a media item', async () => {
	let asked = ''
	server.use(
		http.delete('/api/media/7', ({ request }) => {
			asked = new URL(request.url).pathname
			return new HttpResponse(null, { status: 204 })
		}),
	)

	await deleteMedia(7)

	expect(asked).toBe('/api/media/7')
})

test('reports a delete that failed', async () => {
	server.use(http.delete('/api/media/7', () => HttpResponse.json({}, { status: 500 })))

	await expect(deleteMedia(7)).rejects.toThrow(/500/)
})

test('addresses a stored file under the public media prefix', () => {
	expect(mediaSrc('2026/08/harbor.jpg')).toBe('/media/2026/08/harbor.jpg')
})
