// SPDX-License-Identifier: Apache-2.0

import { http, HttpResponse, server } from '@gophenberg/frontend-sdk/testing'
import { expect, test } from 'vitest'

import { MEDIA_CATEGORIES, toInserterItem } from '../media/inserterCategories'

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
		medium: {
			file: '2026/08/harbor-300x188.jpg',
			width: 300,
			height: 188,
			mime_type: 'image/jpeg',
			filesize: 20000,
		},
	},
	updated_at: '2026-08-10T10:00:00Z',
}

test('offers a category for every kind the inserter tab shows', () => {
	expect(MEDIA_CATEGORIES.map((category) => category.name)).toEqual([
		'gophenberg-images',
		'gophenberg-videos',
		'gophenberg-audio',
	])
	expect(MEDIA_CATEGORIES.map((category) => category.mediaType)).toEqual([
		'image',
		'video',
		'audio',
	])
	for (const category of MEDIA_CATEGORIES) {
		expect(category.labels.name).not.toBe('')
	}
})

test('asks the library for the kind and page the inserter wants', async () => {
	let asked = ''
	server.use(
		http.get('/api/media', ({ request }) => {
			asked = new URL(request.url).search
			return HttpResponse.json({ items: [HARBOR], total: 1 })
		}),
	)

	const found = await MEDIA_CATEGORIES[0].fetch({ per_page: 20, page: 2, search: 'harbor' })

	expect(asked).toContain('mime=image')
	expect(asked).toContain('search=harbor')
	expect(asked).toContain('page=2')
	expect(found).toHaveLength(1)
})

test('asks the first page when the inserter names none', async () => {
	let asked = ''
	server.use(
		http.get('/api/media', ({ request }) => {
			asked = new URL(request.url).search
			return HttpResponse.json({ items: [], total: 0 })
		}),
	)

	await MEDIA_CATEGORIES[1].fetch({})

	expect(asked).toContain('mime=video')
	expect(asked).not.toContain('search=')
})

test('shapes a stored item the way the inserter reads it', () => {
	const shaped = toInserterItem({
		id: 7,
		type: 'image',
		file: '2026/08/harbor.jpg',
		title: 'Harbor at dawn',
		altText: 'Boats at sunrise',
		caption: 'Before the market opens',
		description: '',
		mimeType: 'image/jpeg',
		width: 1600,
		height: 1000,
		filesize: 254000,
		sizes: {
			medium: {
				file: '2026/08/harbor-300x188.jpg',
				width: 300,
				height: 188,
				mimeType: 'image/jpeg',
				filesize: 20000,
			},
		},
		authorId: '',
		createdAt: '',
		updatedAt: '',
	})

	expect(shaped).toEqual({
		id: 7,
		url: '/media/2026/08/harbor.jpg',
		alt: 'Boats at sunrise',
		caption: 'Before the market opens',
		title: 'Harbor at dawn',
		previewUrl: '/media/2026/08/harbor-300x188.jpg',
	})
})

test('previews a stored item that carries no rendition from its own file', () => {
	const shaped = toInserterItem({
		id: 8,
		type: 'video',
		file: '2026/08/launch.mp4',
		title: 'Launch',
		altText: '',
		caption: '',
		description: '',
		mimeType: 'video/mp4',
		width: 0,
		height: 0,
		filesize: 900,
		sizes: {},
		authorId: '',
		createdAt: '',
		updatedAt: '',
	})

	expect(shaped.previewUrl).toBe('/media/2026/08/launch.mp4')
})
