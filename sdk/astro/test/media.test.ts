// SPDX-License-Identifier: Apache-2.0

import { env } from 'node:process'

import { afterEach, beforeEach, describe, expect, test } from 'vitest'

import { mediaFields, mediaItems, mediaUrl, relatedFields, relatedItems } from '../index.ts'
import type { MediaValue, Post } from '../index.ts'

/**
 * Returns a published post carrying the given values.
 * @param fields - The values the post holds.
 * @returns The post.
 */
function posting(fields: Record<string, unknown>): Post {
	return {
		id: '1',
		type: 'post',
		path: 'hello-world',
		slug: 'hello-world',
		title: 'Hello world',
		excerpt: '',
		content: '',
		fields,
		published_at: '2026-08-04T12:00:00Z',
		updated_at: '2026-08-04T12:00:00Z',
	}
}

const SUNRISE: MediaValue = {
	id: 12,
	src: '/media/2026/08/sunrise.jpg',
	title: 'Sunrise',
	alt_text: 'Sunrise over the bay',
	caption: 'Golden hour at the marina',
	mime_type: 'image/jpeg',
	width: 3200,
	height: 1800,
	sizes: {
		large: { src: '/media/2026/08/sunrise-1024x576.jpg', width: 1024, height: 576, mime_type: 'image/jpeg' },
	},
}

const HARBOR: MediaValue = { ...SUNRISE, id: 15, src: '/media/2026/08/harbor.gif', sizes: {} }

const NEWS = { id: 'a1', title: 'News', path: 'categories/news' }

describe('the files a value names', () => {
	test('reads the one file a media value holds', () => {
		expect(mediaItems(SUNRISE)).toEqual([SUNRISE])
	})

	test('reads every file a gallery holds, in order', () => {
		expect(mediaItems([SUNRISE, HARBOR])).toEqual([SUNRISE, HARBOR])
	})

	test('reads nothing from a value naming no file', () => {
		expect(mediaItems(undefined)).toBeUndefined()
		expect(mediaItems('a word')).toBeUndefined()
		expect(mediaItems(12)).toBeUndefined()
		expect(mediaItems([])).toBeUndefined()
		expect(mediaItems({ id: 12 })).toBeUndefined()
	})

	test('reads nothing from a list holding anything but files', () => {
		expect(mediaItems([SUNRISE, 'a word'])).toBeUndefined()
	})
})

describe('the guards stay apart', () => {
	test('a file is never read as an item a relation points at', () => {
		expect(relatedItems([SUNRISE])).toBeUndefined()
		expect(relatedFields(posting({ cover: SUNRISE, gallery: [SUNRISE, HARBOR] }))).toEqual([])
	})

	test('an item a relation points at is never read as a file', () => {
		expect(mediaItems([NEWS])).toBeUndefined()
		expect(mediaFields(posting({ categories: [NEWS] }))).toEqual([])
	})
})

describe('the media fields an item carries', () => {
	test('names each field and the files it holds', () => {
		const post = posting({ cover: SUNRISE, gallery: [HARBOR], subtitle: 'words' })

		expect(mediaFields(post)).toEqual([
			{ key: 'cover', items: [SUNRISE] },
			{ key: 'gallery', items: [HARBOR] },
		])
	})

	test('reads the fields in the order their keys read', () => {
		const post = posting({ zebra: SUNRISE, apple: HARBOR })

		expect(mediaFields(post).map((field) => field.key)).toEqual(['apple', 'zebra'])
	})

	test('carries nothing for an item holding no files', () => {
		expect(mediaFields(posting({ subtitle: 'words' }))).toEqual([])
	})
})

describe('the address a theme loads a file from', () => {
	const held = env.GOPHENBERG_ASSET_ORIGIN

	beforeEach(() => {
		delete env.GOPHENBERG_ASSET_ORIGIN
	})

	afterEach(() => {
		if (held === undefined) {
			delete env.GOPHENBERG_ASSET_ORIGIN
			return
		}
		env.GOPHENBERG_ASSET_ORIGIN = held
	})

	test('loads from the theme origin when it shares one', () => {
		expect(mediaUrl(SUNRISE.src)).toBe('/media/2026/08/sunrise.jpg')
	})

	test('loads from the instance when the theme runs apart', () => {
		env.GOPHENBERG_ASSET_ORIGIN = 'https://example.com/'

		expect(mediaUrl(SUNRISE.src)).toBe('https://example.com/media/2026/08/sunrise.jpg')
	})

	test('leaves an address that already names its origin alone', () => {
		env.GOPHENBERG_ASSET_ORIGIN = 'https://example.com'

		expect(mediaUrl('https://cdn.example.com/a.jpg')).toBe('https://cdn.example.com/a.jpg')
	})
})
