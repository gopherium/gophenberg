// SPDX-License-Identifier: Apache-2.0

import { http, HttpResponse, server } from '@gophenberg/frontend-sdk/testing'
import { setViewport } from '@gopherium/godmin/testing'
import { screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { beforeEach, expect, test } from 'vitest'

import { renderAt, renderRoutedAt } from './render'

/** The name godmin gives the button that opens the sidebar on a phone, as WordPress's new admin names it. */
const MENU_BUTTON = 'Open navigation panel'

beforeEach(() => {
	server.use(
		http.get('/api/content', () => HttpResponse.json({ items: [], total: 0 })),
		http.get('/api/media', () => HttpResponse.json({ items: [], total: 0, per_page: 20 })),
	)
})

test('holds the navigation behind a menu button on a narrow viewport', async () => {
	setViewport({ matches: true })

	renderAt('/')

	expect(await screen.findByRole('button', { name: MENU_BUTTON })).toBeInTheDocument()
	expect(screen.queryByRole('region', { name: 'Navigation' })).toBeNull()
})

test('opens the navigation in a drawer named for it, holding no region of its own', async () => {
	setViewport({ matches: true })
	renderAt('/')

	await userEvent.click(await screen.findByRole('button', { name: MENU_BUTTON }))

	const drawer = await screen.findByRole('dialog', { name: 'Navigation' })
	expect(await within(drawer).findByRole('link', { name: 'Posts' })).toBeInTheDocument()
	expect(within(drawer).queryByRole('region')).toBeNull()
})

test('puts the navigation away once it takes the reader somewhere', async () => {
	setViewport({ matches: true })
	renderAt('/media')
	await userEvent.click(await screen.findByRole('button', { name: MENU_BUTTON }))
	const drawer = await screen.findByRole('dialog')

	await userEvent.click(await within(drawer).findByRole('link', { name: 'Posts' }))

	expect(await screen.findByRole('button', { name: MENU_BUTTON })).toBeInTheDocument()
	await waitFor(() => expect(screen.queryByRole('dialog')).toBeNull())
})

/**
 * Opens the drawer on a phone over the Posts list at the path, and follows its Posts row.
 * @param path - The address the reader starts on.
 * @returns The router and the drawer the row was followed from.
 */
async function openPostsFromDrawer(path: string) {
	setViewport({ matches: true })
	const { router } = renderRoutedAt(path)
	await screen.findByRole('heading', { name: 'Posts', level: 2 })
	await userEvent.click(await screen.findByRole('button', { name: MENU_BUTTON }))
	const drawer = await screen.findByRole('dialog')
	await userEvent.click(await within(drawer).findByRole('link', { name: 'Posts' }))
	return { router, drawer }
}

test('puts the navigation away when its row leaves a status view of the same list', async () => {
	const { router } = await openPostsFromDrawer('/content/post?status=trash')

	await waitFor(() => expect(router.state.location.searchStr).toBe(''))
	await waitFor(() => expect(screen.queryByRole('dialog')).toBeNull())
})

test('keeps the navigation open when its row names the list the root already opened', async () => {
	const { router, drawer } = await openPostsFromDrawer('/')
	await waitFor(() => expect(router.state.status).toBe('idle'))

	expect(router.state.location.pathname).toBe('/content/post')
	expect(screen.getByRole('dialog')).toBe(drawer)
})

test('keeps the sidebar beside the canvas when there is room', async () => {
	setViewport({ matches: false })

	renderAt('/')

	expect(await screen.findByRole('region', { name: 'Navigation' })).toBeInTheDocument()
	expect(screen.queryByRole('button', { name: MENU_BUTTON })).toBeNull()
})
