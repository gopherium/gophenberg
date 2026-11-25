// SPDX-License-Identifier: Apache-2.0

import { http, HttpResponse, server } from '@gophenberg/frontend-sdk/testing'
import { ConfirmDialog } from '@gopherium/godmin'
import { AuthGate, createAuthQueryClient } from '@gopherium/react-auth'
import { defaultUser, seedSession } from '@gopherium/react-auth/testing'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import type { ReactNode } from 'react'
import { afterEach, expect, test, vi } from 'vitest'

import { AdminHost } from '../AdminHost'
import { SiteFormatGate } from '../settings/SiteFormatGate'
import { siteSettings } from './siteSettings'

/** WordPress's words for a write the server never answered. */
const NO_ANSWER = 'Could not get a valid response from the server.'

/** A time limit far shorter than any test. */
const SHORT_LIMIT = 20

/** The longest time a browser timer holds. */
const LONGEST_TIMER = 2 ** 31 - 1

afterEach(() => {
	vi.useRealTimers()
})

/**
 * Returns a write the server never answers.
 * @returns A promise that never settles.
 */
function neverAnswered(): Promise<never> {
	return new Promise(() => {})
}

/**
 * Renders the admin host around the given tree, with a fresh query client unless one is given.
 * @param children - The tree to host.
 * @param client - The query client the tree reads through.
 */
function renderHosted(children: ReactNode, client = new QueryClient()) {
	render(
		<QueryClientProvider client={client}>
			<AdminHost>{children}</AdminHost>
		</QueryClientProvider>,
	)
}

/**
 * Renders a confirm whose write never ends.
 * @returns The confirm element.
 */
function HangingConfirm() {
	return (
		<ConfirmDialog
			title="Delete the draft?"
			confirmLabel="Delete"
			cancelLabel="Cancel"
			onConfirm={neverAnswered}
			onClose={() => {}}
		/>
	)
}

test('gives up on a dialog write once the time the site serves runs out', async () => {
	server.use(
		http.get('/api/settings', () =>
			HttpResponse.json({ ...siteSettings, dialog_write_timeout_milliseconds: SHORT_LIMIT }),
		),
	)
	renderHosted(
		<SiteFormatGate loading={null}>
			<HangingConfirm />
		</SiteFormatGate>,
	)

	await userEvent.click(await screen.findByRole('button', { name: 'Delete' }))

	expect(await screen.findByText(NO_ANSWER)).toBeVisible()
	expect(screen.getByRole('button', { name: 'Cancel' })).not.toHaveAttribute('aria-disabled', 'true')
})

test('gives up on a dialog write once the time the site serves runs out after a sign-in', async () => {
	server.use(
		http.get('/api/settings', () =>
			HttpResponse.json({ ...siteSettings, dialog_write_timeout_milliseconds: SHORT_LIMIT }),
		),
	)
	const client = createAuthQueryClient({ queries: { retry: false, staleTime: Infinity } })
	seedSession(client, null)
	renderHosted(
		<AuthGate
			loginScreen={(onLogin) => (
				<button type="button" onClick={() => void onLogin(defaultUser)}>
					Log In
				</button>
			)}
		>
			<SiteFormatGate loading={null}>
				<HangingConfirm />
			</SiteFormatGate>
		</AuthGate>,
		client,
	)

	await userEvent.click(screen.getByRole('button', { name: 'Log In' }))
	await userEvent.click(await screen.findByRole('button', { name: 'Delete' }))

	expect(await screen.findByText(NO_ANSWER)).toBeVisible()
})

test('waits on a dialog write as long as it takes while the site settings are not loaded yet', async () => {
	vi.useFakeTimers({ shouldAdvanceTime: true })
	renderHosted(<HangingConfirm />)

	await userEvent.click(screen.getByRole('button', { name: 'Delete' }))
	await vi.advanceTimersByTimeAsync(LONGEST_TIMER)

	expect(screen.queryByText(NO_ANSWER)).toBeNull()
})

test('opens the screens and gives a dialog write no time limit once the site settings could not be read', async () => {
	vi.useFakeTimers({ shouldAdvanceTime: true })
	server.use(
		http.get('/api/settings', () =>
			HttpResponse.json({ error: 'database busy', code: 'database_busy' }, { status: 503 }),
		),
	)
	renderHosted(
		<SiteFormatGate loading={null}>
			<HangingConfirm />
		</SiteFormatGate>,
	)

	await userEvent.click(await screen.findByRole('button', { name: 'Delete' }))
	await vi.advanceTimersByTimeAsync(LONGEST_TIMER)

	expect(screen.queryByText(NO_ANSWER)).toBeNull()
	expect(screen.getByRole('alertdialog', { name: 'Delete the draft?' })).toBeVisible()
	expect(screen.getByRole('button', { name: 'Cancel' })).toHaveAttribute('aria-disabled', 'true')
})

test('leaves the settings for the screens to load', async () => {
	const asked: string[] = []
	server.use(
		http.get('/api/settings', () => {
			asked.push('settings')
			return HttpResponse.json(siteSettings)
		}),
	)
	renderHosted(<p>Signed out</p>)

	await new Promise((resolve) => setTimeout(resolve, SHORT_LIMIT * 3))

	expect(screen.getByText('Signed out')).toBeVisible()
	expect(asked).toEqual([])
})
