// SPDX-License-Identifier: Apache-2.0

import { http, HttpResponse, server } from '@gophenberg/frontend-sdk/testing'
import { ConfirmDialog } from '@gopherium/godmin'
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

/** The element the server writes the dialog write limit into. */
const DIALOG_WRITE_LIMIT = 'gophenberg-dialog-write-timeout'

afterEach(() => {
	vi.useRealTimers()
	document.querySelectorAll(`meta[name="${DIALOG_WRITE_LIMIT}"]`).forEach((meta) => meta.remove())
})

/**
 * Writes the dialog write limit into the page, as the server does when it serves the admin.
 * @param content - The limit, in whole milliseconds, or a value the page cannot use.
 */
function writeDialogLimit(content: string) {
	const meta = document.createElement('meta')
	meta.name = DIALOG_WRITE_LIMIT
	meta.content = content
	document.head.append(meta)
}

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

test('gives up on a dialog write once the limit written into the page runs out', async () => {
	writeDialogLimit(String(SHORT_LIMIT))
	renderHosted(
		<SiteFormatGate loading={null}>
			<HangingConfirm />
		</SiteFormatGate>,
	)

	await userEvent.click(await screen.findByRole('button', { name: 'Delete' }))

	expect(await screen.findByText(NO_ANSWER)).toBeVisible()
	expect(screen.getByRole('button', { name: 'Cancel' })).not.toHaveAttribute('aria-disabled', 'true')
})

test('stops a dialog write at the page limit before the settings have loaded', async () => {
	writeDialogLimit(String(SHORT_LIMIT))
	renderHosted(<HangingConfirm />)

	await userEvent.click(screen.getByRole('button', { name: 'Delete' }))

	expect(await screen.findByText(NO_ANSWER)).toBeVisible()
})

test('opens the screens and stops a dialog write at the page limit when the settings cannot be read', async () => {
	writeDialogLimit(String(SHORT_LIMIT))
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

	expect(await screen.findByText(NO_ANSWER)).toBeVisible()
	expect(screen.getByRole('alertdialog', { name: 'Delete the draft?' })).toBeVisible()
	expect(screen.getByRole('button', { name: 'Cancel' })).not.toHaveAttribute('aria-disabled', 'true')
})

test('uses the settings limit for a dialog write when the page carries none', async () => {
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
})

test.each(['0', 'soon'])(
	'ignores a page limit of %j and waits on a dialog write while the settings are not loaded',
	async (content) => {
		vi.useFakeTimers({ shouldAdvanceTime: true })
		writeDialogLimit(content)
		renderHosted(<HangingConfirm />)

		await userEvent.click(screen.getByRole('button', { name: 'Delete' }))
		await vi.advanceTimersByTimeAsync(LONGEST_TIMER)

		expect(screen.queryByText(NO_ANSWER)).toBeNull()
	},
)

test('waits on a dialog write as long as it takes while the page and the settings carry no limit', async () => {
	vi.useFakeTimers({ shouldAdvanceTime: true })
	renderHosted(<HangingConfirm />)

	await userEvent.click(screen.getByRole('button', { name: 'Delete' }))
	await vi.advanceTimersByTimeAsync(LONGEST_TIMER)

	expect(screen.queryByText(NO_ANSWER)).toBeNull()
})

test('opens the screens with no dialog limit when the page and the settings carry none', async () => {
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
