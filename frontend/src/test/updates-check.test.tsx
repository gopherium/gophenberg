// SPDX-License-Identifier: Apache-2.0

import { http, HttpResponse, server } from '@gophenberg/frontend-sdk/testing'
import { screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { beforeAll, expect, test } from 'vitest'

import { renderAt } from './render'
import { checkFinding, patchWaiting, upToDate } from './updatesOverview'

beforeAll(async () => {
	await import('../updates/UpdatesScreen')
}, 120000)

/**
 * Serves a site whose overview turns into the one a check leaves, and counts the checks it answers.
 * @param before - The overview the site answers before a check.
 * @param after - The overview the site answers once a check ran.
 * @param found - What the check answers.
 * @returns The count of checks so far.
 */
function siteChecking(before: object, after: object, found: object): () => number {
	let overview = before
	let checks = 0
	server.use(
		http.get('/api/updates', () => HttpResponse.json(overview)),
		http.post('/api/updates/check', () => {
			overview = after
			checks += 1
			return HttpResponse.json(found)
		}),
	)
	return () => checks
}

/**
 * Presses the control that checks for updates once the screen shows it.
 * @param name - The words the control shows.
 */
async function checkAgain(name = 'Check again.') {
	await userEvent.click(await screen.findByRole('button', { name }))
}

test('checks again and shows the update the check found in place', async () => {
	const checks = siteChecking(upToDate, patchWaiting, checkFinding(patchWaiting))
	renderAt('/update-core')

	await checkAgain()

	expect(
		await screen.findByRole('heading', { level: 3, name: 'An updated version of Gophenberg is available.' }),
	).toBeInTheDocument()
	expect(checks()).toBe(1)
	expect(screen.queryByText('No new updates found.')).toBeNull()
})

test('says no new update was found when the check finds nothing', async () => {
	siteChecking(upToDate, upToDate, checkFinding(upToDate))
	renderAt('/update-core')

	await checkAgain()

	expect(await screen.findByText('No new updates found.', { selector: '.godmin-toast' })).toBeInTheDocument()
})

test('says no new update was found when the check finds the update already on offer', async () => {
	siteChecking(patchWaiting, patchWaiting, checkFinding(patchWaiting))
	renderAt('/update-core')

	await checkAgain()

	expect(await screen.findByText('No new updates found.', { selector: '.godmin-toast' })).toBeInTheDocument()
})

test('runs the first check of a site that never checked', async () => {
	const checks = siteChecking({ ...upToDate, checked_at: null }, upToDate, checkFinding(upToDate))
	renderAt('/update-core')

	await checkAgain('Check now.')

	await waitFor(() => expect(checks()).toBe(1))
	expect(await screen.findByText(/Last checked on/)).toBeInTheDocument()
})

test('asks a reader to wait after a check that ran moments ago', async () => {
	server.use(
		http.post('/api/updates/check', () =>
			HttpResponse.json(
				{
					error: 'the last check is too recent, so try again later',
					code: 'check_too_soon',
					meta: { retry_after: 42 },
				},
				{ status: 429 },
			),
		),
	)
	renderAt('/update-core')

	await checkAgain()

	expect(await screen.findByRole('alert')).toHaveTextContent(
		'Gophenberg checked for updates moments ago. Try again later.',
	)
})

test('says something went wrong when the server refuses a check without saying why', async () => {
	server.use(http.post('/api/updates/check', () => new HttpResponse('Bad Gateway', { status: 502 })))
	renderAt('/update-core')

	await checkAgain()

	expect(await screen.findByRole('alert')).toHaveTextContent('Something went wrong. Try again.')
})

test('shows a check that failed in place and toasts nothing', async () => {
	const failed = { failed_at: '2026-10-22T09:30:00Z', error: 'the update site did not answer' }
	siteChecking(upToDate, { ...upToDate, ...failed }, { ...checkFinding(upToDate), ...failed })
	renderAt('/update-core')

	await checkAgain()

	expect(
		await screen.findByText('The check on 22/10/2026 at 09:30 failed: the update site did not answer'),
	).toBeInTheDocument()
	expect(screen.queryByText('No new updates found.')).toBeNull()
})
