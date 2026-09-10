// SPDX-License-Identifier: Apache-2.0

import { describe, expect, test } from 'vitest'

import { pointingFields, pointingItems, relatedFields, relatedItems } from '../index.ts'
import type { Pointer, Post } from '../index.ts'

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

const NEWS: Pointer = { id: 'a1', title: 'News', path: 'categories/news', type: 'category' }

const CATEGORY = { id: 'a1', title: 'News', path: 'categories/news' }

describe('the items pointing at one', () => {
	test('reads the items a linked from value holds', () => {
		expect(pointingItems([NEWS])).toEqual([NEWS])
	})

	test('reads an empty list as nothing pointing here yet', () => {
		expect(pointingItems([])).toEqual([])
	})

	test('reads nothing from a value that is not a list of pointers', () => {
		expect(pointingItems('news')).toBeUndefined()
		expect(pointingItems([CATEGORY])).toBeUndefined()
		expect(pointingItems([null])).toBeUndefined()
		expect(pointingItems([{ ...NEWS, type: 7 }])).toBeUndefined()
	})

	test('names each linked from field and what points through it', () => {
		const held = pointingFields(posting({ colour: 'red', 'linked-from': [NEWS] }))

		expect(held).toEqual([{ key: 'linked-from', items: [NEWS] }])
	})

	test('leaves out a field nothing points at', () => {
		expect(pointingFields(posting({ 'linked-from': [] }))).toEqual([])
	})
})

describe('a relation and what points back never read as each other', () => {
	test('an item pointing this way is never read as one a relation points at', () => {
		expect(relatedItems([NEWS])).toBeUndefined()
		expect(relatedFields(posting({ 'linked-from': [NEWS] }))).toEqual([])
	})

	test('an item a relation points at is never read as one pointing this way', () => {
		expect(pointingItems([CATEGORY])).toBeUndefined()
		expect(pointingFields(posting({ categories: [CATEGORY] }))).toEqual([])
	})
})
