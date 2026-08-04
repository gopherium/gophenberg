// SPDX-License-Identifier: Apache-2.0

import { afterEach, describe, expect, test, vi } from 'vitest'

import { GophenbergClient } from '../client.ts'
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
 * Returns a fetch double answering with the given body and status, recording the URLs it saw.
 * @param body - What the response carries.
 * @param status - The status the response reports.
 * @returns The double and the URLs it was called with.
 */
function fetchReturning(body: unknown, status = 200) {
	const urls: string[] = []
	const fetch = vi.fn(async (input: string | URL | Request) => {
		urls.push(String(input))
		return new Response(JSON.stringify(body), {
			status,
			headers: { 'Content-Type': 'application/json' },
		})
	})
	return { fetch: fetch as unknown as typeof globalThis.fetch, urls }
}

afterEach(() => {
	vi.unstubAllEnvs()
})

describe('listPosts', () => {
	test('reads the published listing of the default type', async () => {
		const { fetch, urls } = fetchReturning(page)

		const got = await new GophenbergClient({ baseUrl: 'https://example.com', fetch }).listPosts()

		expect(got).toEqual(page)
		expect(urls[0]).toBe('https://example.com/api/content/v1/posts?type=post&page=1&per_page=20')
	})

	test('passes the type and the paging the caller asked for', async () => {
		const { fetch, urls } = fetchReturning(page)

		await new GophenbergClient({ baseUrl: 'https://example.com', fetch }).listPosts({
			type: 'page',
			page: 3,
			perPage: 5,
		})

		expect(urls[0]).toBe('https://example.com/api/content/v1/posts?type=page&page=3&per_page=5')
	})

	test('reports a listing it could not read', async () => {
		const { fetch } = fetchReturning({ error: 'boom' }, 500)

		const reading = new GophenbergClient({ baseUrl: 'https://example.com', fetch }).listPosts()

		await expect(reading).rejects.toThrow('500')
	})
})

describe('getPost', () => {
	test('reads one published post by type and slug', async () => {
		const { fetch, urls } = fetchReturning(post)

		const got = await new GophenbergClient({ baseUrl: 'https://example.com', fetch }).getPost('post', 'hello-world')

		expect(got).toEqual(post)
		expect(urls[0]).toBe('https://example.com/api/content/v1/posts/post/hello-world')
	})

	test('escapes what it puts in the address', async () => {
		const { fetch, urls } = fetchReturning(post)

		await new GophenbergClient({ baseUrl: 'https://example.com', fetch }).getPost('post', 'a slug/../escape')

		expect(urls[0]).toBe('https://example.com/api/content/v1/posts/post/a%20slug%2F..%2Fescape')
	})

	test('answers nothing for a post that is not published', async () => {
		const { fetch } = fetchReturning({ error: 'not found' }, 404)

		const got = await new GophenbergClient({ baseUrl: 'https://example.com', fetch }).getPost('post', 'missing')

		expect(got).toBeUndefined()
	})

	test('reports a post it could not read', async () => {
		const { fetch } = fetchReturning({ error: 'boom' }, 503)

		const reading = new GophenbergClient({ baseUrl: 'https://example.com', fetch }).getPost('post', 'hello-world')

		await expect(reading).rejects.toThrow('503')
	})

	test('reports a transport that failed', async () => {
		const fetch = vi.fn(async () => {
			throw new Error('connection refused')
		})

		const reading = new GophenbergClient({
			baseUrl: 'https://example.com',
			fetch: fetch as unknown as typeof globalThis.fetch,
		}).getPost('post', 'hello-world')

		await expect(reading).rejects.toThrow('connection refused')
	})
})

describe('the address the client reads through', () => {
	test('comes from the environment when the caller names none', async () => {
		vi.stubEnv('GOPHENBERG_API_URL', 'http://127.0.0.1:8081')
		const { fetch, urls } = fetchReturning(page)

		await new GophenbergClient({ fetch }).listPosts()

		expect(urls[0]).toContain('http://127.0.0.1:8081/api/content/v1/posts')
	})

	test('prefers what the caller named', async () => {
		vi.stubEnv('GOPHENBERG_API_URL', 'http://127.0.0.1:8081')
		const { fetch, urls } = fetchReturning(page)

		await new GophenbergClient({ baseUrl: 'https://example.com', fetch }).listPosts()

		expect(urls[0]).toContain('https://example.com/')
	})

	test('is read per request rather than held from construction', async () => {
		vi.stubEnv('GOPHENBERG_API_URL', 'http://first.example.com')
		const { fetch, urls } = fetchReturning(page)
		const client = new GophenbergClient({ fetch })

		await client.listPosts()
		vi.stubEnv('GOPHENBERG_API_URL', 'http://second.example.com')
		await client.listPosts()

		expect(urls[0]).toContain('http://first.example.com/')
		expect(urls[1]).toContain('http://second.example.com/')
	})

	test('is reported missing rather than guessed', async () => {
		vi.stubEnv('GOPHENBERG_API_URL', '')
		const { fetch } = fetchReturning(page)

		const reading = new GophenbergClient({ fetch }).listPosts()

		await expect(reading).rejects.toThrow('GOPHENBERG_API_URL')
	})

	test('drops a trailing slash so the address never doubles it', async () => {
		const { fetch, urls } = fetchReturning(page)

		await new GophenbergClient({ baseUrl: 'https://example.com/', fetch }).listPosts()

		expect(urls[0]).toBe('https://example.com/api/content/v1/posts?type=post&page=1&per_page=20')
	})
})
