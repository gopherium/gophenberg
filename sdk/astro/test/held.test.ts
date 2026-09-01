// SPDX-License-Identifier: Apache-2.0

import { describe, expect, test } from 'vitest'

import { heldRows, heldSection, heldValue } from '../index.ts'
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

describe('the values a section holds', () => {
	test('reads the values under the key', () => {
		const post = posting({ author: { name: 'Maria Perez', bio: 'Writes here.' } })

		expect(heldSection(post, 'author')).toEqual({ name: 'Maria Perez', bio: 'Writes here.' })
	})

	test('holds nothing for a key the item does not carry', () => {
		expect(heldSection(posting({}), 'author')).toBeUndefined()
	})

	test('holds nothing for a value that is not a section', () => {
		expect(heldSection(posting({ author: 'Maria Perez' }), 'author')).toBeUndefined()
		expect(heldSection(posting({ author: [{ name: 'Maria Perez' }] }), 'author')).toBeUndefined()
		expect(heldSection(posting({ author: null }), 'author')).toBeUndefined()
	})
})

describe('the rows a repeater holds', () => {
	test('reads every row under the key', () => {
		const post = posting({ team: [{ name: 'Maria Perez' }, { name: 'Kip' }] })

		expect(heldRows(post, 'team')).toEqual([{ name: 'Maria Perez' }, { name: 'Kip' }])
	})

	test('holds no rows for a key the item does not carry', () => {
		expect(heldRows(posting({}), 'team')).toEqual([])
	})

	test('holds no rows for a value that is not a list of them', () => {
		expect(heldRows(posting({ team: 'Maria Perez' }), 'team')).toEqual([])
	})

	test('leaves out a member that is not a row', () => {
		const post = posting({ team: [{ name: 'Maria Perez' }, 'stray', null] })

		expect(heldRows(post, 'team')).toEqual([{ name: 'Maria Perez' }])
	})
})

describe('the value a path addresses', () => {
	const post = posting({
		author: { name: 'Maria Perez' },
		team: [{ contact: { phone: '184467235' } }, { contact: { phone: '184467236' } }],
	})

	test('reads a value inside a section', () => {
		expect(heldValue(post, ['author', 'name'])).toBe('Maria Perez')
	})

	test('reads a value inside a numbered row', () => {
		expect(heldValue(post, ['team', 1, 'contact', 'phone'])).toBe('184467236')
	})

	test('reads the whole item value for a single key', () => {
		expect(heldValue(post, ['author'])).toEqual({ name: 'Maria Perez' })
	})

	test('holds nothing for a path the item does not carry', () => {
		expect(heldValue(post, ['absent'])).toBeUndefined()
		expect(heldValue(post, ['author', 'absent'])).toBeUndefined()
		expect(heldValue(post, ['team', 9, 'contact'])).toBeUndefined()
		expect(heldValue(post, ['author', 0])).toBeUndefined()
		expect(heldValue(post, ['team', 'contact'])).toBeUndefined()
		expect(heldValue(post, [])).toBeUndefined()
	})
})
