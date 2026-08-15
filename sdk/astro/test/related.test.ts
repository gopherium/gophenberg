// SPDX-License-Identifier: Apache-2.0

import { describe, expect, test } from 'vitest'

import { relatedFields, relatedItems } from '../index.ts'
import type { Post } from '../index.ts'

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

const NEWS = { id: 'a1', title: 'News', path: 'categories/news' }

describe('the targets a value names', () => {
	test('reads the items a relation value holds', () => {
		expect(relatedItems([NEWS])).toEqual([NEWS])
	})

	test('reads nothing from a value that is not a list of targets', () => {
		expect(relatedItems('red')).toBeUndefined()
		expect(relatedItems(4)).toBeUndefined()
		expect(relatedItems([1, 2])).toBeUndefined()
		expect(relatedItems([{ id: 'a1' }])).toBeUndefined()
	})

	test('reads an empty list as a field holding no targets', () => {
		expect(relatedItems([])).toEqual([])
	})
})

describe('the relations a post carries', () => {
	test('names each relation field and what it points at', () => {
		const held = relatedFields(posting({ color: 'red', categories: [NEWS] }))

		expect(held).toEqual([{ key: 'categories', items: [NEWS] }])
	})

	test('leaves out a relation pointing at nothing', () => {
		expect(relatedFields(posting({ categories: [] }))).toEqual([])
	})

	test('names the relations in the order their keys read', () => {
		const guides = { id: 'a2', title: 'Guides', path: 'categories/guides' }

		const held = relatedFields(posting({ series: [guides], categories: [NEWS] }))

		expect(held.map((field) => field.key)).toEqual(['categories', 'series'])
	})
})
