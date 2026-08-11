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
			asked = request.url
			return HttpResponse.json({ items: [HARBOR], total: 1 })
		}),
	)

	const found = await MEDIA_CATEGORIES[0].fetch({ per_page: 20, page: 2, search: 'harbor' })

	const params = new URL(asked).searchParams
	expect(params.get('mime')).toBe('image/')
	expect(params.get('search')).toBe('harbor')
	expect(params.get('page')).toBe('2')
	expect(found).toHaveLength(1)
})

test('asks the first page when the inserter names none', async () => {
	let asked = ''
	server.use(
		http.get('/api/media', ({ request }) => {
			asked = request.url
			return HttpResponse.json({ items: [], total: 0 })
		}),
	)

	await MEDIA_CATEGORIES[1].fetch({})

	const params = new URL(asked).searchParams
	expect(params.get('mime')).toBe('video/')
	expect(params.get('search')).toBeNull()
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

test('places the scaled display copy of an oversized picture', () => {
	const shaped = toInserterItem({
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
