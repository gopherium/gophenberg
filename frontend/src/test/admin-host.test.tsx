// SPDX-License-Identifier: Apache-2.0

import { http, HttpResponse, server } from '@gophenberg/frontend-sdk/testing'
import { ConfirmDialog } from '@gopherium/godmin'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import type { ReactNode } from 'react'
import { expect, test } from 'vitest'

import { AdminHost } from '../AdminHost'
import { SiteFormatGate } from '../settings/SiteFormatGate'
import { siteSettings } from './siteSettings'

/** WordPress's words for a write the server never answered. */
const NO_ANSWER = 'Could not get a valid response from the server.'

/** A time limit far shorter than any test. */
const SHORT_LIMIT = 20

/**
 * Returns a write the server never answers.
 * @returns A promise that never settles.
 */
function neverAnswered(): Promise<never> {
	return new Promise(() => {})
}

/**
 * Renders the admin host around the given tree, with a fresh query client.
 * @param children - The tree to host.
 */
function renderHosted(children: ReactNode) {
	render(
		<QueryClientProvider client={new QueryClient()}>
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
	expect(screen.getByRole('button', { name: 'Cancel' })).toBeEnabled()
})

test('waits on a dialog write as long as it takes while the site settings are not loaded', async () => {
	renderHosted(<HangingConfirm />)

	await userEvent.click(screen.getByRole('button', { name: 'Delete' }))
	await new Promise((resolve) => setTimeout(resolve, SHORT_LIMIT * 3))

	expect(screen.queryByText(NO_ANSWER)).toBeNull()
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
