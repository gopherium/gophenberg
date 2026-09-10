// SPDX-License-Identifier: Apache-2.0

import { expect, test } from 'vitest'

import { galleryHeld, mediaHeld, moved, pickedMedia, pickedMediaList } from '../content/MediaField'

test('reads the identities a gallery holds', () => {
	expect(galleryHeld([1, 2])).toEqual([1, 2])
	expect(galleryHeld([1, 'two'])).toEqual([1])
	expect(galleryHeld(undefined)).toEqual([])
	expect(galleryHeld(3)).toEqual([])
})

test('reads the identity a media field holds', () => {
	expect(mediaHeld(12)).toBe(12)
})

test('reads nothing when a media field holds no number', () => {
	expect(mediaHeld(undefined)).toBeUndefined()
	expect(mediaHeld('12')).toBeUndefined()
})

test('takes the first of the attachments the library reported', () => {
	expect(pickedMedia([{ id: 12 }, { id: 13 }])).toBe(12)
})

test('takes the identity of the one attachment the library reported', () => {
	expect(pickedMedia({ id: 12 })).toBe(12)
})

test('reports nothing when the library named no identity', () => {
	expect(pickedMedia(undefined)).toBeNull()
	expect(pickedMedia({ id: 'twelve' })).toBeNull()
	expect(pickedMedia([])).toBeNull()
})

test('takes every attachment the library reported, each once', () => {
	expect(pickedMediaList([{ id: 12 }, { id: 13 }, { id: 12 }, { id: 'x' }])).toEqual([12, 13])
	expect(pickedMediaList({ id: 12 })).toEqual([12])
	expect(pickedMediaList(undefined)).toEqual([])
})

test('moves one file up or down the gallery', () => {
	expect(moved([1, 2, 3], 3, -1)).toEqual([1, 3, 2])
	expect(moved([1, 2, 3], 1, 1)).toEqual([2, 1, 3])
})

test('leaves the gallery alone when a file cannot move that way', () => {
	expect(moved([1, 2, 3], 1, -1)).toEqual([1, 2, 3])
	expect(moved([1, 2, 3], 3, 1)).toEqual([1, 2, 3])
	expect(moved([1, 2, 3], 9, 1)).toEqual([1, 2, 3])
})
