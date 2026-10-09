// SPDX-License-Identifier: Apache-2.0

import { http, HttpResponse, server } from '@gophenberg/frontend-sdk/testing'
import { screen, within } from '@testing-library/react'
import { expect, test, vi } from 'vitest'

import { gate } from './gate'
import { renderAt } from './render'
import { warmPostsScreen } from './warm'

warmPostsScreen()

/**
 * Serves an empty posts listing so the list screen settles.
 */
function servePosts() {
	server.use(http.get('/api/content', () => HttpResponse.json({ items: [], total: 0 })))
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

test('says under the posts title what the type describes itself as', async () => {
	servePosts()

	renderAt('/content/post')

	expect(await screen.findByText('Manage the posts on this site.')).toHaveClass('godmin-page__subtitle')
})

test('says what a list is for in words of its own when the type describes nothing', async () => {
	servePosts()
	server.use(
		http.get('/api/types', () =>
			HttpResponse.json({
				items: [
					{
						key: 'post',
						singular_label: 'Post',
						plural_label: 'Posts',
						description: '  ',
						route_word: '',
						hierarchical: false,
						revisions: true,
						revision_cap: 100,
						page_kind: 'single',
						default: true,
						active: true,
						fields: [],
					},
				],
			}),
		),
	)

	renderAt('/content/post')

	await screen.findByRole('heading', { level: 1, name: 'Posts' })
	expect(screen.getByText('Manage the items of this content type.')).toHaveClass('godmin-page__subtitle')
})

test('says nothing under the posts title until the type registry answers', async () => {
	servePosts()
	const registry = gate()
	server.use(
		http.get('/api/types', async () => {
			await registry.held
			return HttpResponse.json({
				items: [
					{
						key: 'post',
						singular_label: 'Post',
						plural_label: 'Posts',
						description: 'Manage the posts on this site.',
						route_word: '',
						hierarchical: false,
						revisions: true,
						revision_cap: 100,
						page_kind: 'single',
						default: true,
						active: true,
						fields: [],
					},
				],
			})
		}),
	)

	renderAt('/content/post')

	await screen.findByRole('heading', { level: 1, name: 'Content' })
	expect(screen.queryByText('Manage the items of this content type.')).not.toBeInTheDocument()
	registry.release()
	expect(await screen.findByText('Manage the posts on this site.')).toHaveClass('godmin-page__subtitle')
})

test('leaves the rail out of the heading outline', async () => {
	renderAt('/')

	await screen.findByRole('main')
	expect(screen.getAllByRole('heading', { level: 1 })).toHaveLength(1)
})

test('announces a posts listing it could not load in place of the list', async () => {
	vi.spyOn(console, 'error').mockImplementation(() => {})
	server.use(http.get('/api/content', () => HttpResponse.json({}, { status: 500 })))

	renderAt('/content/post')

	expect(await screen.findByRole('alert')).toHaveTextContent('Items could not be loaded.')
	expect(screen.queryByRole('searchbox')).not.toBeInTheDocument()
})

test('keeps the posts heading when the listing fails', async () => {
	vi.spyOn(console, 'error').mockImplementation(() => {})
	server.use(http.get('/api/content', () => HttpResponse.json({}, { status: 500 })))

	renderAt('/content/post')

	await screen.findByRole('alert')
	const main = screen.getByRole('main')
	expect(within(main).getByRole('heading', { level: 1 })).toHaveTextContent('Posts')
})
