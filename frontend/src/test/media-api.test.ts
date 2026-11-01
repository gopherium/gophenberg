// SPDX-License-Identifier: Apache-2.0

import { http, HttpResponse, server } from '@gophenberg/frontend-sdk/testing'
import { expect, test } from 'vitest'

import {
	deleteMedia,
	editMedia,
	listMedia,
	listMediaByIDs,
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

const OWN_AUTHOR = '019fb000-0000-7000-8000-0000000000ff'

const FOREIGN_AUTHOR = '019fb000-0000-7000-8000-0000000000ee'

/**
 * Serves one listing page and records the parameters the request carried.
 * @param rows - The rows the page holds.
 * @returns The parameters of the last request, filled once it arrives.
 */
function recordedListing(rows: unknown[] = [HARBOR]): { params: URLSearchParams } {
	const seen = { params: new URLSearchParams() }
	server.use(
		http.get('/api/media', ({ request }) => {
			seen.params = new URL(request.url).searchParams
			return HttpResponse.json({ items: rows, total: rows.length, per_page: 10 })
		}),
	)
	return seen
}

test('lists media with the query it was given', async () => {
	const seen = recordedListing()

	const page = await listMedia({ mimes: ['image/'], search: 'harbor', page: 2 })

	expect(page.total).toBe(1)
	expect(page.items[0].title).toBe('Harbor at dawn')
	expect(Object.fromEntries(seen.params)).toEqual({ mime: 'image/', search: 'harbor', page: '2' })
})

test('asks for the sort, the authors and both dates a list narrows by, under the names the server reads', async () => {
	const seen = recordedListing()

	await listMedia({
		orderBy: 'title',
		order: 'asc',
		author: [OWN_AUTHOR, FOREIGN_AUTHOR],
		authorExclude: [FOREIGN_AUTHOR],
		before: '2026-08-16T00:00:00.000Z',
		after: '2026-08-01T00:00:00.000Z',
		modifiedBefore: '2026-09-16T00:00:00.000Z',
		modifiedAfter: '2026-09-01T00:00:00.000Z',
	})

	expect(Object.fromEntries(seen.params)).toEqual({
		orderby: 'title',
		order: 'asc',
		author: `${OWN_AUTHOR},${FOREIGN_AUTHOR}`,
		author_exclude: FOREIGN_AUTHOR,
		before: '2026-08-16T00:00:00.000Z',
		after: '2026-08-01T00:00:00.000Z',
		modified_before: '2026-09-16T00:00:00.000Z',
		modified_after: '2026-09-01T00:00:00.000Z',
	})
})

test('asks for no author and no kind when a list keeps or leaves out none', async () => {
	const seen = recordedListing()

	await listMedia({ author: [], authorExclude: [], mimes: [] })

	expect([...seen.params.keys()]).toEqual([])
})

test('reads the name and the avatar of the account that uploaded each listed file', async () => {
	recordedListing([{ ...HARBOR, author_name: 'Maria Perez', author_avatar: 'https://example.com/maria.png' }])

	const page = await listMedia({})

	expect(page.items[0]).toMatchObject({ authorName: 'Maria Perez', authorAvatar: 'https://example.com/maria.png' })
})

test('asks for the first page without naming it', async () => {
	let asked = ''
	server.use(
		http.get('/api/media', ({ request }) => {
			asked = new URL(request.url).search
			return HttpResponse.json({ items: [], total: 0, per_page: 30 })
		}),
	)

	await listMedia({ perPage: 30 })

	expect(asked).toBe('?per_page=30')
})

test('leaves the page size to the server when the caller names none', async () => {
	let asked: string | null = null
	server.use(
		http.get('/api/media', ({ request }) => {
			asked = new URL(request.url).search
			return HttpResponse.json({ items: [], total: 0, per_page: 10 })
		}),
	)

	await listMedia({})

	expect(asked).toBe('')
})

test('reads the page size the server used, which may fall short of the one asked for', async () => {
	server.use(http.get('/api/media', () => HttpResponse.json({ items: [HARBOR], total: 25, per_page: 10 })))

	const page = await listMedia({ perPage: 30 })

	expect(page).toMatchObject({ total: 25, perPage: 10 })
})

test('refuses a listing that names no page size', async () => {
	server.use(http.get('/api/media', () => HttpResponse.json({ items: [HARBOR], total: 1 })))

	await expect(listMedia({})).rejects.toThrow()
})

test('asks for the files a gallery holds by identity in one call', async () => {
	let asked = ''
	server.use(
		http.get('/api/media', ({ request }) => {
			asked = new URL(request.url).search
			return HttpResponse.json({ items: [HARBOR], total: 1 })
		}),
	)

	const held = await listMediaByIDs([7, 9])

	expect(asked).toBe('?ids=7%2C9')
	expect(held.map((item) => item.title)).toEqual(['Harbor at dawn'])
})

test('asks in pages when a gallery holds more files than one request carries', async () => {
	const asked: string[] = []
	server.use(
		http.get('/api/media', ({ request }) => {
			const ids = new URL(request.url).searchParams.get('ids') ?? ''
			asked.push(ids)
			return HttpResponse.json({
				items: ids.split(',').map((id) => ({ ...HARBOR, id: Number(id) })),
				total: ids.split(',').length,
			})
		}),
	)

	const held = await listMediaByIDs(Array.from({ length: 250 }, (_, at) => at + 1))

	expect(asked.map((page) => page.split(',').length)).toEqual([100, 100, 50])
	expect(held.map((item) => item.id)).toEqual(Array.from({ length: 250 }, (_, at) => at + 1))
})

test('reports the files it could not read by identity', async () => {
	server.use(http.get('/api/media', () => new HttpResponse(null, { status: 500 })))

	await expect(listMediaByIDs([7])).rejects.toThrow(/500/)
})

test('asks for no file when a gallery holds none', async () => {
	let called = false
	server.use(
		http.get('/api/media', () => {
			called = true
			return HttpResponse.json({ items: [], total: 0 })
		}),
	)

	const held = await listMediaByIDs([])

	expect(held).toEqual([])
	expect(called).toBe(false)
})

test('asks for the first page without naming it', async () => {
	let asked = ''
	server.use(
		http.get('/api/media', ({ request }) => {
			asked = new URL(request.url).search
			return HttpResponse.json({ items: [], total: 0, per_page: 30 })
		}),
	)

	await listMedia({ page: 1, perPage: 30 })

	expect(asked).toBe('?per_page=30')
})

test('fills in what a sparse media row leaves out', async () => {
	server.use(
		http.get('/api/media', () =>
			HttpResponse.json({
				items: [{ id: 8, type: 'file', file: 'manual.pdf', mime_type: 'application/pdf' }],
				total: 1,
				per_page: 10,
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
		authorName: '',
		authorAvatar: '',
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

	const stored = await uploadMedia(new File(['the bytes'], 'harbor.jpg', { type: 'image/jpeg' }))

	expect(stored).toEqual(expect.objectContaining({ id: 7 }))
	expect(contentType).toMatch(/^multipart\/form-data; boundary=/)
	expect(body).toMatch(/name="file"; filename="harbor\.jpg"/)
	expect(body).toContain('Content-Type: image/jpeg')
	expect(body).toContain('the bytes')
})

test('throws the reason an upload was refused, so a run never counts it as done', async () => {
	server.use(
		http.post('/api/media', () =>
			HttpResponse.json({ error: 'the file type is not allowed' }, { status: 422 }),
		),
	)

	await expect(uploadMedia(new File(['x'], 'notes.txt', { type: 'text/plain' }))).rejects.toThrow(
		'the file type is not allowed',
	)
})

test('throws a stand in when a refused upload carries no message', async () => {
	server.use(http.post('/api/media', () => new HttpResponse('', { status: 413 })))

	await expect(uploadMedia(new File(['x'], 'huge.jpg', { type: 'image/jpeg' }))).rejects.toThrow(
		'Something went wrong. Try again.',
	)
})

test('throws in the reader words an upload that never reached the server', async () => {
	server.use(http.post('/api/media', () => HttpResponse.error()))

	await expect(uploadMedia(new File(['x'], 'harbor.jpg', { type: 'image/jpeg' }))).rejects.toThrow(
		'The server could not be reached, so nothing was uploaded.',
	)
})

test('throws a stand in, never the words of a parser, when a stored upload answers with no JSON', async () => {
	server.use(http.post('/api/media', () => new HttpResponse('<html>gateway</html>', { status: 201 })))

	await expect(uploadMedia(new File(['x'], 'harbor.jpg', { type: 'image/jpeg' }))).rejects.toThrow(
		/^Something went wrong\. Try again\.$/,
	)
})

test('throws a stand in, never the words of a schema, when a stored upload answers with a misshapen item', async () => {
	server.use(http.post('/api/media', () => HttpResponse.json({ id: 'seven' }, { status: 201 })))

	await expect(uploadMedia(new File(['x'], 'harbor.jpg', { type: 'image/jpeg' }))).rejects.toThrow(
		/^Something went wrong\. Try again\.$/,
	)
})

/** The version of the harbor picture the server holds once another write landed after the list was read. */
const CURRENT_VERSION = '2026-08-12T09:00:00Z'

/** The reason the server gives for a file the library no longer holds, in the reader's words. */
const NOT_FOUND = [
	'This media item is not in the library, and someone may have deleted it.',
	'Reload the page to see what is there now.',
].join(' ')

/**
 * Answers a request as the server answers for a file the library no longer holds.
 * @returns The refusal.
 */
function gone(): Response {
	return HttpResponse.json({ error: 'gone', code: 'media_not_found' }, { status: 404 })
}

/**
 * Serves the harbor picture at its current version and records the body of every edit sent to it.
 * @returns The bodies, filled as requests arrive.
 */
function recordEdits(): Record<string, unknown>[] {
	const sent: Record<string, unknown>[] = []
	server.use(
		http.get('/api/media/7', () => HttpResponse.json({ ...HARBOR, updated_at: CURRENT_VERSION })),
		http.patch('/api/media/7', async ({ request }) => {
			const body = (await request.json()) as Record<string, unknown>
			sent.push(body)
			return HttpResponse.json({ ...HARBOR, ...body })
		}),
	)
	return sent
}

test('edits a file over the version read just before the write, sending only the fields it was given', async () => {
	const sent = recordEdits()

	const saved = await editMedia(7, { title: 'Harbor at noon', caption: '' }, 'unreached')

	expect(sent).toEqual([{ updated_at: CURRENT_VERSION, title: 'Harbor at noon', caption: '' }])
	expect(saved).toMatchObject({ id: 7, title: 'Harbor at noon' })
})

test('sends the alt text and the description under the names the server reads', async () => {
	const sent = recordEdits()

	await editMedia(7, { altText: 'Boats at noon', description: 'From the western pier' }, 'unreached')

	expect(sent).toEqual([
		{ updated_at: CURRENT_VERSION, alt_text: 'Boats at noon', description: 'From the western pier' },
	])
})

test('sends an edit as JSON', async () => {
	let sentAs: string | null = null
	server.use(
		http.get('/api/media/7', () => HttpResponse.json({ ...HARBOR, updated_at: CURRENT_VERSION })),
		http.patch('/api/media/7', ({ request }) => {
			sentAs = request.headers.get('content-type')
			return HttpResponse.json(HARBOR)
		}),
	)

	await editMedia(7, { title: 'Harbor at noon' }, 'unreached')

	expect(sentAs).toBe('application/json')
})

test.each([
	{ step: 'the read of the version', method: http.get },
	{ step: 'the write', method: http.patch },
])('throws in the reader words the reason $step was refused with', async ({ method }) => {
	recordEdits()
	server.use(method('/api/media/7', gone))

	await expect(editMedia(7, { title: 'Harbor at noon' }, 'unreached')).rejects.toThrow(NOT_FOUND)
})

test.each([
	{ step: 'the read of the version', method: http.get },
	{ step: 'the write', method: http.patch },
])('throws the words it was given when $step never reached the server', async ({ method }) => {
	recordEdits()
	server.use(method('/api/media/7', () => HttpResponse.error()))

	await expect(editMedia(7, { title: 'Harbor at noon' }, 'The name could not be updated.')).rejects.toThrow(
		/^The name could not be updated\.$/,
	)
})

test('throws a stand in, never the words of a schema, when the version read answers a misshapen item', async () => {
	server.use(http.get('/api/media/7', () => HttpResponse.json({ id: 'seven' })))

	await expect(editMedia(7, { title: 'Harbor at noon' }, 'unreached')).rejects.toThrow(
		/^Something went wrong\. Try again\.$/,
	)
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

test('throws in the reader words the reason a delete was refused with', async () => {
	server.use(http.delete('/api/media/7', gone))

	await expect(deleteMedia(7)).rejects.toThrow(NOT_FOUND)
})

test('throws the words of a permanent delete in the posts list when a delete never reached the server', async () => {
	server.use(http.delete('/api/media/7', () => HttpResponse.error()))

	await expect(deleteMedia(7)).rejects.toThrow(/^The item could not be permanently deleted\.$/)
})

test('addresses a stored file under the public media prefix', () => {
	expect(mediaSrc('2026/08/harbor.jpg')).toBe('/media/2026/08/harbor.jpg')
})
