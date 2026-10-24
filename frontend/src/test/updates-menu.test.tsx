// SPDX-License-Identifier: Apache-2.0

import { CHANGE_OTHERS_WORK } from '@gophenberg/frontend-sdk'
import { http, HttpResponse, server } from '@gophenberg/frontend-sdk/testing'
import { LIST_CHROME_DOMAIN, listChromeCatalogFor } from '@gopherium/godmin'
import { defaultUser } from '@gopherium/react-auth/testing'
import { screen, within } from '@testing-library/react'
import { resetLocaleData, setLocaleData } from '@wordpress/i18n'
import { beforeEach, expect, onTestFinished, test } from 'vitest'

import { catalogFor } from '../i18n/catalog'
import { renderAt } from './render'
import { patchOffer, patchWaiting, serveUpdates, updatesAnswered } from './updatesOverview'
import { warmPostsScreen } from './warm'

warmPostsScreen()

beforeEach(() => server.use(http.get('/api/content', () => HttpResponse.json({ items: [], total: 0 }))))

/**
 * Returns the sidebar once its rows have arrived.
 * @param name - The name the sidebar region goes by.
 * @returns The sidebar region.
 */
async function menu(name = 'Navigation'): Promise<HTMLElement> {
	const nav = await screen.findByRole('region', { name })
	await within(nav).findByRole('link', { name: /Posts|Entradas/ })
	return nav
}

test('puts Updates after Settings, opening the Updates screen', async () => {
	renderAt('/content/post')

	const nav = await menu()
	const targets = within(nav)
		.getAllByRole('link')
		.map((link) => link.getAttribute('href'))

	expect(within(nav).getByRole('link', { name: 'Updates' })).toHaveAttribute('href', '/admin/update-core')
	expect(targets.indexOf('/admin/update-core')).toBe(targets.indexOf('/admin/settings') + 1)
})

test('counts the update that waits beside the Updates label, as WordPress does', async () => {
	serveUpdates(patchWaiting)
	renderAt('/content/post')

	const nav = await menu()
	const updates = await within(nav).findByRole('link', { name: 'Updates 1 update available' })

	expect(within(updates).getByText('1')).toHaveAttribute('aria-hidden', 'true')
})

test('counts nothing while no update is on offer', async () => {
	const client = renderAt('/content/post')

	const nav = await menu()
	await updatesAnswered(client)

	expect(within(nav).getByRole('link', { name: 'Updates' })).toBeInTheDocument()
	expect(nav.querySelector('.godmin-menu-count')).toBeNull()
})

test('counts nothing while the offer names a version this site refused', async () => {
	serveUpdates({ ...patchWaiting, refused: [patchOffer.version] })
	const client = renderAt('/content/post')

	const nav = await menu()
	await updatesAnswered(client)

	expect(within(nav).getByRole('link', { name: 'Updates' })).toBeInTheDocument()
	expect(nav.querySelector('.godmin-menu-count')).toBeNull()
})

test('asks a reader without update_core nothing about updates and shows it no Updates row', async () => {
	let asked = false
	server.use(
		http.get('/api/updates', () => {
			asked = true
			return HttpResponse.json(patchWaiting)
		}),
	)
	renderAt('/content/post', { ...defaultUser, role: 'editor', capabilities: [CHANGE_OTHERS_WORK] })

	const nav = await menu()

	expect(within(nav).queryByRole('link', { name: /Updates/ })).toBeNull()
	expect(asked).toBe(false)
})

test('names the Updates row and its count in the words of WordPress es_ES for a Spanish reader', async () => {
	setLocaleData(await catalogFor('es-ES'), 'gophenberg')
	setLocaleData(await listChromeCatalogFor('es-ES'), LIST_CHROME_DOMAIN)
	onTestFinished(() => {
		resetLocaleData({}, 'gophenberg')
		resetLocaleData({}, LIST_CHROME_DOMAIN)
	})
	serveUpdates(patchWaiting)
	renderAt('/content/post')

	const nav = await menu('Navegación')

	expect(
		await within(nav).findByRole('link', { name: 'Actualizaciones 1 actualización disponible' }),
	).toHaveAttribute('href', '/admin/update-core')
})
