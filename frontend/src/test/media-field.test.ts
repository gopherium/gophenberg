// SPDX-License-Identifier: Apache-2.0

import { expect, test } from 'vitest'

import { galleryHeld, mediaHeld, pickedMedia } from '../content/MediaField'

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
