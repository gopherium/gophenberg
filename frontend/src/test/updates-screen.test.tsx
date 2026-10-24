// SPDX-License-Identifier: Apache-2.0

import { Button } from '@gophenberg/frontend-sdk'
import { http, HttpResponse, server } from '@gophenberg/frontend-sdk/testing'
import { screen, within } from '@testing-library/react'
import { resetLocaleData, setLocaleData } from '@wordpress/i18n'
import { beforeAll, expect, onTestFinished, test } from 'vitest'

import { catalogFor } from '../i18n/catalog'
import { classesOf } from './classes'
import { renderAt } from './render'
import { declarations } from './stylesheet'
import { patchFailed, patchOffer, patchWaiting, serveUpdates, upToDate } from './updatesOverview'

/** The page that shows how to update Gophenberg. */
const UPDATING_PAGE = 'https://docs.gophenberg.org/self-hosting/updates-and-backups/'

/** How many milliseconds a minute lasts. */
const MINUTE = 60_000

beforeAll(async () => {
	await import('../updates/UpdatesScreen')
}, 120000)

/**
 * Returns the paragraph of the screen that reads exactly the given words.
 * @param words - The whole text of the paragraph.
 * @returns The paragraph element.
 */
function paragraph(words: string): Promise<HTMLElement> {
	return screen.findByText((_content, element) => element?.tagName === 'P' && element.textContent === words)
}

/**
 * Opens the Updates screen on a site answering the given overview.
 * @param overview - What GET /api/updates answers.
 */
async function openUpdates(overview: object) {
	serveUpdates(overview)
	renderAt('/update-core')
	await screen.findByRole('heading', { level: 2, name: 'Gophenberg Updates' })
}

/**
 * Returns the overview of a site whose next check runs a while from now.
 * @param away - How many milliseconds from now the next check runs.
 * @returns The overview.
 */
function nextCheckIn(away: number) {
	return { ...upToDate, next_check: new Date(Date.now() + away).toISOString() }
}

test('names the version that runs under the title, as WordPress does', async () => {
	await openUpdates(upToDate)

	expect(await screen.findByText('Current version: 0.20.0')).toHaveClass('godmin-page__subtitle')
})

test('says when the site last checked, in the site format, with the check one click away', async () => {
	await openUpdates(upToDate)

	const line = await paragraph('Last checked on 22/10/2026 at 09:12. Check again.')

	expect(within(line).getByRole('button', { name: 'Check again.' })).toBeInTheDocument()
})

test.each([
	['minutes away', 30 * MINUTE, 'The next check runs in 30 minutes.'],
	['under a minute away', MINUTE / 3, 'The next check runs in 1 minute.'],
	['hours away', 11 * 60 * MINUTE, 'The next check runs in 11 hours.'],
	['days away', 3 * 24 * 60 * MINUTE, 'The next check runs in 3 days.'],
	['already due', -5 * MINUTE, 'The next check runs in 1 minute.'],
])('says when the next check runs while it is %s', async (_condition, away, words) => {
	await openUpdates(nextCheckIn(away))

	expect(await paragraph(words)).toBeInTheDocument()
})

test('says the site has not checked yet and offers the first check', async () => {
	await openUpdates({ ...upToDate, checked_at: null })

	const line = await paragraph('Gophenberg has not checked for updates yet. Check now.')

	expect(within(line).getByRole('button', { name: 'Check now.' })).toBeInTheDocument()
})

test('wraps the line holding the check greedily, so the control pulls no words down with it', async () => {
	await openUpdates({ ...upToDate, checked_at: null })

	const line = await paragraph('Gophenberg has not checked for updates yet. Check now.')

	expect(line).toHaveClass('gophenberg-updates__last-check')
	expect(declarations('.gophenberg-updates__last-check')).toEqual({ 'text-wrap': 'wrap' })
})

test('says the site does not check on its own while its checks are off', async () => {
	await openUpdates({ ...upToDate, next_check: null })

	expect(await paragraph('This site does not check for updates on its own.')).toBeInTheDocument()
})

test('names when the last check failed and why, in the words the server gave', async () => {
	await openUpdates({ ...upToDate, failed_at: '2026-10-22T10:00:00Z', error: 'the update site did not answer' })

	expect(await paragraph('The check on 22/10/2026 at 10:00 failed: the update site did not answer')).toBeInTheDocument()
})

test('says the site receives no automatic updates, in the words WordPress uses while they are off', async () => {
	await openUpdates(upToDate)

	expect(
		await paragraph('This site will not receive automatic updates for new versions of Gophenberg.'),
	).toBeInTheDocument()
})

test('says the site runs the latest version and offers no reinstall', async () => {
	await openUpdates(upToDate)

	expect(
		await screen.findByRole('heading', { level: 3, name: 'You have the latest version of Gophenberg.' }),
	).toBeInTheDocument()
	expect(screen.queryByRole('button', { name: /version/i })).toBeNull()
})

test.each([
	['no check ran yet', { ...upToDate, checked_at: null }],
	['the only check failed', { ...upToDate, checked_at: null, failed_at: '2026-10-22T10:00:00Z', error: 'timeout' }],
])('claims no latest version while %s', async (_condition, overview) => {
	await openUpdates(overview)

	await paragraph('Gophenberg has not checked for updates yet. Check now.')

	expect(screen.queryByRole('heading', { level: 3 })).toBeNull()
})

test('offers the patch release with its release notes and the button that updates', async () => {
	await openUpdates(patchWaiting)

	expect(
		await screen.findByRole('heading', { level: 3, name: 'An updated version of Gophenberg is available.' }),
	).toBeInTheDocument()
	const offer = await paragraph('You can update from Gophenberg 0.20.0 to Gophenberg 0.20.1 manually:')
	expect(within(offer).getByRole('link', { name: 'Gophenberg 0.20.1' })).toHaveAttribute(
		'href',
		'https://example.com/releases/0.20.1',
	)
	expect(offer.nextElementSibling).toBe(screen.getByRole('button', { name: 'Update to version 0.20.1' }))
	expect(
		await paragraph('While your site is being updated, it keeps serving until Gophenberg restarts on the new version.'),
	).toBeInTheDocument()
})

test('keeps the release notes link on one line, so the version never parts from its name', async () => {
	await openUpdates(patchWaiting)

	const offer = await paragraph('You can update from Gophenberg 0.20.0 to Gophenberg 0.20.1 manually:')

	expect(within(offer).getByRole('link', { name: 'Gophenberg 0.20.1' })).toHaveClass('gophenberg-updates__version')
	expect(declarations('.gophenberg-updates__version')).toEqual({ 'white-space': 'nowrap' })
})

test('warns to back up before updating, linking the backup advice and the page on updating', async () => {
	await openUpdates(patchWaiting)

	const warning = (await screen.findByText('Important:')).parentElement as HTMLElement

	expect(warning).toHaveTextContent(
		'Important: Before updating, please back up your database and files. ' +
			'For help with updates, visit the Updating Gophenberg documentation page.',
	)
	expect(within(warning).getByRole('link', { name: 'back up your database and files' })).toHaveAttribute(
		'href',
		`${UPDATING_PAGE}#what-to-back-up`,
	)
	expect(within(warning).getByRole('link', { name: 'Updating Gophenberg' })).toHaveAttribute('href', UPDATING_PAGE)
})

test('refuses an update the PostgreSQL server is too old for and offers no button', async () => {
	await openUpdates({
		...patchWaiting,
		offer: { ...patchOffer, version: '0.21.0', postgresql: 18, needs_postgresql: true },
	})

	const refused = await paragraph(
		'You cannot update because Gophenberg 0.21.0 requires PostgreSQL version 18 or higher.',
	)

	expect(within(refused).getByRole('link', { name: 'Gophenberg 0.21.0' })).toBeInTheDocument()
	expect(screen.queryByRole('button', { name: /Update to version/ })).toBeNull()
	expect(screen.queryByText(/keeps serving until/)).toBeNull()
})

test('says an update past the minimum version needs the backup above the sentence leading to the button', async () => {
	await openUpdates({ ...patchWaiting, offer: { ...patchOffer, long_jump: true } })

	const warning = await paragraph(
		'This update needs a backup of the database, and the site stays down if the new version does not start.',
	)
	const offer = await paragraph('You can update from Gophenberg 0.20.0 to Gophenberg 0.20.1 manually:')

	expect(warning.compareDocumentPosition(offer) & Node.DOCUMENT_POSITION_FOLLOWING).toBeTruthy()
	expect(offer.nextElementSibling).toBe(screen.getByRole('button', { name: 'Update to version 0.20.1' }))
})

test('offers a version that failed its start check again as a reinstall under the sentence saying so', async () => {
	await openUpdates({ ...patchWaiting, refused: ['0.20.1'] })

	const failed = await paragraph(
		'Gophenberg 0.20.1 failed its start check on this site. Fix what failed before you try it again.',
	)
	const reinstall = screen.getByRole('button', { name: 'Re-install version 0.20.1' })

	expect(failed.nextElementSibling).toBe(reinstall)
	expect(reinstall.className).toBe(classesOf(<Button variant="outline">Outline</Button>))
	expect(screen.queryByRole('button', { name: 'Update to version 0.20.1' })).toBeNull()
	expect(screen.queryByText(/manually:/)).toBeNull()
})

test('claims no update is available while the only version on offer is one this site refuses', async () => {
	await openUpdates({ ...patchWaiting, refused: ['0.20.1'] })

	await paragraph('Gophenberg 0.20.1 failed its start check on this site. Fix what failed before you try it again.')

	expect(screen.queryByRole('heading', { level: 3 })).toBeNull()
})

test.each([
	['no install root', 'no_install_root', 'This install of Gophenberg has no install root, so it updates by hand.'],
	['a custom build', 'custom_build', 'This is a custom build of Gophenberg, so it updates by hand.'],
	['a reason a newer server names', 'a_reason_from_the_future', 'This install of Gophenberg updates by hand.'],
])('shows the steps by hand in place of the button for an install with %s', async (_condition, reason, words) => {
	await openUpdates({ ...patchWaiting, can_update: false, cannot_update: reason })

	expect(await paragraph(words)).toBeInTheDocument()
	const steps = await paragraph(
		'Follow the steps on the Updating Gophenberg documentation page to update by hand.',
	)
	expect(within(steps).getByRole('link', { name: 'Updating Gophenberg' })).toHaveAttribute('href', UPDATING_PAGE)
	expect(screen.queryByRole('button', { name: /Update to version/ })).toBeNull()
	expect(screen.queryByText(/manually:/)).toBeNull()
})

test('counts the backups kept, naming when the newest was taken', async () => {
	await openUpdates(upToDate)

	expect(await paragraph('Backups kept: 2 of 3, the newest on 21/10/2026 at 14:02.')).toBeInTheDocument()
})

test('names when the one backup kept was taken', async () => {
	await openUpdates({ ...upToDate, backups: { kept: 1, limit: 3, newest: '2026-10-21T14:02:00Z' } })

	expect(await paragraph('Backups kept: 1 of 3, taken on 21/10/2026 at 14:02.')).toBeInTheDocument()
})

test('says nothing of backups while the install keeps none', async () => {
	await openUpdates({ ...upToDate, backups: { kept: 0, limit: 3, newest: null } })

	await screen.findByRole('heading', { level: 3, name: 'You have the latest version of Gophenberg.' })

	expect(screen.queryByText(/Backups kept/)).toBeNull()
})

test('shows the last update that failed before anything changed, with the reason the server gave', async () => {
	await openUpdates(patchFailed)

	expect(await screen.findByText('The update failed before anything changed.')).toBeInTheDocument()
	expect(screen.getByText('the checksum of the archive does not match the index')).toBeInTheDocument()
})

test('says the updates could not be read when the server answers none', async () => {
	server.use(http.get('/api/updates', () => HttpResponse.json({ error: 'boom' }, { status: 500 })))
	renderAt('/update-core')

	expect(await screen.findByRole('alert')).toHaveTextContent(
		'The updates could not be read, so nothing can be updated here yet.',
	)
})

test('says the updates could not be read and offers no button when the site settings cannot be read', async () => {
	server.use(http.get('/api/settings', () => HttpResponse.json({ error: 'boom' }, { status: 500 })))
	serveUpdates(patchWaiting)
	renderAt('/update-core')

	expect(
		await screen.findByText('The updates could not be read, so nothing can be updated here yet.'),
	).toBeInTheDocument()
	expect(screen.queryByRole('button', { name: 'Update to version 0.20.1' })).toBeNull()
})

test('shows a Spanish reader the Updates screen in the words of WordPress es_ES', async () => {
	setLocaleData(await catalogFor('es-ES'), 'gophenberg')
	onTestFinished(() => resetLocaleData({}, 'gophenberg'))
	serveUpdates(patchWaiting)
	renderAt('/update-core')

	expect(await screen.findByRole('heading', { level: 2, name: 'Actualizaciones de Gophenberg' })).toBeInTheDocument()
	expect(await screen.findByText('Versión actual: 0.20.0')).toBeInTheDocument()
	expect(await paragraph('Última comprobación el 22/10/2026 a las 09:12. Comprobar de nuevo.')).toBeInTheDocument()
	expect(
		await paragraph('Este sitio no recibirá actualizaciones automáticas de nuevas versiones de Gophenberg.'),
	).toBeInTheDocument()
	expect(
		screen.getByRole('heading', { level: 3, name: 'Hay disponible una versión actualizada de Gophenberg.' }),
	).toBeInTheDocument()
	expect(
		await paragraph('Puedes actualizar desde Gophenberg 0.20.0 a Gophenberg 0.20.1 manualmente:'),
	).toBeInTheDocument()
	expect(screen.getByRole('button', { name: 'Actualizar a la versión 0.20.1' })).toBeInTheDocument()
})

test('offers a Spanish reader the retry of a refused version in the words of WordPress es_ES', async () => {
	setLocaleData(await catalogFor('es-ES'), 'gophenberg')
	onTestFinished(() => resetLocaleData({}, 'gophenberg'))
	serveUpdates({ ...patchWaiting, refused: ['0.20.1'] })
	renderAt('/update-core')

	expect(await screen.findByRole('button', { name: 'Reinstalar la versión 0.20.1' })).toBeInTheDocument()
})
