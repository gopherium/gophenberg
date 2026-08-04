// SPDX-License-Identifier: Apache-2.0

import { describe, expect, test, vi } from 'vitest'

import { GophenbergClient } from '../client.ts'
import { gophenbergLoader } from '../loader.ts'
import type { Page, Post, PostSummary } from '../content.ts'

/** A published summary as the content API serves it. */
const summary: PostSummary = {
	id: '019fb000-0000-7000-8000-000000000001',
	type: 'post',
	slug: 'hello-world',
	title: 'Hello World',
	excerpt: 'An excerpt.',
	published_at: '2026-08-04T12:00:00Z',
	updated_at: '2026-08-04T12:00:00Z',
}

/** The same post carrying its block markup. */
const post: Post = { ...summary, content: '<!-- wp:paragraph --><p>Body</p><!-- /wp:paragraph -->' }

/** A page holding one summary. */
const page: Page<PostSummary> = { items: [summary], total: 1, page: 1, per_page: 20 }

/**
 * Returns a loader reading through a client double.
 * @param behaviour - What the doubled client answers.
 * @returns The loader and the doubled methods.
 */
function loaderOver(behaviour: {
	listPosts?: () => Promise<Page<PostSummary>>
	getPost?: () => Promise<Post | undefined>
}) {
	const listPosts = vi.fn(behaviour.listPosts ?? (async () => page))
	const getPost = vi.fn(behaviour.getPost ?? (async () => post))
	const client = { listPosts, getPost } as unknown as GophenbergClient
	return { loader: gophenbergLoader({ client }), listPosts, getPost }
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

		const got = await loader.loadEntry({ collection: 'posts', filter: { type: 'post', slug: 'hello-world' } })

		expect(got).toEqual({ id: 'post/hello-world', data: post })
	})

	test('asks for the type and slug the page addressed', async () => {
		const { loader, getPost } = loaderOver({})

		await loader.loadEntry({ collection: 'posts', filter: { type: 'page', slug: 'about-us' } })

		expect(getPost).toHaveBeenCalledWith('page', 'about-us')
	})

	test('answers nothing when no post is published there', async () => {
		const { loader } = loaderOver({ getPost: async () => undefined })

		const got = await loader.loadEntry({ collection: 'posts', filter: { type: 'post', slug: 'missing' } })

		expect(got).toBeUndefined()
	})

	test('reports a post it could not read', async () => {
		const { loader } = loaderOver({
			getPost: async () => {
				throw new Error('connection refused')
			},
		})

		const got = await loader.loadEntry({ collection: 'posts', filter: { type: 'post', slug: 'hello-world' } })

		expect(got).toHaveProperty('error')
	})

	test('reports a failure that was not thrown as an error', async () => {
		const { loader } = loaderOver({
			getPost: async () => {
				throw 'connection refused'
			},
		})

		const got = await loader.loadEntry({ collection: 'posts', filter: { slug: 'hello-world' } })

		expect((got as { error: Error }).error).toBeInstanceOf(Error)
		expect((got as { error: Error }).error.message).toBe('connection refused')
	})

	test('reads the default type when the page names none', async () => {
		const { loader, getPost } = loaderOver({})

		await loader.loadEntry({ collection: 'posts', filter: { slug: 'hello-world' } })

		expect(getPost).toHaveBeenCalledWith('post', 'hello-world')
	})
})

describe('the client a loader reads through', () => {
	test('is built from the options when the caller passes none', () => {
		expect(gophenbergLoader().name).toBe('@gophenberg/astro')
	})
})
