// SPDX-License-Identifier: Apache-2.0

import { http, HttpResponse, server } from '@gophenberg/frontend-sdk/testing'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { render, screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { beforeEach, expect, test, vi } from 'vitest'

import { allowedMimes, MediaLibraryPicker, selectionFrom } from '../media/MediaLibraryPicker'
import { SiteFormatGate } from '../settings/SiteFormatGate'
import { gate } from './gate'
import { siteSettings } from './siteSettings'

const HARBOR = {
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

const CLIFF = {
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

/**
 * Serves the given rows from the media listing.
 * @param items - The rows the listing answers.
 */
function listing(items: unknown[] = [HARBOR, CLIFF, MANUAL]) {
	server.use(
		http.get('/api/media', () =>
			HttpResponse.json({ items, total: items.length, per_page: siteSettings.picker_page_size }),
		),
	)
}

beforeEach(() => {
	listing()
})

/**
 * Renders the picker behind its trigger and opens it.
 * @param props - The props the block editor would pass.
 * @returns What the picker reported to onSelect.
 */
async function openPicker(props: Record<string, unknown> = {}) {
	const chosen: unknown[] = []
	renderPicker({ onSelect: (media: unknown) => chosen.push(media), ...props })
	await userEvent.click(screen.getByRole('button', { name: 'Media Library' }))
	return chosen
}

/**
 * Renders the picker inside the query client it reads through.
 * @param props - The props the block editor would pass.
 */
function renderPicker(props: Record<string, unknown> = {}) {
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

test('renders the trigger the block editor handed it', () => {
	renderPicker()

	expect(screen.getByRole('button', { name: 'Media Library' })).toBeInTheDocument()
	expect(screen.queryByRole('dialog')).not.toBeInTheDocument()
})

test('opens a library holding what the API listed', async () => {
	await openPicker()

	expect(await screen.findByRole('dialog')).toBeInTheDocument()
	expect(await screen.findByText('Harbor at dawn')).toBeInTheDocument()
	expect(screen.getByText('Mountain ridge')).toBeInTheDocument()
})

test('asks the page size the settings name for the picker', async () => {
	const asked: (string | null)[] = []
	server.use(
		http.get('/api/settings', () => HttpResponse.json({ ...siteSettings, picker_page_size: 100 })),
		http.get('/api/media', ({ request }) => {
			asked.push(new URL(request.url).searchParams.get('per_page'))
			return HttpResponse.json({ items: [HARBOR], total: 1, per_page: 100 })
		}),
	)
	const client = new QueryClient({ defaultOptions: { queries: { retry: false } } })
	render(
		<QueryClientProvider client={client}>
			<SiteFormatGate loading={null}>
				<MediaLibraryPicker
					onSelect={() => {}}
					render={({ open }) => <button onClick={open}>Media Library</button>}
				/>
			</SiteFormatGate>
		</QueryClientProvider>,
	)
	await userEvent.click(await screen.findByRole('button', { name: 'Media Library' }))

	await screen.findByText('Harbor at dawn')

	expect(asked).toEqual(['100'])
})

test('counts its pages by the size the server served', async () => {
	server.use(
		http.get('/api/media', () => HttpResponse.json({ items: [HARBOR, CLIFF], total: 25, per_page: 10 })),
	)
	await openPicker()
	await screen.findByText('Harbor at dawn')

	expect(screen.getByText('of 3')).toBeInTheDocument()
})

const OWN_AUTHOR = '019fb000-0000-7000-8000-0000000000ff'

/**
 * Returns the sort control of View options, drawn inside the picker's dialog where a reader reaches it.
 * @returns The Sort by control.
 */
function sortControl(): Promise<HTMLElement> {
	return screen.findByRole('combobox', { name: 'Sort by' })
}

/**
 * Serves the library and the authors, recording what each request for the library asked for.
 * @returns The parameters of every request so far.
 */
function recording(): URLSearchParams[] {
	const asked: URLSearchParams[] = []
	server.use(
		http.get('/api/media', ({ request }) => {
			asked.push(new URL(request.url).searchParams)
			return HttpResponse.json({ items: [HARBOR, CLIFF], total: 2, per_page: siteSettings.picker_page_size })
		}),
		http.get('/api/authors', () => HttpResponse.json({ items: [{ id: OWN_AUTHOR, name: 'Maria Perez' }] })),
	)
	return asked
}

test.each([
	{ accepts: ['video'], mime: 'video/', name: 'only for the kinds the block accepts' },
	{ accepts: ['image', 'video'], mime: 'image/,video/', name: 'for each kind a mixed block accepts' },
])('asks $name', async ({ accepts, mime }) => {
	const asked = recording()

	await openPicker({ allowedTypes: accepts })

	await waitFor(() => expect(asked.at(-1)?.get('mime')).toBe(mime))
})

test('asks for every kind when the block accepts anything', async () => {
	const asked = recording()

	await openPicker()

	await waitFor(() => expect(asked.at(-1)?.has('mime')).toBe(false))
})

test('offers Author, Date added and Date modified as filters, and no Type, as the WordPress picker does', async () => {
	await openPicker()
	await screen.findByText('Harbor at dawn')

	await userEvent.click(screen.getByRole('button', { name: 'Add filter' }))

	const offered = await screen.findAllByRole('menuitem')
	expect(offered.map((item) => item.textContent)).toEqual(['Author', 'Date added', 'Date modified'])
})

test('sorts by Title, Date added, Date modified and Author alone', async () => {
	await openPicker()
	await screen.findByText('Harbor at dawn')

	await userEvent.click(screen.getByRole('button', { name: 'View options' }))

	const sorts = within(await sortControl()).getAllByRole('option')
	expect(sorts.map((option) => option.textContent)).toEqual(['Title', 'Date added', 'Date modified', 'Author'])
})

test('offers no page sizes, the picker showing the size the site sets for it', async () => {
	await openPicker()
	await screen.findByText('Harbor at dawn')

	await userEvent.click(screen.getByRole('button', { name: 'View options' }))

	expect(await sortControl()).toBeInTheDocument()
	expect(screen.queryByRole('radiogroup', { name: 'Items per page' })).not.toBeInTheDocument()
})

test('asks again with the sort a reader picks once the first page arrived, beside the block kinds', async () => {
	const asked = recording()
	await openPicker({ allowedTypes: ['image'] })
	await screen.findByText('Harbor at dawn')
	await userEvent.click(screen.getByRole('button', { name: 'View options' }))

	await userEvent.selectOptions(await sortControl(), 'modified')

	await waitFor(() => expect(asked.at(-1)?.get('orderby')).toBe('modified'))
	expect(asked.at(-1)?.get('mime')).toBe('image/')
	expect(asked.at(-1)?.get('per_page')).toBe(String(siteSettings.picker_page_size))
})

test('asks again with the author a reader picks once the first page arrived', async () => {
	const asked = recording()
	await openPicker({ allowedTypes: ['image'] })
	await screen.findByText('Harbor at dawn')
	await userEvent.click(screen.getByRole('button', { name: 'Add filter' }))
	await userEvent.click(await screen.findByRole('menuitem', { name: 'Author' }))

	await userEvent.click(await screen.findByRole('option', { name: 'Maria Perez' }))

	await waitFor(() => expect(asked.at(-1)?.get('author')).toBe(OWN_AUTHOR))
	expect(asked.at(-1)?.get('mime')).toBe('image/')
})

test('keeps the cards and the page count while the sort a reader picks loads', async () => {
	server.use(
		http.get('/api/media', () => HttpResponse.json({ items: [HARBOR, CLIFF], total: 25, per_page: 10 })),
	)
	await openPicker()
	await screen.findByText('Harbor at dawn')
	const sorted = gate()
	let asking = false
	server.use(
		http.get('/api/media', async () => {
			asking = true
			await sorted.held
			return HttpResponse.json({ items: [CLIFF, HARBOR], total: 25, per_page: 10 })
		}),
	)
	await userEvent.click(screen.getByRole('button', { name: 'View options' }))

	await userEvent.selectOptions(await sortControl(), 'title')
	await waitFor(() => expect(asking).toBe(true))

	expect(screen.getByText('Harbor at dawn')).toBeInTheDocument()
	expect(screen.getByText('of 3')).toBeInTheDocument()
	sorted.release()
})

test('offers what the block already holds as the selection', async () => {
	const chosen = await openPicker({ value: 7 })
	await screen.findByText('Harbor at dawn')

	await userEvent.click(screen.getByRole('button', { name: /^Select$/ }))

	await waitFor(() => expect(chosen).toHaveLength(1))
	expect(chosen[0]).toMatchObject({ id: 7 })
})

test('hands one chosen item back as an object', async () => {
	const chosen = await openPicker()
	await screen.findByText('Harbor at dawn')

	await userEvent.click(screen.getByText('Harbor at dawn'))
	await userEvent.click(screen.getByRole('button', { name: /^Select$/ }))

	await waitFor(() => expect(chosen).toHaveLength(1))
	expect(chosen[0]).toMatchObject({ id: 7, url: '/media/2026/08/harbor.jpg', alt: 'Boats at sunrise' })
})

test('hands several chosen items back as a list', async () => {
	const chosen = await openPicker({ multiple: true })
	await screen.findByText('Harbor at dawn')

	await userEvent.click(screen.getByText('Harbor at dawn'))
	await userEvent.click(screen.getByText('Mountain ridge'))
	await userEvent.click(screen.getByRole('button', { name: /^Select$/ }))

	await waitFor(() => expect(chosen).toHaveLength(1))
	expect(chosen[0]).toHaveLength(2)
})

test('closes without choosing anything', async () => {
	const chosen = await openPicker()
	await screen.findByText('Harbor at dawn')

	await userEvent.click(screen.getByRole('button', { name: 'Cancel' }))

	await waitFor(() => expect(screen.queryByRole('dialog')).not.toBeInTheDocument())
	expect(chosen).toEqual([])
})

test('dismisses the library without choosing anything', async () => {
	const chosen = await openPicker()
	await screen.findByText('Harbor at dawn')

	await userEvent.keyboard('{Escape}')

	await waitFor(() => expect(screen.queryByRole('dialog')).not.toBeInTheDocument())
	expect(chosen).toEqual([])
})

test('reports a library it could not read', async () => {
	vi.spyOn(console, 'error').mockImplementation(() => {})
	server.use(http.get('/api/media', () => HttpResponse.json({}, { status: 500 })))

	await openPicker()

	expect(await screen.findByRole('alert')).toHaveTextContent(/could not/i)
})

test('says the library is empty when nothing was uploaded', async () => {
	listing([])

	await openPicker()

	expect(await screen.findByText(/no media/i)).toBeInTheDocument()
})

test('narrows the library to what the block accepts', () => {
	expect(allowedMimes(['image'])).toEqual(['image/'])
	expect(allowedMimes(['video'])).toEqual(['video/'])
	expect(allowedMimes(['image/jpeg'])).toEqual(['image/jpeg'])
	expect(allowedMimes(['text/vtt'])).toEqual(['text/vtt'])
	expect(allowedMimes(['image/jpeg', 'image/png'])).toEqual(['image/jpeg', 'image/png'])
	expect(allowedMimes(['image', 'video'])).toEqual(['image/', 'video/'])
	expect(allowedMimes(['image', 'image'])).toEqual(['image/'])
	expect(allowedMimes([])).toEqual([])
	expect(allowedMimes(undefined)).toEqual([])
})

test('picks the selection a block value stands for', () => {
	expect(selectionFrom(undefined)).toEqual([])
	expect(selectionFrom(7)).toEqual(['7'])
	expect(selectionFrom([7, 9])).toEqual(['7', '9'])
	expect(selectionFrom([7, undefined as unknown as number])).toEqual(['7'])
})
