// SPDX-License-Identifier: Apache-2.0

import { http, HttpResponse, server } from '@gophenberg/frontend-sdk/testing'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'

import { MediaLibraryPicker } from '../media/MediaLibraryPicker'
import { siteSettings } from './siteSettings'

export const HARBOR = {
	id: 7,
	type: 'image',
	file: '2026/08/harbor.jpg',
	title: 'Harbor at dawn',
	alt_text: 'Boats at sunrise',
	caption: 'Before the market opens',
	mime_type: 'image/jpeg',
	width: 1600,
	height: 1000,
	filesize: 254000,
	sizes: {},
	updated_at: '2026-08-10T10:00:00Z',
}

export const CLIFF = {
	...HARBOR,
	id: 9,
	file: '2026/08/cliff.jpg',
	title: 'Mountain ridge',
	alt_text: 'A dark ridge',
}

const MANUAL = {
	...HARBOR,
	id: 8,
	type: 'file',
	file: '2026/08/manual.pdf',
	title: 'Manual',
	mime_type: 'application/pdf',
}

export const OWN_AUTHOR = '019fb000-0000-7000-8000-0000000000ff'

/**
 * Serves the given rows from the media listing, and the rows an ids query names in the order it asked for them.
 * @param items - The rows the listing answers.
 */
export function listing(items: (typeof HARBOR)[] = [HARBOR, CLIFF, MANUAL]) {
	server.use(
		http.get('/api/media', ({ request }) => {
			const ids = new URL(request.url).searchParams.get('ids')
			if (ids === null) {
				return HttpResponse.json({ items, total: items.length, per_page: siteSettings.picker_page_size })
			}
			const named = ids.split(',').flatMap((id) => items.filter((item) => String(item.id) === id))
			return HttpResponse.json({ items: named, total: named.length })
		}),
	)
}

/**
 * Serves the library and the authors, recording what each request for the library asked for.
 * @param total - How many files the library says it holds.
 * @returns The parameters of every request so far.
 */
export function recording(total = 2): URLSearchParams[] {
	const asked: URLSearchParams[] = []
	server.use(
		http.get('/api/media', ({ request }) => {
			asked.push(new URL(request.url).searchParams)
			return HttpResponse.json({ items: [HARBOR, CLIFF], total, per_page: siteSettings.picker_page_size })
		}),
		http.get('/api/authors', () => HttpResponse.json({ items: [{ id: OWN_AUTHOR, name: 'Maria Perez' }] })),
	)
	return asked
}

/**
 * Renders the picker inside the query client it reads through.
 * @param props - The props the block editor would pass.
 */
export function renderPicker(props: Record<string, unknown> = {}) {
	const client = new QueryClient({ defaultOptions: { queries: { retry: false } } })
	render(
		<QueryClientProvider client={client}>
			<MediaLibraryPicker
				onSelect={() => {}}
				render={({ open }) => <button onClick={open}>Media Library</button>}
				{...props}
			/>
		</QueryClientProvider>,
	)
}

/**
 * Renders the picker behind its trigger and opens it.
 * @param props - The props the block editor would pass.
 * @returns What the picker reported to onSelect.
 */
export async function openPicker(props: Record<string, unknown> = {}) {
	const chosen: unknown[] = []
	renderPicker({ onSelect: (media: unknown) => chosen.push(media), ...props })
	await userEvent.click(screen.getByRole('button', { name: 'Media Library' }))
	return chosen
}

/**
 * Returns the card a row shows as, once the picker drew it.
 * @param fixture - The row the card stands for.
 * @returns The card, named by the row's title.
 */
export function card(fixture: { title: string }): Promise<HTMLElement> {
	return screen.findByRole('option', { name: fixture.title })
}
