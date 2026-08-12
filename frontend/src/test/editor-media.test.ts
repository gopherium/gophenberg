// SPDX-License-Identifier: Apache-2.0

import { http, HttpResponse, server } from '@gophenberg/frontend-sdk/testing'
import { beforeEach, expect, test, vi } from 'vitest'

import {
	ALLOWED_MIME_TYPES,
	IMAGE_SIZES,
	MAX_UPLOAD_BYTES,
	editorMediaUpload,
	toAttachment,
} from '../media/editorMedia'
import type { EditorAttachment } from '../media/editorMedia'
import { EDITOR_SETTINGS } from '../content/editorSetup'

const HARBOR = {
	id: 7,
	type: 'image',
	file: '2026/08/harbor.jpg',
	title: 'Harbor at dawn',
	alt_text: 'Boats at sunrise',
	caption: 'Before the market opens',
	mime_type: 'image/jpeg',
	width: 1600,
	height: 1000,
	filesize: 254000,
	sizes: {
		large: {
			file: '2026/08/harbor-1024x640.jpg',
			width: 1024,
			height: 640,
			mime_type: 'image/jpeg',
			filesize: 90000,
		},
	},
	updated_at: '2026-08-10T10:00:00Z',
}

beforeEach(() => {
	let handed = 0
	URL.createObjectURL = vi.fn(() => {
		handed += 1
		return `blob:test-${handed}`
	})
	URL.revokeObjectURL = vi.fn()
})

/**
 * Returns a small JPEG file for an upload.
 * @param name - The name the file carries.
 * @returns The file to hand the seam.
 */
function photo(name = 'harbor.jpg'): File {
	return new File(['bytes'], name, { type: 'image/jpeg' })
}

/**
 * Runs the seam and resolves once it stops reporting.
 * @param args - The arguments the block editor would pass.
 * @returns Every onFileChange batch, in order.
 */
async function uploadBatches(
	args: Partial<Parameters<typeof editorMediaUpload>[0]>,
): Promise<{ batches: EditorAttachment[][], errors: string[] }> {
	const batches: EditorAttachment[][] = []
	const errors: string[] = []
	await editorMediaUpload({
		filesList: [photo()],
		onFileChange: (attachments) => batches.push(attachments),
		onError: (error) => errors.push(error.message),
		...args,
	})
	return { batches, errors }
}

test('reports a blob preview first and the stored attachment second', async () => {
	server.use(http.post('/api/media', () => HttpResponse.json(HARBOR, { status: 201 })))

	const { batches, errors } = await uploadBatches({})

	expect(errors).toEqual([])
	expect(batches[0]).toEqual([{ url: 'blob:test-1' }])
	expect(batches[1]).toHaveLength(1)
	expect(batches[1][0].id).toBe(7)
	expect(URL.revokeObjectURL).toHaveBeenCalledWith('blob:test-1')
})

test('shapes the stored attachment the way blocks read it', async () => {
	server.use(http.post('/api/media', () => HttpResponse.json(HARBOR, { status: 201 })))

	const { batches } = await uploadBatches({})

	expect(batches[1][0]).toEqual({
		id: 7,
		url: '/media/2026/08/harbor.jpg',
		alt: 'Boats at sunrise',
		caption: 'Before the market opens',
		title: 'Harbor at dawn',
		link: '/media/2026/08/harbor.jpg',
		mime_type: 'image/jpeg',
		media_type: 'image',
		media_details: {
			width: 1600,
			height: 1000,
			sizes: {
				large: { source_url: '/media/2026/08/harbor-1024x640.jpg', width: 1024, height: 640 },
			},
		},
	})
})

test('drops a refused upload and carries its reason', async () => {
	server.use(
		http.post('/api/media', () =>
			HttpResponse.json({ error: 'the image cannot be read' }, { status: 422 }),
		),
	)

	const { batches, errors } = await uploadBatches({})

	expect(errors).toEqual(['the image cannot be read'])
	expect(batches[0]).toEqual([{ url: 'blob:test-1' }])
	expect(batches[1]).toEqual([])
	expect(URL.revokeObjectURL).toHaveBeenCalledWith('blob:test-1')
})

test('reports an upload that never reached the server', async () => {
	vi.spyOn(console, 'error').mockImplementation(() => {})
	server.use(http.post('/api/media', () => HttpResponse.error()))

	const { batches, errors } = await uploadBatches({})

	expect(errors).toEqual(['The server could not be reached, so nothing was uploaded.'])
	expect(batches[1]).toEqual([])
})

test('refuses a file outside the allowed types before uploading', async () => {
	let uploaded = 0
	server.use(
		http.post('/api/media', () => {
			uploaded += 1
			return HttpResponse.json(HARBOR, { status: 201 })
		}),
	)

	const { batches, errors } = await uploadBatches({
		filesList: [new File(['x'], 'notes.txt', { type: 'text/plain' })],
		allowedTypes: ['image'],
	})

	expect(errors).toHaveLength(1)
	expect(errors[0]).toContain('notes.txt')
	expect(batches).toEqual([[]])
	expect(uploaded).toBe(0)
})

test('accepts a file matching an exact allowed mime', async () => {
	server.use(http.post('/api/media', () => HttpResponse.json(HARBOR, { status: 201 })))

	const { errors } = await uploadBatches({ allowedTypes: ['image/jpeg'] })

	expect(errors).toEqual([])
})

test('refuses a file over the size cap before uploading', async () => {
	let uploaded = 0
	server.use(
		http.post('/api/media', () => {
			uploaded += 1
			return HttpResponse.json(HARBOR, { status: 201 })
		}),
	)

	const { errors } = await uploadBatches({ maxUploadFileSize: 1 })

	expect(errors).toHaveLength(1)
	expect(errors[0]).toContain('harbor.jpg')
	expect(uploaded).toBe(0)
})

test('refuses a second file where only one may land', async () => {
	let uploaded = 0
	server.use(
		http.post('/api/media', () => {
			uploaded += 1
			return HttpResponse.json(HARBOR, { status: 201 })
		}),
	)

	const { batches, errors } = await uploadBatches({
		filesList: [photo('one.jpg'), photo('two.jpg')],
		multiple: false,
	})

	expect(errors).toHaveLength(1)
	expect(batches).toEqual([])
	expect(uploaded).toBe(0)
})

test('uploads several files where more than one may land', async () => {
	server.use(http.post('/api/media', () => HttpResponse.json(HARBOR, { status: 201 })))

	const { batches, errors } = await uploadBatches({
		filesList: [photo('one.jpg'), photo('two.jpg')],
	})

	expect(errors).toEqual([])
	expect(batches[0]).toHaveLength(2)
	expect(batches.at(-1)).toHaveLength(2)
})

test('places the scaled display copy of an oversized picture', () => {
	const shaped = toAttachment({
		id: 9,
		type: 'image',
		file: '2026/08/cliff.jpg',
		title: 'Cliff',
		altText: '',
		caption: '',
		description: '',
		mimeType: 'image/jpeg',
		width: 3000,
		height: 2000,
		filesize: 900000,
		sizes: {
			full: {
				file: '2026/08/cliff-scaled.jpg',
				width: 2560,
				height: 1707,
				mimeType: 'image/jpeg',
				filesize: 400000,
			},
		},
		authorId: '',
		createdAt: '',
		updatedAt: '',
	})

	expect(shaped.url).toBe('/media/2026/08/cliff-scaled.jpg')
	expect(shaped.link).toBe('/media/2026/08/cliff-scaled.jpg')
})

test('shapes a plain file without measurements', () => {
	const shaped = toAttachment({
		id: 8,
		type: 'file',
		file: '2026/08/manual.pdf',
		title: 'Manual',
		altText: '',
		caption: '',
		description: '',
		mimeType: 'application/pdf',
		width: 0,
		height: 0,
		filesize: 12000,
		sizes: {},
		authorId: '',
		createdAt: '',
		updatedAt: '',
	})

	expect(shaped.media_type).toBe('file')
	expect(shaped.media_details).toEqual({ width: 0, height: 0, sizes: {} })
})

test('hands the editor the media seam settings', () => {
	expect(EDITOR_SETTINGS.mediaUpload).toBe(editorMediaUpload)
	expect(EDITOR_SETTINGS.allowedMimeTypes).toBe(ALLOWED_MIME_TYPES)
	expect(EDITOR_SETTINGS.imageSizes).toBe(IMAGE_SIZES)
	expect(EDITOR_SETTINGS.imageDefaultSize).toBe('large')
	expect(EDITOR_SETTINGS.imageEditing).toBe(false)
	expect(EDITOR_SETTINGS.maxUploadFileSize).toBe(MAX_UPLOAD_BYTES)
})

test('mirrors the upload rules the server enforces', () => {
	expect(ALLOWED_MIME_TYPES).toEqual({
		'jpg|jpeg': 'image/jpeg',
		png: 'image/png',
		gif: 'image/gif',
		webp: 'image/webp',
		mp4: 'video/mp4',
		webm: 'video/webm',
		mp3: 'audio/mpeg',
		m4a: 'audio/mp4',
		wav: 'audio/wav',
		ogg: 'audio/ogg',
		flac: 'audio/flac',
		pdf: 'application/pdf',
		zip: 'application/zip',
	})
	expect(MAX_UPLOAD_BYTES).toBe(128 * 1024 * 1024)
	expect(IMAGE_SIZES).toEqual([
		{ slug: 'thumbnail', name: 'Thumbnail' },
		{ slug: 'medium', name: 'Medium' },
		{ slug: 'large', name: 'Large' },
		{ slug: 'full', name: 'Full Size' },
	])
})
