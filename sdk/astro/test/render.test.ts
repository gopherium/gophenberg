// SPDX-License-Identifier: Apache-2.0

import { experimental_AstroContainer as AstroContainer } from 'astro/container'
import { describe, expect, test } from 'vitest'

import Blocks from '../components/Blocks.astro'
import Quote from './fixtures/Quote.astro'
import Wrapper from './fixtures/Wrapper.astro'

/**
 * Returns the markup a document renders to.
 * @param html - The stored block markup.
 * @param components - The components overriding block names.
 * @returns The rendered HTML.
 */
async function render(html: string, components: Record<string, unknown> = {}): Promise<string> {
	const container = await AstroContainer.create()
	return container.renderToString(Blocks, { props: { html, components } })
}

describe('rendering a document', () => {
	test('serves the markup a block saved', async () => {
		const got = await render('<!-- wp:paragraph --><p>Body</p><!-- /wp:paragraph -->')

		expect(got).toContain('<p>Body</p>')
	})

	test('keeps the wrapper around the child it wraps', async () => {
		const document =
			'<!-- wp:group --><div class="g"><!-- wp:paragraph --><p>Hi</p><!-- /wp:paragraph --></div><!-- /wp:group -->'

		const got = await render(document)

		expect(got).toContain('<div class="g"><p>Hi</p></div>')
	})

	test('keeps two children in the order they were saved', async () => {
		const document =
			'<!-- wp:group --><div>' +
			'<!-- wp:paragraph --><p>One</p><!-- /wp:paragraph -->' +
			'<!-- wp:paragraph --><p>Two</p><!-- /wp:paragraph -->' +
			'</div><!-- /wp:group -->'

		const got = await render(document)

		expect(got).toContain('<div><p>One</p><p>Two</p></div>')
	})

	test('serves freeform content as it was stored', async () => {
		const got = await render('<p>No delimiters here</p>')

		expect(got).toContain('<p>No delimiters here</p>')
	})

	test('serves a block it has never heard of verbatim', async () => {
		const got = await render('<!-- wp:future/block --><aside>Tomorrow</aside><!-- /wp:future/block -->')

		expect(got).toContain('<aside>Tomorrow</aside>')
	})

	test('leaves a block that saved no markup out of the page', async () => {
		const got = await render('<!-- wp:spacer {"height":"40px"} /-->')

		expect(got.trim()).toBe('')
	})

	test('carries no delimiter into the page', async () => {
		const got = await render('<!-- wp:paragraph --><p>Body</p><!-- /wp:paragraph -->')

		expect(got).not.toContain('wp:')
	})
})

describe('a component overriding a block', () => {
	test('replaces only the block it names', async () => {
		const document =
			'<!-- wp:quote --><blockquote>Quoted</blockquote><!-- /wp:quote -->' +
			'<!-- wp:paragraph --><p>Body</p><!-- /wp:paragraph -->'

		const got = await render(document, { 'core/quote': Quote })

		expect(got).toContain('<figure class="pulled">')
		expect(got).not.toContain('<blockquote>Quoted</blockquote>')
		expect(got).toContain('<p>Body</p>')
	})

	test('reads the attributes the editor saved', async () => {
		const document = '<!-- wp:quote {"citation":"Maria Perez"} --><blockquote>Q</blockquote><!-- /wp:quote -->'

		const got = await render(document, { 'core/quote': Quote })

		expect(got).toContain('Maria Perez')
	})

	test('renders the children it chooses to keep', async () => {
		const document =
			'<!-- wp:group --><div class="ignored">' +
			'<!-- wp:paragraph --><p>Inner</p><!-- /wp:paragraph -->' +
			'</div><!-- /wp:group -->'

		const got = await render(document, { 'core/group': Wrapper })

		expect(got).toContain('<section class="wrapped">')
		expect(got).not.toContain('class="ignored"')
		expect(got).toContain('<p>Inner</p>')
	})

	test('applies to a child as well as a top level block', async () => {
		const document =
			'<!-- wp:group --><div>' +
			'<!-- wp:quote --><blockquote>Deep</blockquote><!-- /wp:quote -->' +
			'</div><!-- /wp:group -->'

		const got = await render(document, { 'core/quote': Quote })

		expect(got).toContain('<figure class="pulled">')
	})
})
