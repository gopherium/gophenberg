// SPDX-License-Identifier: Apache-2.0

import { http, HttpResponse, server } from '@gophenberg/frontend-sdk/testing'
import { formatDate, formatNumber, rememberFormatLocale, rememberLocale } from '@gopherium/gottext'
import { createAuthQueryClient } from '@gopherium/react-auth'
import { QueryClientProvider } from '@tanstack/react-query'
import { render, screen } from '@testing-library/react'
import { afterEach, expect, test } from 'vitest'

import { DEFAULT_LOCALE } from '../i18n/catalog'
import { SiteFormatGate } from '../settings/SiteFormatGate'
import { renderAt } from './render'
import { siteSettings } from './siteSettings'
import { warmPostsScreen } from './warm'

warmPostsScreen()

afterEach(() => {
	rememberLocale(DEFAULT_LOCALE)
})

/** A published post the list shows with its date. */
const PUBLISHED = {
	id: '019fb000-0000-7000-8000-000000000001',
	type: 'post',
	slug: 'welcome',
	title: 'Welcome to Gophenberg',
	excerpt: '',
	status: 'published',
	author_id: '019fb000-0000-7000-8000-0000000000ff',
	author_name: 'Maria Perez',
	published_at: '2026-07-20T10:00:00Z',
	created_at: '2026-07-19T10:00:00Z',
	updated_at: '2026-07-20T10:00:00Z',
}

/**
 * Shows one day and one count as the screens write them.
 * @returns The dated line.
 */
function Dated() {
	return <p>{`${formatDate('2026-10-04')} ${formatNumber(1234)}`}</p>
}

/**
 * Renders the gate around a dated screen, the loading text standing in until it opens.
 * @param client - The query client the gate reads the settings through.
 */
function renderGate(client = createAuthQueryClient({ queries: { retry: false, staleTime: Infinity } })) {
	render(
		<QueryClientProvider client={client}>
			<SiteFormatGate loading={<p>Loading settings</p>}>
				<Dated />
			</SiteFormatGate>
		</QueryClientProvider>,
	)
}

/**
 * Serves the site settings naming the given format locale.
 * @param formatLocale - The locale the settings name.
 */
function serveSettings(formatLocale: string) {
	server.use(http.get('/api/settings', () => HttpResponse.json({ ...siteSettings, format_locale: formatLocale })))
}

/**
 * Serves the site settings naming the given format locale once the returned call lets them answer.
 * @param formatLocale - The locale the settings name.
 * @returns The call that lets the settings answer.
 */
function holdSettings(formatLocale: string): () => void {
	let release = () => {}
	const released = new Promise<void>((resolve) => {
		release = resolve
	})
	server.use(
		http.get('/api/settings', async () => {
			await released
			return HttpResponse.json({ ...siteSettings, format_locale: formatLocale })
		}),
	)
	return release
}

test('holds the screens back until the site names its format', async () => {
	rememberFormatLocale(undefined)
	const release = holdSettings('de-DE')
	renderGate()

	expect(await screen.findByText('Loading settings')).toBeInTheDocument()
	release()

	expect(await screen.findByText('04.10.2026 1.234')).toBeInTheDocument()
	expect(screen.queryByText('Loading settings')).not.toBeInTheDocument()
})

test.each(['es-ES', 'en-US'])('writes dates and counts in the site format for a reader of %s', async (reader) => {
	rememberFormatLocale(undefined)
	rememberLocale(reader)
	serveSettings('es-ES')

	renderGate()

	expect(await screen.findByText('04/10/2026 1.234')).toBeInTheDocument()
})

test('opens the screens in the reader language when the settings cannot be read', async () => {
	rememberLocale('en-US')
	server.use(http.get('/api/settings', () => HttpResponse.json({ error: 'boom' }, { status: 500 })))

	renderGate()

	expect(await screen.findByText('10/04/2026 1,234')).toBeInTheDocument()
})

test('opens the screens on the first failed read through the client the admin builds', async () => {
	rememberLocale('en-US')
	let asked = 0
	server.use(
		http.get('/api/settings', () => {
			asked += 1
			return HttpResponse.json({ error: 'boom' }, { status: 500 })
		}),
	)

	renderGate(createAuthQueryClient())

	expect(await screen.findByText('10/04/2026 1,234')).toBeInTheDocument()
	expect(asked).toBe(1)
})

test('writes the dates of the admin screens in the format the site names', async () => {
	serveSettings('de-DE')
	server.use(
		http.get('/api/content', () => HttpResponse.json({ items: [PUBLISHED], total: 1 })),
		http.get('/api/content/counts', () => HttpResponse.json({ published: 1 })),
	)

	renderAt('/content/post')

	expect(await screen.findByText(/20\.07\.2026/)).toBeInTheDocument()
})
