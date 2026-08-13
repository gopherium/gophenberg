// SPDX-License-Identifier: Apache-2.0

import { describe, expect, test, vi } from 'vitest'

import { GophenbergClient } from '../client.ts'
import { gophenbergLoader } from '../loader.ts'
import type { ContentType, Page, Post, PostSummary, Resolved } from '../content.ts'

/** A published summary as the content API serves it. */
const summary: PostSummary = {
	id: '019fb000-0000-7000-8000-000000000001',
	type: 'post',
	path: 'hello-world',
	slug: 'hello-world',
	title: 'Hello World',
	excerpt: 'An excerpt.',
	published_at: '2026-08-04T12:00:00Z',
	updated_at: '2026-08-04T12:00:00Z',
}

/** The same post carrying its block markup. */
const post: Post = { ...summary, content: '<!-- wp:paragraph --><p>Body</p><!-- /wp:paragraph -->' }

/** The default content type as the handshake advertises it. */
const postType: ContentType = {
	key: 'post',
	singular_label: 'Post',
	plural_label: 'Posts',
	route_word: '',
	hierarchical: false,
	page_kind: 'single',
	default: true,
}

/** A page holding one summary. */
const page: Page<PostSummary> = { items: [summary], total: 1, page: 1, per_page: 20 }

/**
 * Returns a loader reading through a client double.
 * @param behaviour - What the doubled client answers.
 * @returns The loader and the doubled methods.
 */
function loaderOver(behaviour: {
	listPosts?: () => Promise<Page<PostSummary>>
	resolve?: () => Promise<Resolved | undefined>
}) {
	const listPosts = vi.fn(behaviour.listPosts ?? (async () => page))
	const resolve = vi.fn(behaviour.resolve ?? (async () => ({ kind: 'item', type: postType, item: post })))
	const client = { listPosts, resolve } as unknown as GophenbergClient
	return { loader: gophenbergLoader({ client }), listPosts, resolve }
}

describe('the loader itself', () => {
	test('names the package it comes from', () => {
		expect(loaderOver({}).loader.name).toBe('@gophenberg/astro')
	})
})

describe('loadCollection', () => {
	test('serves the published summaries as live entries', async () => {
		const { loader } = loaderOver({})

		const got = await loader.loadCollection({ collection: 'posts' })

		expect(got).toEqual({ entries: [{ id: 'post/hello-world', data: summary }] })
	})

	test('narrows by the filter a page asked for', async () => {
		const { loader, listPosts } = loaderOver({})

		await loader.loadCollection({ collection: 'posts', filter: { type: 'page', page: 2, perPage: 5 } })

		expect(listPosts).toHaveBeenCalledWith({ type: 'page', page: 2, perPage: 5 })
	})

	test('reports a listing it could not read', async () => {
		const { loader } = loaderOver({
			listPosts: async () => {
				throw new Error('connection refused')
			},
		})

		const got = await loader.loadCollection({ collection: 'posts' })

		expect(got).toHaveProperty('error')
		expect((got as { error: Error }).error.message).toContain('connection refused')
	})
})

describe('loadEntry', () => {
	test('serves the published post as a live entry', async () => {
		const { loader } = loaderOver({})

		const got = await loader.loadEntry({ collection: 'posts', filter: { path: 'hello-world' } })

		expect(got).toEqual({ id: 'post/hello-world', data: post })
	})

	test('asks for the address the page carried', async () => {
		const { loader, resolve } = loaderOver({})

		await loader.loadEntry({ collection: 'posts', filter: { path: 'pages/about-us' } })

		expect(resolve).toHaveBeenCalledWith('pages/about-us')
	})

	test('answers nothing when no post is published there', async () => {
		const { loader } = loaderOver({ resolve: async () => undefined })

		const got = await loader.loadEntry({ collection: 'posts', filter: { path: 'missing' } })

		expect(got).toBeUndefined()
	})

	test('reports a post it could not read', async () => {
		const { loader } = loaderOver({
			resolve: async () => {
				throw new Error('connection refused')
			},
		})

		const got = await loader.loadEntry({ collection: 'posts', filter: { path: 'hello-world' } })

		expect(got).toHaveProperty('error')
	})

	test('reports a failure that was not thrown as an error', async () => {
		const { loader } = loaderOver({
			resolve: async () => {
				throw 'connection refused'
			},
		})

		const got = await loader.loadEntry({ collection: 'posts', filter: { path: 'hello-world' } })

		expect((got as { error: Error }).error).toBeInstanceOf(Error)
		expect((got as { error: Error }).error.message).toBe('connection refused')
	})

	test('reads the address the page carried', async () => {
		const { loader, resolve } = loaderOver({})

		await loader.loadEntry({ collection: 'posts', filter: { path: 'hello-world' } })

		expect(resolve).toHaveBeenCalledWith('hello-world')
	})
})

describe('the client a loader reads through', () => {
	test('is built from the options when the caller passes none', () => {
		expect(gophenbergLoader().name).toBe('@gophenberg/astro')
	})
})
