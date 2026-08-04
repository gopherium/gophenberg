// SPDX-License-Identifier: Apache-2.0

import { describe, expect, expectTypeOf, test } from 'vitest'

import { defineTheme } from '../index.ts'

import type { BlockComponentMap, GophenbergTheme, ThemeLayouts, ThemeSeo } from '../index.ts'
import type { AstroComponentFactory } from 'astro/runtime/server/index.js'

/** A stand-in for the layout components a real theme compiles. */
const layout = (() => undefined) as unknown as AstroComponentFactory

describe('defineTheme', () => {
	test('hands back the theme it was given', () => {
		const declared = {
			layouts: { Base: layout, Post: layout, Archive: layout },
			seo: { siteName: 'Example Site' },
		}

		expect(defineTheme(declared)).toBe(declared)
	})

	test('refuses a theme missing its layouts at compile time', () => {
		// @ts-expect-error a theme without layouts is not a theme
		expect(defineTheme({ seo: { siteName: 'Example Site' } })).toBeDefined()
	})

	test('takes and returns the contract itself', () => {
		expectTypeOf(defineTheme).returns.toEqualTypeOf<GophenbergTheme>()
		expectTypeOf(defineTheme).parameter(0).toEqualTypeOf<GophenbergTheme>()
	})
})

describe('the contract a theme declares', () => {
	test('asks for the three layouts the routes render through, and one the kit falls back for', () => {
		expectTypeOf<ThemeLayouts>().toEqualTypeOf<{
			Base: AstroComponentFactory
			Post: AstroComponentFactory
			Archive: AstroComponentFactory
			NotFound?: AstroComponentFactory
		}>()
	})

	test('requires the layouts and the site name, and nothing else', () => {
		expectTypeOf<GophenbergTheme>().toEqualTypeOf<{
			layouts: ThemeLayouts
			blocks?: BlockComponentMap
			pagination?: { perPage: number }
			seo: ThemeSeo
		}>()
	})

	test('names the site in one field a theme cannot forget', () => {
		expectTypeOf<ThemeSeo>().toEqualTypeOf<{ siteName: string }>()
	})
})
