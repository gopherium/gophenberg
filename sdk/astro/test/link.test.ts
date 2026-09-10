// SPDX-License-Identifier: Apache-2.0

import { describe, expect, test } from 'vitest'

import { linkFields, linkValue } from '../index.ts'
import type { LinkValue, Post } from '../index.ts'

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

const SOURCE: LinkValue = { url: 'https://example.com/a', title: 'A page', new_tab: true }

describe('the address a value points at', () => {
	test('reads the three parts a link holds', () => {
		expect(linkValue(SOURCE)).toEqual(SOURCE)
	})

	test('reads a link that points nowhere, so a theme may say so', () => {
		expect(linkValue({ url: '', title: '', new_tab: false })).toEqual({
			url: '',
			title: '',
			new_tab: false,
		})
	})

	test('reads nothing from a value that is not a link', () => {
		expect(linkValue('https://example.com/a')).toBeUndefined()
		expect(linkValue(null)).toBeUndefined()
		expect(linkValue([SOURCE])).toBeUndefined()
	})

	test('reads nothing from an object that is nearly a link', () => {
		expect(linkValue({ url: '/a', title: 'A' })).toBeUndefined()
		expect(linkValue({ url: '/a', title: 'A', new_tab: 'yes' })).toBeUndefined()
		expect(linkValue({ url: '/a', title: 'A', new_tab: false, rel: 'me' })).toBeUndefined()
		expect(linkValue({ url: 7, title: 'A', new_tab: false })).toBeUndefined()
	})
})

describe('the links an item carries', () => {
	test('names each link field and the address it points at', () => {
		const held = linkFields(posting({ colour: 'red', source: SOURCE }))

		expect(held).toEqual([{ key: 'source', link: SOURCE }])
	})

	test('leaves out a link that points nowhere', () => {
		expect(linkFields(posting({ source: { url: '', title: 'A page', new_tab: false } }))).toEqual([])
	})

	test('names the links in the order their keys read', () => {
		const held = linkFields(posting({ source: SOURCE, appendix: { ...SOURCE, url: '/b' } }))

		expect(held.map((field) => field.key)).toEqual(['appendix', 'source'])
	})
})
