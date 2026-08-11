// SPDX-License-Identifier: Apache-2.0

import { expect, test } from 'vitest'

import { bestRendition, fileSize, imageDimensions, mediaName } from '../media/format'
import type { MediaItem } from '../media/api'

/**
 * Returns a media item carrying the given overrides.
 * @param overrides - The fields to set on the item.
 * @returns The item to describe.
 */
function item(overrides: Partial<MediaItem> = {}): MediaItem {
	return {
		id: 1,
		type: 'image',
		file: '2026/08/harbor.jpg',
		title: 'Harbor at dawn',
		altText: '',
		caption: '',
		description: '',
		mimeType: 'image/jpeg',
		width: 1600,
		height: 1000,
		filesize: 254000,
		sizes: {},
		authorId: '',
		createdAt: '',
		updatedAt: '',
		...overrides,
	}
}

test('humanizes a file size in binary steps', () => {
	expect(fileSize(512)).toBe('512 B')
	expect(fileSize(51200)).toBe('50 KB')
	expect(fileSize(5 * 1024 * 1024)).toBe('5 MB')
	expect(fileSize(2.5 * 1024 * 1024 * 1024)).toBe('2.5 GB')
	expect(fileSize(1024)).toBe('1 KB')
})

test('rounds a file size to two decimals', () => {
	expect(fileSize(3.14159 * 1024 * 1024)).toBe('3.14 MB')
})

test('says nothing about a size it was never told', () => {
	expect(fileSize(0)).toBe('')
	expect(fileSize(0.5)).toBe('')
})

test('formats the dimensions of a picture', () => {
	expect(imageDimensions(item())).toBe('1600 × 1000')
})

test('says nothing about the dimensions of a file that has none', () => {
	expect(imageDimensions(item({ width: 0, height: 0 }))).toBe('')
	expect(imageDimensions(item({ height: 0 }))).toBe('')
})

test('names a media item by its title', () => {
	expect(mediaName(item())).toBe('Harbor at dawn')
})

test('names a media item that has no title by its file', () => {
	expect(mediaName(item({ title: '' }))).toBe('harbor.jpg')
})

test('picks the rendition closest above the size a grid asks for', () => {
	const sized = item({
		sizes: {
			thumbnail: { file: 'a-150.jpg', width: 150, height: 150, mimeType: 'image/jpeg', filesize: 1 },
			medium: { file: 'a-300.jpg', width: 300, height: 200, mimeType: 'image/jpeg', filesize: 2 },
			large: { file: 'a-1024.jpg', width: 1024, height: 640, mimeType: 'image/jpeg', filesize: 3 },
		},
	})

	expect(bestRendition(sized, 200)).toBe('a-300.jpg')
	expect(bestRendition(sized, 150)).toBe('a-150.jpg')
})

test('falls back to the largest rendition when every one is too small', () => {
	const sized = item({
		sizes: {
			thumbnail: { file: 'a-150.jpg', width: 150, height: 150, mimeType: 'image/jpeg', filesize: 1 },
			medium: { file: 'a-300.jpg', width: 300, height: 200, mimeType: 'image/jpeg', filesize: 2 },
		},
	})

	expect(bestRendition(sized, 4000)).toBe('a-300.jpg')
})

test('falls back to the stored file when there are no renditions', () => {
	expect(bestRendition(item(), 200)).toBe('2026/08/harbor.jpg')
})
