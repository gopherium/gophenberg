// SPDX-License-Identifier: Apache-2.0

import { Notice } from '@gophenberg/frontend-sdk'
import { http, HttpResponse, server } from '@gophenberg/frontend-sdk/testing'
import { screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { resetLocaleData, setLocaleData } from '@wordpress/i18n'
import { beforeAll, beforeEach, expect, onTestFinished, test } from 'vitest'

import { catalogFor } from '../i18n/catalog'
import { listenAt } from './announcements'
import { classesOf } from './classes'
import { renderAt } from './render'
import { siteSettings } from './siteSettings'
import { declarations } from './stylesheet'
import {
	backedUpAgain,
	briefUpdateTimings,
	longJumpWaiting,
	patchAt,
	patchFailed,
	patchOffer,
	patchRetryFailed,
	patchRolledBack,
	patchRunning,
	patchStaysFailed,
	patchWaiting,
	patchWentBack,
} from './updatesOverview'
import { warmPostsScreen } from './warm'

/** The page that shows how to update Gophenberg. */
const UPDATING_PAGE = 'https://docs.gophenberg.org/self-hosting/updates-and-backups/'

warmPostsScreen()

beforeAll(async () => {
	await import('../updates/UpdatesScreen')
}, 120000)

beforeEach(() => {
	server.use(
		http.get('/api/settings', () => HttpResponse.json({ ...siteSettings, ...briefUpdateTimings })),
		http.get('/api/content', () => HttpResponse.json({ items: [], total: 0 })),
	)
})

/** What the site answers for its overview: an overview, or no answer at all while it restarts. */
type Answer = object | 'down'

/**
 * Serves the overview the test names, which it changes as the update moves on.
 * @param first - What the site answers first.
 * @returns What switches the answer, settling once the site served the new answer.
 */
function siteAnswering(first: Answer): (next: Answer) => Promise<void> {
	let answer = first
	let served = () => {}
	server.use(
		http.get('/api/updates', () => {
			const held = answer
			served()
			return held === 'down' ? HttpResponse.error() : HttpResponse.json(held)
		}),
	)
	return (next) => {
		answer = next
		return new Promise((resolve) => {
			served = resolve
		})
	}
}

/**
 * Lets the screen read an answer twice, so the first read has reached the screen.
 * @param site - What switches the site's answer.
 * @param answer - The answer to read.
 */
async function readTwice(site: (next: Answer) => Promise<void>, answer: Answer) {
	await site(answer)
	await site(answer)
}

/**
 * Records each start of an update the screen sends, answering it as the server does.
 * @param answer - The overview the start answers with.
 * @returns The bodies sent so far.
 */
function acceptStarts(answer: object): unknown[] {
	const sent: unknown[] = []
	server.use(
		http.post('/api/updates/apply', async ({ request }) => {
			sent.push(await request.json())
			return HttpResponse.json(answer, { status: 202 })
		}),
	)
	return sent
}

/** The words of the button that updates to the patch release on its first try. */
const UPDATE = 'Update to version 0.20.1'

/** The words of the button that tries the patch release again once it failed its start check. */
const REINSTALL = 'Re-install version 0.20.1'

/** The title of the notice saying the patch release failed its start check past the minimum version and stays. */
const STAYS_FAILED = 'The update to 0.20.1 failed its start check, and Gophenberg cannot go back to 0.20.0 on its own.'

/**
 * Opens the confirm of the update on offer.
 * @param button - The words of the button that opens it.
 * @returns The confirm dialog.
 */
async function openConfirm(button = UPDATE): Promise<HTMLElement> {
	await userEvent.click(await screen.findByRole('button', { name: button }))
	return screen.findByRole('alertdialog')
}

/**
 * Accepts the confirm of the update on offer and waits until it closes.
 * @param button - The words of the button that opens it.
 */
async function confirmUpdate(button = UPDATE) {
	const dialog = await openConfirm(button)
	await userEvent.click(within(dialog).getByRole('button', { name: 'Update' }))
	await waitFor(() => expect(screen.queryByRole('alertdialog')).toBeNull())
}

/**
 * Checks that the screen shows neither the warning to back up nor the sentence leading to the button.
 */
function noBackupWarningOrLead() {
	expect(screen.queryByText('Important:')).toBeNull()
	expect(screen.queryByText(/manually:/)).toBeNull()
}

/**
 * Waits until the steps mark the line reading the given words as the one under way.
 * @param words - The words of the line.
 */
async function underWay(words: string) {
	await waitFor(() => expect(screen.getByText(words).closest('li')).toHaveAttribute('aria-current', 'step'))
}

/**
 * Returns the list of the steps the line reading the given words belongs to.
 * @param words - The words of a line of the steps.
 * @returns The list element.
 */
function stepsOf(words: string): HTMLElement | null {
	return screen.getByText(words).closest('ol')
}

/**
 * Opens the Updates screen on a site offering an update and starts it, the start answering with an overview.
 * @param waiting - The overview the site answers before the start.
 * @param answered - The overview the start answers with.
 * @returns What switches the site's answer.
 */
async function startUpdate(waiting: object, answered: object): Promise<(next: Answer) => Promise<void>> {
	const site = siteAnswering(waiting)
	acceptStarts(answered)
	renderAt('/update-core')
	await confirmUpdate()
	return site
}

/**
 * Opens the Updates screen on a site offering the patch release and starts the update to it.
 * @returns What switches the site's answer.
 */
function updateToPatch(): Promise<(next: Answer) => Promise<void>> {
	return startUpdate(patchWaiting, patchAt('download'))
}

test('asks before updating, naming the version and how Gophenberg goes back', async () => {
	siteAnswering(patchWaiting)
	renderAt('/update-core')

	const dialog = await openConfirm()

	expect(dialog).toHaveAccessibleName('Update to Gophenberg 0.20.1?')
	expect(dialog).toHaveAccessibleDescription(
		'A backup of the database is taken first, and Gophenberg goes back to 0.20.0 on its own ' +
			'if the new version does not start.',
	)
})

test('starts nothing when the confirm is cancelled', async () => {
	siteAnswering(patchWaiting)
	const sent = acceptStarts(patchAt('download'))
	renderAt('/update-core')

	const dialog = await openConfirm()
	await userEvent.click(within(dialog).getByRole('button', { name: 'Cancel' }))

	await waitFor(() => expect(screen.queryByRole('alertdialog')).toBeNull())
	expect(sent).toEqual([])
	expect(screen.getByRole('button', { name: 'Update to version 0.20.1' })).toBeInTheDocument()
})

test('says in the confirm of an update past the minimum version that the site stays down when it fails', async () => {
	siteAnswering({ ...patchWaiting, offer: { ...patchOffer, long_jump: true } })
	renderAt('/update-core')

	const dialog = await openConfirm()

	expect(dialog).toHaveAccessibleDescription(
		'A backup of the database is taken first, and the site stays down until you restore it ' +
			'if the new version does not start.',
	)
})

test('sends the version on offer once the confirm is accepted', async () => {
	siteAnswering(patchWaiting)
	const sent = acceptStarts(patchAt('download'))
	renderAt('/update-core')

	await confirmUpdate()

	expect(sent).toEqual([{ version: '0.20.1', retry: false }])
})

test('asks for a retry of a version that failed its start check here', async () => {
	siteAnswering(patchWentBack)
	const sent = acceptStarts(patchWentBack)
	renderAt('/update-core')

	await confirmUpdate(REINSTALL)

	expect(sent).toEqual([{ version: '0.20.1', retry: true }])
})

test('leaves the warning to back up and the sentence leading to the button out while the update runs', async () => {
	await updateToPatch()

	await underWay('Downloading Gophenberg 0.20.1…')

	noBackupWarningOrLead()
})

test('follows the update one step at a time, marking the step under way', async () => {
	const site = await updateToPatch()

	await underWay('Downloading Gophenberg 0.20.1…')
	expect(screen.queryByRole('button', { name: 'Update to version 0.20.1' })).toBeNull()
	site(patchAt('stage'))
	await underWay('Unpacking the update…')
	expect(screen.getByText('Downloading Gophenberg 0.20.1…').closest('li')).not.toHaveAttribute('aria-current')
	site(patchAt('backup'))
	await underWay('Backing up the database…')
	expect(screen.getByText('Verifying the unpacked files…').closest('li')).not.toHaveAttribute('aria-current')
	site(patchAt('switched'))
	await underWay('Restarting Gophenberg…')
	site('down')
	await underWay('Restarting Gophenberg…')
	site(patchRunning('switched'))
	await underWay('Checking the new version…')
})

test('keeps the steps in one live region as the offer goes away at the check of the new version', async () => {
	const site = await updateToPatch()

	site(patchAt('switched'))
	await underWay('Restarting Gophenberg…')
	const steps = stepsOf('Restarting Gophenberg…')
	site(patchRunning('switched'))
	await underWay('Checking the new version…')

	expect(stepsOf('Checking the new version…')).toBe(steps)
})

test('moves the focus to the steps once the confirm starts the update', async () => {
	await updateToPatch()

	await underWay('Downloading Gophenberg 0.20.1…')

	await waitFor(() => expect(document.activeElement).toBe(stepsOf('Downloading Gophenberg 0.20.1…')))
})

test('draws no focus ring around the steps, which only the screen moves the focus to', () => {
	expect(declarations('.gophenberg-updates__steps:focus')).toEqual({ outline: 'none' })
})

test('marks the step under way by its colour and its spinner alone', () => {
	expect(Object.keys(declarations(".gophenberg-updates__steps > li[aria-current='step']"))).toEqual(['color'])
})

test('says the update succeeded once the new version runs and offers to reload the admin', async () => {
	const polite = listenAt('polite')
	const site = await updateToPatch()

	site('down')
	await underWay('Restarting Gophenberg…')
	site(patchRunning('committed'))

	expect(await screen.findByText('Gophenberg updated successfully.')).toBeInTheDocument()
	const reload = screen.getByRole('link', { name: 'Reload the admin' })
	expect(reload).toHaveAttribute('href', '/admin/update-core')
	expect(reload.className).toBe(classesOf(<Notice.ActionLink href="/">Reload</Notice.ActionLink>))
	expect(screen.getByText('Current version: 0.20.1')).toBeInTheDocument()
	expect(within(screen.getByRole('main')).queryByRole('list')).toBeNull()
	expect(polite()).toContain('Gophenberg updated successfully.')
})

test('says the update failed and Gophenberg went back, until the notice is dismissed', async () => {
	const assertive = listenAt('assertive')
	const site = await updateToPatch()

	site(patchAt('switched'))
	await underWay('Restarting Gophenberg…')
	site(patchWentBack)

	const title = await screen.findByText('The update to 0.20.1 failed.')
	const description = screen.getByText('Gophenberg went back to 0.20.0 and is running normally.')
	expect(title.compareDocumentPosition(description) & Node.DOCUMENT_POSITION_FOLLOWING).toBeTruthy()
	expect(assertive()).toContain(
		'The update to 0.20.1 failed. Gophenberg went back to 0.20.0 and is running normally.',
	)
	await userEvent.click(screen.getByRole('button', { name: 'Dismiss' }))
	await waitFor(() => expect(screen.queryByText(/Gophenberg went back to/)).toBeNull())
	expect(screen.getByRole('button', { name: REINSTALL })).toBeInTheDocument()
})

test('offers the retry under the failed start check once the update went back, with no offer above it', async () => {
	const site = await updateToPatch()

	site(patchAt('switched'))
	await underWay('Restarting Gophenberg…')
	site(patchWentBack)

	await screen.findByText('The update to 0.20.1 failed.')
	const failed = screen.getByText(
		'Gophenberg 0.20.1 failed its start check on this site. Fix what failed before you try it again.',
	)
	expect(failed.nextElementSibling).toBe(screen.getByRole('button', { name: REINSTALL }))
	expect(screen.queryByRole('button', { name: UPDATE })).toBeNull()
	noBackupWarningOrLead()
})

test.each([
	['the start check', patchWentBack],
	['the rollback unit', patchRolledBack],
])('says Gophenberg went back once %s took back a first try no read caught under way', async (_by, wentBack) => {
	const site = await startUpdate(patchWaiting, patchWaiting)
	await underWay('Downloading Gophenberg 0.20.1…')

	site(wentBack)

	expect(await screen.findByText(/Gophenberg went back to 0.20.0/)).toBeInTheDocument()
})

test('keeps following a retry of a version the rollback unit took back until it goes back again', async () => {
	const site = siteAnswering(patchRolledBack)
	const sent = acceptStarts(patchRolledBack)
	renderAt('/update-core')

	await confirmUpdate(REINSTALL)
	await readTwice(site, patchRolledBack)

	await underWay('Downloading Gophenberg 0.20.1…')
	expect(screen.queryByText(/Gophenberg went back to/)).toBeNull()
	site(patchAt('switched'))
	await underWay('Restarting Gophenberg…')
	site(patchRolledBack)
	expect(await screen.findByText(/Gophenberg went back to 0.20.0/)).toBeInTheDocument()
	expect(sent).toEqual([{ version: '0.20.1', retry: true }])
})

test('says a retry of a version the rollback unit took back failed before anything changed', async () => {
	const assertive = listenAt('assertive')
	const site = siteAnswering(patchRolledBack)
	acceptStarts(patchRolledBack)
	renderAt('/update-core')

	await confirmUpdate(REINSTALL)
	await readTwice(site, { ...patchAt('download'), refused: ['0.20.1'] })
	await underWay('Downloading Gophenberg 0.20.1…')
	site(patchRetryFailed)

	expect(await screen.findByText('The update failed before anything changed.')).toBeInTheDocument()
	expect(screen.queryByText(/went back to/)).toBeNull()
	expect(screen.getByRole('button', { name: REINSTALL })).toBeInTheDocument()
	expect(assertive()).toEqual(['The update failed before anything changed.'])
})

test('says the update stays failed once a release past the minimum version fails its start check', async () => {
	const assertive = listenAt('assertive')
	const site = await startUpdate(longJumpWaiting, { ...longJumpWaiting, job: patchAt('download').job })

	site('down')
	await underWay('Restarting Gophenberg…')
	site(patchStaysFailed)

	expect(await screen.findByText(STAYS_FAILED)).toBeInTheDocument()
	expect(screen.getByText('update:status', { selector: 'code' }).parentElement).toHaveTextContent(
		/^Run update:status on the server to see the backup taken before the update\./,
	)
	expect(screen.getByRole('link', { name: 'restore that backup' })).toHaveAttribute(
		'href',
		`${UPDATING_PAGE}#restoring-on-a-server-install`,
	)
	expect(screen.queryByText('Checking the new version…')).toBeNull()
	expect(screen.queryByRole('heading', { level: 3 })).toBeNull()
	expect(assertive()).toContain(STAYS_FAILED)
})

test('says an update stays failed, speaking nothing, when the screen opens after its start check failed', async () => {
	const assertive = listenAt('assertive')
	siteAnswering(patchStaysFailed)
	renderAt('/update-core')

	expect(await screen.findByText(STAYS_FAILED)).toBeInTheDocument()
	expect(screen.getByRole('link', { name: 'restore that backup' })).toHaveAttribute(
		'href',
		`${UPDATING_PAGE}#restoring-on-a-server-install`,
	)
	expect(screen.queryByRole('heading', { level: 3 })).toBeNull()
	expect(assertive()).toEqual([])
})

test('says the update failed before anything changed, with the reason the server gave', async () => {
	const assertive = listenAt('assertive')
	const site = await updateToPatch()

	await underWay('Downloading Gophenberg 0.20.1…')
	site(patchFailed)

	expect(await screen.findByText('The update failed before anything changed.')).toBeInTheDocument()
	expect(screen.getByText('the checksum of the archive does not match the index')).toBeInTheDocument()
	expect(screen.getByRole('button', { name: UPDATE })).toBeInTheDocument()
	expect(assertive()).toContain('The update failed before anything changed.')
	noBackupWarningOrLead()
})

test('speaks nothing of an update that failed before anything changed on an earlier visit', async () => {
	const assertive = listenAt('assertive')
	siteAnswering(patchFailed)
	renderAt('/update-core')

	expect(await screen.findByText('The update failed before anything changed.')).toBeInTheDocument()
	expect(assertive()).toEqual([])
})

test('keeps following an update while the overview still holds the failure of an earlier try', async () => {
	const site = siteAnswering(patchFailed)
	acceptStarts(patchFailed)
	renderAt('/update-core')

	await confirmUpdate()
	await readTwice(site, patchFailed)

	await underWay('Downloading Gophenberg 0.20.1…')
	expect(screen.queryByText('The update failed before anything changed.')).toBeNull()
	site(patchAt('stage'))
	await underWay('Unpacking the update…')
})

test('keeps following a retry while the overview still names the version it went back from', async () => {
	const site = siteAnswering(patchWentBack)
	acceptStarts(patchWentBack)
	renderAt('/update-core')

	await confirmUpdate(REINSTALL)
	await readTwice(site, patchWentBack)

	await underWay('Downloading Gophenberg 0.20.1…')
	expect(screen.queryByText(/Gophenberg went back to/)).toBeNull()
	await readTwice(site, { ...patchAt('download'), refused: ['0.20.1'] })
	await underWay('Downloading Gophenberg 0.20.1…')
	expect(screen.queryByText(/Gophenberg went back to/)).toBeNull()
	site(patchAt('switched'))
	await underWay('Restarting Gophenberg…')
	site(patchWentBack)
	expect(await screen.findByText(/Gophenberg went back to 0.20.0/)).toBeInTheDocument()
})

test.each([
	['the start check', patchWentBack],
	['the rollback unit', patchRolledBack],
])('says Gophenberg went back once %s took back a retry no read caught under way', async (_by, wentBack) => {
	const site = siteAnswering(wentBack)
	acceptStarts(wentBack)
	renderAt('/update-core')

	await confirmUpdate(REINSTALL)
	await readTwice(site, wentBack)
	await underWay('Downloading Gophenberg 0.20.1…')
	site(backedUpAgain(wentBack))

	expect(await screen.findByText(/Gophenberg went back to 0.20.0/)).toBeInTheDocument()
	expect(screen.getByRole('button', { name: REINSTALL })).toBeInTheDocument()
})

test('says Gophenberg has not come back once the restart outlasts the wait the site sets', async () => {
	const assertive = listenAt('assertive')
	const site = await updateToPatch()

	site('down')

	expect(await screen.findByText('Gophenberg has not come back yet.')).toBeInTheDocument()
	const advice = screen.getByText('update:status', { selector: 'code' }).parentElement as HTMLElement
	expect(advice).toHaveTextContent('after exit code 75, and run update:status on the server')
	expect(within(advice).getByText('75', { selector: 'code' })).toBeInTheDocument()
	expect(screen.queryByRole('button', { name: UPDATE })).toBeNull()
	expect(assertive()).toContain('Gophenberg has not come back yet.')
	noBackupWarningOrLead()
})

test('says the update succeeded when Gophenberg comes back after the wait passed', async () => {
	const site = await updateToPatch()

	site('down')
	await screen.findByText('Gophenberg has not come back yet.')
	site(patchRunning('committed'))

	expect(await screen.findByText('Gophenberg updated successfully.')).toBeInTheDocument()
	expect(screen.queryByText('Gophenberg has not come back yet.')).toBeNull()
})

test('shows in the confirm why the server refused to start the update', async () => {
	siteAnswering(patchWaiting)
	server.use(
		http.post('/api/updates/apply', () =>
			HttpResponse.json(
				{
					error: 'refused',
					code: 'update_refused',
					meta: { reason: 'Gophenberg 0.20.1 needs PostgreSQL 18 or newer' },
				},
				{ status: 422 },
			),
		),
	)
	renderAt('/update-core')

	const dialog = await openConfirm()
	await userEvent.click(within(dialog).getByRole('button', { name: 'Update' }))

	expect(
		await within(dialog).findByText(
			'Gophenberg cannot start this update: Gophenberg 0.20.1 needs PostgreSQL 18 or newer',
		),
	).toBeInTheDocument()
	expect(screen.getByRole('alertdialog')).toBeInTheDocument()
})

test('follows the update under way once the server refuses a second start', async () => {
	const site = siteAnswering(patchWaiting)
	server.use(
		http.post('/api/updates/apply', () =>
			HttpResponse.json({ error: 'another update runs', code: 'update_runs' }, { status: 409 }),
		),
	)
	renderAt('/update-core')

	const dialog = await openConfirm()
	site(patchAt('stage'))
	await userEvent.click(within(dialog).getByRole('button', { name: 'Update' }))

	await underWay('Unpacking the update…')
	expect(screen.queryByRole('button', { name: 'Update to version 0.20.1' })).toBeNull()
	await waitFor(() => expect(document.activeElement).toBe(stepsOf('Unpacking the update…')))
})

test('follows an update already under way when the screen opens', async () => {
	siteAnswering(patchAt('stage'))
	renderAt('/update-core')

	await underWay('Unpacking the update…')
	expect(screen.queryByRole('button', { name: 'Update to version 0.20.1' })).toBeNull()
})

test('says an update already under way when the screen opens succeeded once the new version runs', async () => {
	const site = siteAnswering(patchAt('backup'))
	renderAt('/update-core')
	await underWay('Backing up the database…')

	site('down')
	await underWay('Restarting Gophenberg…')
	site(patchRunning('committed'))

	expect(await screen.findByText('Gophenberg updated successfully.')).toBeInTheDocument()
	expect(screen.getByRole('link', { name: 'Reload the admin' })).toHaveAttribute('href', '/admin/update-core')
})

test('says an update already under way when the screen opens went back once the version before it runs', async () => {
	const site = siteAnswering(patchAt('backup'))
	renderAt('/update-core')
	await underWay('Backing up the database…')

	site(patchAt('switched'))
	await underWay('Restarting Gophenberg…')
	site(patchWentBack)

	expect(await screen.findByText(/Gophenberg went back to 0.20.0/)).toBeInTheDocument()
})

test.each([
	['succeeded once it passes', patchRunning('committed'), 'Gophenberg updated successfully.'],
	['went back once it fails', patchWentBack, 'Gophenberg went back to 0.20.0 and is running normally.'],
])('says an update the screen opens on while the new version checks itself %s', async (_ending, ended, words) => {
	const site = siteAnswering(patchRunning('switched'))
	renderAt('/update-core')

	await underWay('Checking the new version…')
	expect(screen.queryByRole('heading', { level: 3, name: 'You have the latest version of Gophenberg.' })).toBeNull()
	site(ended)

	expect(await screen.findByText(words)).toBeInTheDocument()
})

test('leaves the focus on the menu when Updates opens on an update under way', async () => {
	siteAnswering(patchAt('stage'))
	renderAt('/content/post')

	const nav = await screen.findByRole('region', { name: 'Navigation' })
	const updates = await within(nav).findByRole('link', { name: 'Updates 1 update available' })
	await userEvent.click(updates)

	await underWay('Unpacking the update…')
	expect(document.activeElement).toBe(updates)
})

test('follows the steps in the words of WordPress es_ES for a Spanish reader', async () => {
	setLocaleData(await catalogFor('es-ES'), 'gophenberg')
	onTestFinished(() => resetLocaleData({}, 'gophenberg'))
	siteAnswering(patchAt('backup'))
	renderAt('/update-core')

	await underWay('Haciendo copia de seguridad de la base de datos…')
	expect(screen.getByText('Descomprimiendo la actualización…')).toBeInTheDocument()
	expect(screen.getByText('Verificando los archivos descomprimidos…')).toBeInTheDocument()
})
