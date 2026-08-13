// SPDX-License-Identifier: Apache-2.0

import { http, HttpResponse, server } from '@gophenberg/frontend-sdk/testing'
import { screen, within } from '@testing-library/react'
import { beforeEach, expect, test } from 'vitest'

import { placeholderType } from '../content/useContentType'
import { renderAt } from './render'

const POST_TYPE = {
	key: 'post',
	singular_label: 'Post',
	plural_label: 'Posts',
	route_word: '',
	hierarchical: false,
	revisions: true,
	revision_cap: 100,
	page_kind: 'single',
	default: true,
	active: true,
}

const PAGE_TYPE = {
	...POST_TYPE,
	key: 'page',
	singular_label: 'Page',
	plural_label: 'Pages',
	route_word: 'pages',
	hierarchical: true,
	default: false,
}

const CLOSED_TYPE = {
	...POST_TYPE,
	key: 'guide',
	singular_label: 'Guide',
	plural_label: 'Guides',
	route_word: 'guides',
	default: false,
	active: false,
}

beforeEach(() => {
	server.use(
		http.get('/api/types', () =>
			HttpResponse.json({ items: [POST_TYPE, PAGE_TYPE, CLOSED_TYPE] }),
		),
		http.get('/api/content', () => HttpResponse.json({ items: [], total: 0 })),
		http.get('/api/content/counts', () => HttpResponse.json({})),
	)
})

test('lists a navigation row for every active type', async () => {
	renderAt('/')

	const menu = await screen.findByRole('navigation', { name: 'Navigation' })

	expect(await within(menu).findByRole('link', { name: 'Posts' })).toBeInTheDocument()
	expect(await within(menu).findByRole('link', { name: 'Pages' })).toBeInTheDocument()
})

test('leaves an inactive type out of the navigation', async () => {
	renderAt('/')

	const menu = await screen.findByRole('navigation', { name: 'Navigation' })
	await within(menu).findByRole('link', { name: 'Pages' })

	expect(within(menu).queryByRole('link', { name: 'Guides' })).not.toBeInTheDocument()
})

test('keeps the sections that do not come from the registry', async () => {
	renderAt('/')

	const menu = await screen.findByRole('navigation', { name: 'Navigation' })
	await within(menu).findByRole('link', { name: 'Pages' })

	expect(within(menu).getByRole('link', { name: 'Media' })).toBeInTheDocument()
	expect(within(menu).getByRole('link', { name: 'Themes' })).toBeInTheDocument()
	expect(within(menu).getByRole('link', { name: 'Content Types' })).toBeInTheDocument()
})

test('stands in with the key the route carries until the registry answers', () => {
	expect(placeholderType('guide').key).toBe('guide')
	expect(placeholderType('guide').pluralLabel).toBe('Content')
	expect(placeholderType(undefined).key).toBe('')
})
