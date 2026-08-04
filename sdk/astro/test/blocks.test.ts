// SPDX-License-Identifier: Apache-2.0

import { describe, expect, test } from 'vitest'

import { parseBlocks, toBlockNode, verbatimSegments } from '../blocks.ts'

describe('parseBlocks', () => {
	test('reads a block and the markup it saved', () => {
		const got = parseBlocks('<!-- wp:paragraph --><p>Body</p><!-- /wp:paragraph -->')

		expect(got).toHaveLength(1)
		expect(got[0].name).toBe('core/paragraph')
		expect(got[0].innerHTML).toBe('<p>Body</p>')
		expect(got[0].innerBlocks).toEqual([])
	})

	test('reads the attributes the editor serialized', () => {
		const got = parseBlocks('<!-- wp:group {"layout":{"type":"constrained"}} --><div></div><!-- /wp:group -->')

		expect(got[0].attrs).toEqual({ layout: { type: 'constrained' } })
	})

	test('nests a child inside the wrapper its parent saved', () => {
		const document =
			'<!-- wp:group --><div class="g"><!-- wp:paragraph --><p>Hi</p><!-- /wp:paragraph --></div><!-- /wp:group -->'

		const got = parseBlocks(document)

		expect(got[0].name).toBe('core/group')
		expect(got[0].innerContent).toEqual(['<div class="g">', null, '</div>'])
		expect(got[0].innerBlocks[0].name).toBe('core/paragraph')
	})

	test('names freeform content nothing', () => {
		const got = parseBlocks('just text')

		expect(got[0].name).toBeNull()
		expect(got[0].innerHTML).toBe('just text')
	})

	test('reads a block that saved no markup at all', () => {
		const got = parseBlocks('<!-- wp:spacer {"height":"40px"} /-->')

		expect(got[0].name).toBe('core/spacer')
		expect(got[0].innerContent).toEqual([])
	})

	test('reads nothing out of an empty document', () => {
		expect(parseBlocks('')).toEqual([])
	})
})

describe('toBlockNode', () => {
	test('reads a block the parser reported without attributes', () => {
		const got = toBlockNode({
			blockName: 'core/paragraph',
			attrs: null,
			innerHTML: '<p>Body</p>',
			innerContent: ['<p>Body</p>'],
			innerBlocks: [],
		})

		expect(got.attrs).toEqual({})
	})
})

describe('verbatimSegments', () => {
	test('emits the markup a leaf block saved', () => {
		const [block] = parseBlocks('<!-- wp:paragraph --><p>Body</p><!-- /wp:paragraph -->')

		expect(verbatimSegments(block)).toEqual([{ html: '<p>Body</p>' }])
	})

	test('interleaves the wrapper around the child it wraps', () => {
		const document =
			'<!-- wp:group --><div class="g"><!-- wp:paragraph --><p>Hi</p><!-- /wp:paragraph --></div><!-- /wp:group -->'
		const [group] = parseBlocks(document)

		const got = verbatimSegments(group)

		expect(got).toHaveLength(3)
		expect(got[0]).toEqual({ html: '<div class="g">' })
		expect(got[1]).toEqual({ child: group.innerBlocks[0] })
		expect(got[2]).toEqual({ html: '</div>' })
	})

	test('keeps two children in the order they were saved', () => {
		const document =
			'<!-- wp:group --><div>' +
			'<!-- wp:paragraph --><p>One</p><!-- /wp:paragraph -->' +
			'<!-- wp:paragraph --><p>Two</p><!-- /wp:paragraph -->' +
			'</div><!-- /wp:group -->'
		const [group] = parseBlocks(document)

		const got = verbatimSegments(group)

		expect(got).toEqual([
			{ html: '<div>' },
			{ child: group.innerBlocks[0] },
			{ child: group.innerBlocks[1] },
			{ html: '</div>' },
		])
	})

	test('emits nothing for a block that saved no markup', () => {
		const [spacer] = parseBlocks('<!-- wp:spacer {"height":"40px"} /-->')

		expect(verbatimSegments(spacer)).toEqual([])
	})

	test('emits nothing for a slot with no child behind it', () => {
		const orphan = { name: 'core/group', attrs: {}, innerHTML: '', innerContent: [null], innerBlocks: [] }

		expect(verbatimSegments(orphan)).toEqual([])
	})
})
