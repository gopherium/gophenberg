// SPDX-License-Identifier: Apache-2.0

import { beforeAll, expect, test } from 'vitest'

import { CURATED_BLOCKS, parse, registerCuratedBlocks, serialize } from '../editor'

const CORPUS = import.meta.glob('../../../frontend/src/test/fixtures/blocks/*.html', {
	query: '?raw',
	import: 'default',
	eager: true,
}) as Record<string, string>

const FOREIGN = '<p>Markup no block claims.</p>'

beforeAll(() => {
	registerCuratedBlocks()
}, 120000)

/**
 * Returns the corpus as a name and markup pair per fixture.
 * @returns One entry per fixture file.
 */
function corpus(): { name: string, html: string }[] {
	return Object.entries(CORPUS).map(([path, html]) => ({
		name: path.split('/').pop()?.replace('.html', '') ?? path,
		html,
	}))
}

/**
 * Returns the blocks without the identity parse mints on every call.
 * @param blocks - The blocks to strip.
 * @returns The blocks carrying structure only.
 */
function withoutIds(blocks: unknown): unknown {
	if (!Array.isArray(blocks)) {
		return blocks
	}
	return blocks.map((block: Record<string, unknown>) => {
		const { clientId, ...rest } = block
		void clientId
		return { ...rest, innerBlocks: withoutIds(rest.innerBlocks) }
	})
}

test('holds a fixture for every curated block that authors content', () => {
	const covered = new Set(corpus().map((entry) => `core/${entry.name}`))
	const expected = CURATED_BLOCKS.filter((name) => name !== 'core/missing')

	expect([...covered].toSorted()).toEqual([...expected].toSorted())
})

test.each(corpus())('reserializes $name exactly as stored', ({ html }) => {
	expect(serialize(parse(html))).toBe(html)
})

test.each(corpus())('parses $name back to the same blocks it serialized', ({ html }) => {
	const once = parse(html)

	expect(withoutIds(parse(serialize(once)))).toEqual(withoutIds(once))
})

test('reads markup no block claims as the fallback', () => {
	const parsed = parse(FOREIGN) as { name: string }[]

	expect(parsed.map((block) => block.name)).toContain('core/missing')
})

test('settles markup no block claims after one save', () => {
	const once = serialize(parse(FOREIGN))

	expect(serialize(parse(once))).toBe(once)
})

test('keeps the whitespace of markup no block claims', () => {
	const spaced = '<p>Spaced   out\n\n   words.</p>'

	expect(serialize(parse(spaced))).toBe(spaced)
})
