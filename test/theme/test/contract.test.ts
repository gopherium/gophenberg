// SPDX-License-Identifier: Apache-2.0

import { readFileSync } from 'node:fs'

import { describe, expect, expectTypeOf, test } from 'vitest'

import type { GophenbergTheme } from '@gophenberg/astro'

import theme from '../src/theme.ts'

describe('what the starter declares to the kit', () => {
	test('is a theme the contract accepts', () => {
		expectTypeOf(theme).toEqualTypeOf<GophenbergTheme>()
	})

	test('supplies every layout the injected routes render through', () => {
		expect(theme.layouts.Base).toBeDefined()
		expect(theme.layouts.Post).toBeDefined()
		expect(theme.layouts.Archive).toBeDefined()
	})

	test('supplies the not-found layout, so the kit fallback stays unused', () => {
		expect(theme.layouts.NotFound).toBeDefined()
	})

	test('overrides one block, which is what proves the map', () => {
		expect(Object.keys(theme.blocks ?? {})).toEqual(['core/quote'])
	})

	test('names the site and its page size', () => {
		expect(theme.seo.siteName).toBe('Gophenberg Starter')
		expect(theme.pagination?.perPage).toBe(2)
	})
})

describe('what the starter shows of an item', () => {
	const layout = readFileSync(new URL('../src/layouts/Post.astro', import.meta.url), 'utf8')

	test('renders the files a media field names', () => {
		expect(layout).toContain('mediaFields')
		expect(layout).toContain('mediaUrl')
	})

	test('addresses each file and describes it for a reader who cannot see it', () => {
		expect(layout).toContain('alt={item.alt_text}')
		expect(layout).toContain('src={mediaUrl(item.src)}')
	})

	test('gives an image its size so the page does not jump while it loads', () => {
		expect(layout).toContain('width={item.width}')
		expect(layout).toContain('height={item.height}')
	})

	test('links a file that is not an image rather than showing a broken picture', () => {
		expect(layout).toContain("item.mime_type.startsWith('image/')")
		expect(layout).toContain('<a href={mediaUrl(item.src)}')
	})

	test('ties a caption to the file it describes', () => {
		const captioned = layout.slice(layout.indexOf('<figure>'), layout.indexOf('</figure>'))

		expect(captioned).toContain('figcaption')
		expect(captioned).toContain('mediaUrl(item.src)')
	})
})
