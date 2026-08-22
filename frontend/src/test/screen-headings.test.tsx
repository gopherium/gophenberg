// SPDX-License-Identifier: Apache-2.0

import { http, HttpResponse, server } from '@gophenberg/frontend-sdk/testing'
import { screen, within } from '@testing-library/react'
import { expect, test, vi } from 'vitest'

import { renderAt } from './render'
import { warmPostsScreen } from './warm'

warmPostsScreen()

/**
 * Serves an empty posts listing so the list screen settles.
 */
function servePosts() {
	server.use(
		http.get('/api/content', () => HttpResponse.json({ items: [], total: 0 })),
		http.get('/api/content/counts', () => HttpResponse.json({})),
	)
}

test('names the home screen with its only first level heading', async () => {
	renderAt('/')

	const main = await screen.findByRole('main')
	expect(within(main).getByRole('heading', { level: 1 })).toHaveTextContent('Home')
	expect(within(main).getAllByRole('heading', { level: 1 })).toHaveLength(1)
})

test('names the posts screen with its only first level heading', async () => {
	servePosts()

	renderAt('/content/post')

	const main = await screen.findByRole('main')
	expect(await within(main).findByRole('heading', { level: 1 })).toHaveTextContent('Posts')
	expect(within(main).getAllByRole('heading', { level: 1 })).toHaveLength(1)
})

test('leaves the rail out of the heading outline', async () => {
	renderAt('/')

	await screen.findByRole('main')
	expect(screen.getAllByRole('heading', { level: 1 })).toHaveLength(1)
})

test('announces a posts listing it could not load', async () => {
	vi.spyOn(console, 'error').mockImplementation(() => {})
	server.use(
		http.get('/api/content', () => HttpResponse.json({}, { status: 500 })),
		http.get('/api/content/counts', () => HttpResponse.json({})),
	)

	renderAt('/content/post')

	expect(await screen.findByRole('alert')).toHaveTextContent(/could not load posts/i)
})

test('keeps the posts heading when the listing fails', async () => {
	vi.spyOn(console, 'error').mockImplementation(() => {})
	server.use(
		http.get('/api/content', () => HttpResponse.json({}, { status: 500 })),
		http.get('/api/content/counts', () => HttpResponse.json({})),
	)

	renderAt('/content/post')

	await screen.findByRole('alert')
	const main = screen.getByRole('main')
	expect(within(main).getByRole('heading', { level: 1 })).toHaveTextContent('Posts')
})
