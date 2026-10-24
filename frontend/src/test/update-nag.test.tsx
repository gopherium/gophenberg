// SPDX-License-Identifier: Apache-2.0

import { ACCESS_ADMIN, CHANGE_OTHERS_WORK } from '@gophenberg/frontend-sdk'
import { http, HttpResponse, server } from '@gophenberg/frontend-sdk/testing'
import { defaultUser, seedSession } from '@gopherium/react-auth/testing'
import { act, screen, waitFor, within } from '@testing-library/react'
import { resetLocaleData, setLocaleData } from '@wordpress/i18n'
import { beforeEach, expect, onTestFinished, test } from 'vitest'

import { catalogFor } from '../i18n/catalog'
import { adminUser, renderAt } from './render'
import { patchFailed, patchOffer, patchWaiting, serveUpdates, updatesAnswered } from './updatesOverview'
import { warmPostsScreen } from './warm'

warmPostsScreen()

beforeEach(() => server.use(http.get('/api/content', () => HttpResponse.json({ items: [], total: 0 }))))

/**
 * Returns the band of notices holding the link of the given name.
 * @param name - The name of a link the band holds.
 * @returns The band element.
 */
async function bandHolding(name: string): Promise<HTMLElement> {
	const link = await screen.findByRole('link', { name })
	const band = link.closest<HTMLElement>('.godmin-layout__notices')
	expect(band).not.toBeNull()
	return band as HTMLElement
}

/**
 * Returns the band of notices when it holds a notice.
 * @returns The band element, or null while it stands empty or absent.
 */
function filledBand(): Element | null {
	return document.querySelector('.godmin-layout__notices:not(:empty)')
}

test('warns an update_core holder above the screen that the patch release is available', async () => {
	serveUpdates(patchWaiting)
	renderAt('/content/post')

	const band = await bandHolding('Please update now')
	const heading = await screen.findByRole('heading', { level: 2, name: 'Posts' })

	expect(band).toHaveTextContent('Gophenberg 0.20.1 is available! Please update now.')
	expect(within(band).getByRole('link', { name: 'Gophenberg 0.20.1' })).toHaveAttribute(
		'href',
		'https://example.com/releases/0.20.1',
	)
	expect(screen.getByRole('main')).toContainElement(band)
	expect(band.compareDocumentPosition(heading) & Node.DOCUMENT_POSITION_FOLLOWING).toBeTruthy()
})

test('names the update link by the words it shows, so a spoken command naming them reaches it', async () => {
	serveUpdates(patchWaiting)
	renderAt('/content/post')

	const band = await bandHolding('Please update now')

	expect(within(band).getByRole('link', { name: 'Please update now' })).toHaveAttribute('href', '/admin/update-core')
})

test('leaves the update nag without a button to dismiss it', async () => {
	serveUpdates(patchWaiting)
	renderAt('/content/post')

	const band = await bandHolding('Please update now')

	expect(within(band).queryByRole('button')).toBeNull()
})

test('warns on the settings screen as on every other admin screen', async () => {
	serveUpdates(patchWaiting)
	renderAt('/settings')

	expect(await bandHolding('Please update now')).toHaveTextContent('Gophenberg 0.20.1 is available!')
})

test('keeps the nag off the Updates screen', async () => {
	serveUpdates(patchWaiting)
	const client = renderAt('/update-core')

	expect(await screen.findByRole('heading', { level: 2, name: 'Gophenberg Updates' })).toBeInTheDocument()
	await updatesAnswered(client)

	expect(screen.queryByRole('link', { name: 'Please update now' })).toBeNull()
})

test.each([
	['no release notes', ''],
	['release notes away from the web', 'ftp://example.com/releases/0.20.1'],
])('names the version without a link when the offer carries %s, keeping it on one line', async (_condition, notes) => {
	serveUpdates({ ...patchWaiting, offer: { ...patchOffer, notes } })
	renderAt('/content/post')

	const band = await bandHolding('Please update now')

	expect(band).toHaveTextContent('Gophenberg 0.20.1 is available! Please update now.')
	expect(within(band).queryByRole('link', { name: 'Gophenberg 0.20.1' })).toBeNull()
	expect(within(band).getByText('Gophenberg 0.20.1')).toHaveClass('gophenberg-updates__version')
})

test('names the version the failed update went to and links to try again while the job holds a failure', async () => {
	serveUpdates(patchFailed)
	renderAt('/content/post')

	const band = await bandHolding('Please attempt the update again now')

	expect(band).toHaveTextContent(
		'The update to Gophenberg 0.20.1 failed to complete. Please attempt the update again now.',
	)
	expect(within(band).getByRole('link', { name: 'Please attempt the update again now' })).toHaveAttribute(
		'href',
		'/admin/update-core',
	)
	expect(within(band).queryByRole('button')).toBeNull()
})

test('shows only the failure nag while the failure names the version on offer', async () => {
	serveUpdates(patchFailed)
	renderAt('/content/post')

	const band = await bandHolding('Please attempt the update again now')

	expect(band).toHaveTextContent(
		/^The update to Gophenberg 0\.20\.1 failed to complete\. Please attempt the update again now\.$/,
	)
	expect(within(band).queryByRole('link', { name: 'Please update now' })).toBeNull()
})

test('warns of a newer update before the failure of the last one, as WordPress orders its nags', async () => {
	serveUpdates({ ...patchFailed, offer: { ...patchOffer, version: '0.20.2', notes: '' } })
	renderAt('/content/post')

	const band = await bandHolding('Please attempt the update again now')
	const retry = within(band).getByRole('link', { name: 'Please attempt the update again now' })
	const update = within(band).getByRole('link', { name: 'Please update now' })

	expect(band).toHaveTextContent('Gophenberg 0.20.2 is available!')
	expect(band).toHaveTextContent('The update to Gophenberg 0.20.1 failed to complete.')
	expect(update.compareDocumentPosition(retry) & Node.DOCUMENT_POSITION_FOLLOWING).toBeTruthy()
})

test('says the update failed once the failed version is no longer on offer', async () => {
	serveUpdates({ ...patchFailed, offer: null })
	renderAt('/content/post')

	expect(await bandHolding('Please attempt the update again now')).toHaveTextContent(
		'The update to Gophenberg 0.20.1 failed to complete.',
	)
})

test('says nothing while no update waits', async () => {
	const client = renderAt('/content/post')

	expect(await screen.findByRole('heading', { level: 2, name: 'Posts' })).toBeInTheDocument()
	await updatesAnswered(client)

	expect(filledBand()).toBeNull()
})

test('says nothing while the offer names a version this site refused', async () => {
	serveUpdates({ ...patchWaiting, refused: [patchOffer.version] })
	const client = renderAt('/content/post')

	expect(await screen.findByRole('heading', { level: 2, name: 'Posts' })).toBeInTheDocument()
	await updatesAnswered(client)

	expect(filledBand()).toBeNull()
})

test('says nothing while the overview of the updates cannot be read', async () => {
	server.use(http.get('/api/updates', () => HttpResponse.json({ error: 'boom' }, { status: 500 })))
	const client = renderAt('/content/post')

	expect(await screen.findByRole('heading', { level: 2, name: 'Posts' })).toBeInTheDocument()
	await updatesAnswered(client, 'error')

	expect(filledBand()).toBeNull()
	expect(screen.getByRole('link', { name: 'Updates' })).toBeInTheDocument()
})

test('says nothing about updates to a reader without update_core', async () => {
	let asked = false
	server.use(
		http.get('/api/updates', () => {
			asked = true
			return HttpResponse.json(patchFailed)
		}),
	)
	renderAt('/content/post', { ...defaultUser, role: 'editor', capabilities: [CHANGE_OTHERS_WORK] })

	expect(await screen.findByRole('heading', { level: 2, name: 'Posts' })).toBeInTheDocument()

	expect(filledBand()).toBeNull()
	expect(asked).toBe(false)
})

test('stops warning a reader whose role loses update_core while the admin stays open', async () => {
	serveUpdates(patchWaiting)
	const client = renderAt('/content/post')

	await bandHolding('Please update now')
	act(() => seedSession(client, { ...adminUser, role: 'editor', capabilities: [ACCESS_ADMIN, CHANGE_OTHERS_WORK] }))

	await waitFor(() => expect(filledBand()).toBeNull())
})

test('warns a Spanish reader in the words of WordPress es_ES', async () => {
	setLocaleData(await catalogFor('es-ES'), 'gophenberg')
	onTestFinished(() => resetLocaleData({}, 'gophenberg'))
	serveUpdates(patchWaiting)
	renderAt('/content/post')

	const band = await bandHolding('Por favor, actualiza ahora')

	expect(band).toHaveTextContent('¡Ya está disponible Gophenberg 0.20.1! Por favor, actualiza ahora.')
})

test('tells a Spanish reader the update failed, asking for the retry in the words of WordPress es_ES', async () => {
	setLocaleData(await catalogFor('es-ES'), 'gophenberg')
	onTestFinished(() => resetLocaleData({}, 'gophenberg'))
	serveUpdates(patchFailed)
	renderAt('/content/post')

	const band = await bandHolding('vuelve a intentarlo')

	expect(band).toHaveTextContent(
		'No se ha podido completar la actualización a Gophenberg 0.20.1. Por favor, vuelve a intentarlo.',
	)
})
