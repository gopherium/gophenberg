// SPDX-License-Identifier: Apache-2.0

import { http, HttpResponse, server } from '@gophenberg/frontend-sdk/testing'
import { screen, within } from '@testing-library/react'
import { beforeEach, expect, test } from 'vitest'

import { renderAt } from './render'
import { warmPostsScreen } from './warm'

warmPostsScreen()

beforeEach(() => {
	server.use(http.get('/api/content', () => HttpResponse.json({ items: [], total: 0 })))
})

/**
 * Returns the sidebar once the posts list has drawn its heading.
 * @returns The sidebar region, named Navigation.
 */
async function sidebarBesidePosts(): Promise<HTMLElement> {
	await screen.findByRole('heading', { name: 'Posts', level: 2 })
	return screen.getByRole('region', { name: 'Navigation' })
}

test('the main menu offers the posts section', async () => {
	renderAt('/content/post')

	expect(await within(await sidebarBesidePosts()).findByRole('link', { name: 'Posts' })).toBeInTheDocument()
})

test('keeps the main menu in the sidebar while the posts list shows', async () => {
	renderAt('/content/post')

	const sidebar = await sidebarBesidePosts()

	expect(await within(sidebar).findByRole('link', { name: 'Users' })).toBeInTheDocument()
	expect(within(sidebar).queryByRole('link', { name: 'Back' })).not.toBeInTheDocument()
})

test('marks the posts row as the current page while its list shows', async () => {
	renderAt('/content/post')

	const row = await within(await sidebarBesidePosts()).findByRole('link', { name: 'Posts' })

	expect(row).toHaveAttribute('aria-current', 'page')
})

test('adds from the list header alone, the sidebar offering no row of its own', async () => {
	renderAt('/content/post')

	const sidebar = await sidebarBesidePosts()
	await within(sidebar).findByRole('link', { name: 'Posts' })

	expect(within(sidebar).queryByRole('link', { name: 'All Posts' })).not.toBeInTheDocument()
	expect(within(sidebar).queryByRole('button', { name: 'Add Post' })).not.toBeInTheDocument()
	expect(within(screen.getByRole('main')).getAllByRole('button', { name: 'Add Post' })).not.toHaveLength(0)
})
