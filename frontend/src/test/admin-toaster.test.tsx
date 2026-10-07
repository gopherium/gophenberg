// SPDX-License-Identifier: Apache-2.0

import { http, HttpResponse, server } from '@gophenberg/frontend-sdk/testing'
import { useToaster } from '@gopherium/godmin'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { render, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { expect, test, vi } from 'vitest'

import { AdminToaster } from '../toasts'
import { siteSettings } from './siteSettings'

const LONG_TITLE = 'A title far longer than the toast name length a site picks'

/**
 * Renders the admin toast region around a control raising one toast that names an item.
 */
function renderToaster() {
	/**
	 * Renders the name a toast would carry and the control raising it.
	 * @returns The name and the control.
	 */
	function Raise() {
		const toaster = useToaster()
		const named = toaster.name(LONG_TITLE)
		return (
			<>
				<p>{named}</p>
				<button onClick={() => toaster.show(named)}>raise</button>
			</>
		)
	}
	render(
		<QueryClientProvider client={new QueryClient()}>
			<AdminToaster>
				<Raise />
			</AdminToaster>
		</QueryClientProvider>,
	)
}

/**
 * Serves the list settings with the given toast settings in place of the defaults.
 * @param toast - The toast settings the site picks.
 */
function serveToastSettings(toast: { toast_milliseconds?: number, toast_name_length?: number }) {
	server.use(http.get('/api/settings', () => HttpResponse.json({ ...siteSettings, ...toast })))
}

test('cuts a long name at the length the site serves', async () => {
	serveToastSettings({ toast_name_length: 10 })

	renderToaster()

	expect(await screen.findByText('A title fa…')).toBeInTheDocument()
})

test('sends a toast away once the time the site serves runs out', async () => {
	serveToastSettings({ toast_milliseconds: 50, toast_name_length: 10 })
	renderToaster()
	await screen.findByText('A title fa…')

	await userEvent.click(screen.getByRole('button', { name: 'raise' }))

	expect(await screen.findByText('A title fa…', { selector: '.godmin-toast' })).toBeInTheDocument()
	await waitFor(() => expect(screen.queryByText('A title fa…', { selector: '.godmin-toast' })).toBeNull())
})

test('keeps the admin kit name length when the settings cannot be read', async () => {
	vi.spyOn(console, 'error').mockImplementation(() => {})
	const asked: string[] = []
	server.use(
		http.get('/api/settings', () => {
			asked.push('settings')
			return HttpResponse.json({ error: 'the store is down' }, { status: 500 })
		}),
	)

	renderToaster()

	await waitFor(() => expect(asked).toEqual(['settings']))
	expect(screen.getByText(`${LONG_TITLE.slice(0, 45).trimEnd()}…`)).toBeInTheDocument()
})
