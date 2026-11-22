// SPDX-License-Identifier: Apache-2.0

import { http, HttpResponse, server } from '@gophenberg/frontend-sdk/testing'
import { screen } from '@testing-library/react'
import { expect, test, vi } from 'vitest'

import { createAppRouter } from '../router'
import { renderRoutedAt } from './render'

const settingsCode = await vi.hoisted(async () => (await import('./gate')).gate())

vi.mock('../settings/SettingsScreen', async (importOriginal) => {
	await settingsCode.held
	return importOriginal()
})

const SPLIT_ROUTES = [
	'/framed/content/$typeKey',
	'/framed/admin/content-types',
	'/framed/admin/users',
	'/framed/admin/users/new',
	'/framed/admin/plugins',
	'/content/$typeKey/$postId/edit',
]

/** Longer than the second the router waits before it draws a pending screen, had one been set. */
const PAST_PENDING_DELAY = 1100

test('splits every heavy route behind a component a preload can fetch before the click', () => {
	const router = createAppRouter()

	for (const path of SPLIT_ROUTES) {
		const route = router.routesById[path as keyof typeof router.routesById]
		const component = route.options.component as { preload?: unknown } | undefined
		expect(typeof component?.preload, `${path} cannot be preloaded`).toBe('function')
	}
})

test('keeps the screen on show while the next one arrives, with no loading screen between them', async () => {
	server.use(http.get('/api/content', () => HttpResponse.json({ items: [], total: 0 })))
	const { router } = renderRoutedAt('/content-types')
	await screen.findByRole('heading', { name: 'Content Types', level: 2 })

	const moving = router.navigate({ to: '/settings' })
	await new Promise((resolve) => setTimeout(resolve, PAST_PENDING_DELAY))

	expect(screen.getByRole('heading', { name: 'Content Types', level: 2 })).toBeInTheDocument()
	expect(screen.queryByRole('status')).toBeNull()
	settingsCode.release()
	await moving
	expect(await screen.findByRole('heading', { name: 'Settings', level: 2 })).toBeInTheDocument()
})
