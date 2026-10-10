// SPDX-License-Identifier: Apache-2.0

import { experimental_AstroContainer as AstroContainer } from 'astro/container'
import { afterEach, describe, expect, test, vi } from 'vitest'

import { GophenbergClient } from '@gophenberg/astro'
import type { Resolved } from '@gophenberg/astro'
import Content from '@gophenberg/astro/routes/content.astro'
import NotFound from '@gophenberg/astro/routes/not-found.astro'

/** The published item the route renders after resolving its address. */
const resolved: Resolved = {
	kind: 'item',
	type: {
		key: 'post',
		singular_label: 'Post',
		plural_label: 'Posts',
		route_word: '',
		hierarchical: false,
		page_kind: 'single',
		default: true,
		fields: [],
	},
	item: {
		id: '019fb000-0000-7000-8000-000000000001',
		type: 'post',
		path: 'hello-world',
		slug: 'hello-world',
		title: 'Hello World',
		excerpt: '',
		published_at: '2026-08-04T12:00:00Z',
		updated_at: '2026-08-04T12:00:00Z',
		content: '<p>Published body</p>',
		fields: {},
	},
}

afterEach(() => {
	vi.restoreAllMocks()
	vi.unstubAllGlobals()
	vi.unstubAllEnvs()
})

describe('the public address the injected content route resolves', () => {
	test.each([
		['the root archive', '/', '/'],
		['an ASCII slug', '/hello-world', '/hello-world'],
		['an accented slug', '/caf%C3%A9', '/café'],
		['a Chinese slug', '/%E4%B8%AD%E6%96%87', '/中文'],
		['a nested slug', '/pages/%E6%97%A5%E6%9C%AC%E8%AA%9E', '/pages/日本語'],
		['a paged term', '/tags/%E4%B8%AD%E6%96%87/page/2', '/tags/中文/page/2'],
		['an encoded slash', '/pages%2Fabout', '/pages/about'],
		['an encoded percent', '/100%25', '/100%'],
		['a single decoding pass', '/caf%25C3%25A9', '/caf%C3%A9'],
		['an incomplete escape', '/broken%2', '/broken%2'],
		['an invalid UTF-8 sequence', '/broken%E9', '/broken%E9'],
	])('passes %s to the content API', async (_name, pathname, expected) => {
		const resolve = vi.spyOn(GophenbergClient.prototype, 'resolve').mockResolvedValue(resolved)
		const container = await AstroContainer.create()

		const html = await container.renderToString(Content, {
			request: new Request(`https://example.com${pathname}`),
		})

		expect(resolve).toHaveBeenCalledWith(expected, { perPage: 2 })
		expect(html).toContain('Published body')
	})

	test('renders a Unicode address through the content client', async () => {
		vi.stubEnv('GOPHENBERG_API_URL', 'https://api.example.com')
		vi.stubGlobal('fetch', vi.fn(async (input: string | URL | Request) => {
			const url = new URL(input instanceof Request ? input.url : input)
			const found = url.searchParams.get('path') === '/中文'
			return new Response(JSON.stringify(found ? resolved : { error: 'not found' }), {
				status: found ? 200 : 404,
				headers: { 'Content-Type': 'application/json' },
			})
		}))
		const container = await AstroContainer.create()
		container.insertPageRoute('/404', NotFound)

		const html = await container.renderToString(Content, {
			request: new Request('https://example.com/%E4%B8%AD%E6%96%87'),
		})

		expect(html).toContain('Published body')
	})
})
