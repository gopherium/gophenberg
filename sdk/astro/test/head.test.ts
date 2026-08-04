// SPDX-License-Identifier: Apache-2.0

import { experimental_AstroContainer as AstroContainer } from 'astro/container'
import { afterEach, describe, expect, test, vi } from 'vitest'

import GophenbergHead from '../components/GophenbergHead.astro'

/**
 * Returns the head markup a page renders.
 * @param props - What the page passed the component.
 * @returns The rendered HTML.
 */
async function renderHead(props: Record<string, unknown> = {}): Promise<string> {
	const container = await AstroContainer.create()
	return container.renderToString(GophenbergHead, { props })
}

afterEach(() => {
	vi.unstubAllEnvs()
})

describe('what a themed page says about itself', () => {
	test('names what served it at the version a survey charts', async () => {
		const got = await renderHead()

		expect(got).toContain('<meta name="generator" content="Gophenberg')
	})

	test('loads the stylesheets a stored block was written against', async () => {
		const got = await renderHead()

		expect(got).toContain('href="/gophenberg/blocks.css"')
		expect(got).toContain('href="/gophenberg/presets.css"')
	})

	test('points at the icon and the feed', async () => {
		const got = await renderHead()

		expect(got).toContain('href="/gophenberg/favicon.svg"')
		expect(got).toContain('href="/api/plugins/feed/rss.xml"')
	})

	test('carries the title a page gave it', async () => {
		const got = await renderHead({ title: 'Hello World' })

		expect(got).toContain('<title>Hello World</title>')
	})

	test('leaves the title out when a page names none', async () => {
		expect(await renderHead()).not.toContain('<title>')
	})

	test('escapes a title an author typed', async () => {
		const got = await renderHead({ title: '<script>alert(1)</script>' })

		expect(got).not.toContain('<script>alert(1)</script>')
		expect(got).toContain('&lt;script&gt;')
	})
})

describe('where it points during development', () => {
	test('keeps same-origin paths when nothing says otherwise', async () => {
		const got = await renderHead()

		expect(got).toContain('href="/gophenberg/blocks.css"')
	})

	test('reaches the instance when a theme is served from another origin', async () => {
		vi.stubEnv('GOPHENBERG_ASSET_ORIGIN', 'http://127.0.0.1:8081')

		const got = await renderHead()

		expect(got).toContain('href="http://127.0.0.1:8081/gophenberg/blocks.css"')
		expect(got).toContain('href="http://127.0.0.1:8081/gophenberg/favicon.svg"')
	})

	test('reaches it just the same when the origin was written with a trailing slash', async () => {
		vi.stubEnv('GOPHENBERG_ASSET_ORIGIN', 'http://127.0.0.1:8081/')

		const got = await renderHead()

		expect(got).toContain('href="http://127.0.0.1:8081/gophenberg/blocks.css"')
		expect(got).toContain('href="http://127.0.0.1:8081/api/plugins/feed/rss.xml"')
	})
})
