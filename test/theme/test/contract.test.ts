// SPDX-License-Identifier: Apache-2.0

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
		expect(theme.pagination?.perPage).toBe(10)
	})
})
