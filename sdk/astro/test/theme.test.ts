// SPDX-License-Identifier: Apache-2.0

import type { AstroComponentFactory } from 'astro/runtime/server/index.js'
import { describe, expect, expectTypeOf, test } from 'vitest'

import { defineTheme } from '../index.ts'
import type {
	BlockComponentMap,
	ContentType,
	ContentTypeField,
	GophenbergTheme,
	Post,
	Resolved,
	ThemeLayouts,
	ThemeSeo,
} from '../index.ts'

/** A stand-in for the layout components a real theme compiles. */
const layout = (() => undefined) as unknown as AstroComponentFactory

describe('defineTheme', () => {
	test('hands back the theme it was given', () => {
		const declared = {
			layouts: { Base: layout, Post: layout, Archive: layout, Term: layout },
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
	test('asks for the four layouts the routes render through, and one the kit falls back for', () => {
		expectTypeOf<ThemeLayouts>().toEqualTypeOf<{
			Base: AstroComponentFactory
			Post: AstroComponentFactory
			Archive: AstroComponentFactory
			Term: AstroComponentFactory
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

describe('what a public address holds', () => {
	test('names a term page beside an item and an archive', () => {
		expectTypeOf<Resolved['kind']>().toEqualTypeOf<'item' | 'archive' | 'term'>()
	})

	test('carries the item and its page together for a term', () => {
		const held: Resolved = {
			kind: 'term',
			type: {
				key: 'category',
				singular_label: 'Category',
				plural_label: 'Categories',
				route_word: 'categories',
				hierarchical: false,
				page_kind: 'archive',
				default: false,
				fields: [],
			},
			item: {
				id: '1',
				type: 'category',
				path: 'categories/news',
				slug: 'news',
				title: 'News',
				excerpt: '',
				content: '',
				fields: {},
				published_at: '',
				updated_at: '',
			},
			page: { items: [], total: 0, page: 1, per_page: 10 },
		}

		expect(held.item?.title).toBe('News')
		expect(held.page?.total).toBe(0)
	})

	test('carries the values an item holds under its declared fields', () => {
		expectTypeOf<Post['fields']>().toEqualTypeOf<Record<string, unknown>>()
	})

	test('advertises the fields a type declares', () => {
		expectTypeOf<ContentType['fields']>().toEqualTypeOf<ContentTypeField[]>()
	})
})
