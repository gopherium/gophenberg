// SPDX-License-Identifier: Apache-2.0

import { expect, test } from 'vitest'

import { pointersHeld } from '../content/PointersList'

const NEWS = { id: '019fb000-0000-7000-8000-0000000000a1', title: 'News', type: 'category' }

test('reads the items a backlinks field holds', () => {
	expect(pointersHeld([NEWS])).toEqual([NEWS])
})

test('reads nothing when the field holds no list at all', () => {
	expect(pointersHeld(undefined)).toEqual([])
	expect(pointersHeld('a')).toEqual([])
})

test('leaves out an entry carrying no identity of its own', () => {
	expect(pointersHeld([NEWS, 'a', null, {}, { id: NEWS.id }])).toEqual([NEWS])
})

test('names an item by its id when it carries no title', () => {
	expect(pointersHeld([{ id: NEWS.id, type: 'category' }])).toEqual([
		{ id: NEWS.id, title: NEWS.id, type: 'category' },
	])
})
