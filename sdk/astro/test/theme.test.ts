// SPDX-License-Identifier: Apache-2.0

import { describe, expect, test } from 'vitest'

import { defineTheme } from '../theme.ts'

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
})
