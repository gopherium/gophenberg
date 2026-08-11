// SPDX-License-Identifier: Apache-2.0

import { http, HttpResponse, server } from '@gophenberg/frontend-sdk/testing'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { render, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { beforeEach, expect, test, vi } from 'vitest'

import { MediaLibraryPicker, narrowedMime, selectionFrom } from '../media/MediaLibraryPicker'

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
	server.use(http.get('/api/media', () => HttpResponse.json({ items, total: items.length })))
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

test('asks only for the kinds the block accepts', async () => {
	let asked = ''
	server.use(
		http.get('/api/media', ({ request }) => {
			asked = new URL(request.url).search
			return HttpResponse.json({ items: [HARBOR], total: 1 })
		}),
	)

	await openPicker({ allowedTypes: ['video'] })

	await waitFor(() => expect(asked).toContain('mime=video'))
})

test('asks for every kind when the block accepts anything', async () => {
	let asked = ''
	server.use(
		http.get('/api/media', ({ request }) => {
			asked = new URL(request.url).search
			return HttpResponse.json({ items: [HARBOR], total: 1 })
		}),
	)

	await openPicker()

	await waitFor(() => expect(asked).not.toContain('mime='))
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
	expect(narrowedMime(['image'])).toBe('image')
	expect(narrowedMime(['video'])).toBe('video')
	expect(narrowedMime(['audio'])).toBe('audio')
	expect(narrowedMime(['image/jpeg'])).toBe('image/jpeg')
	expect(narrowedMime(['text/vtt'])).toBe('text/vtt')
	expect(narrowedMime(['image/jpeg', 'image/png'])).toBe('image')
	expect(narrowedMime(['image', 'video'])).toBe('')
	expect(narrowedMime([])).toBe('')
	expect(narrowedMime(undefined)).toBe('')
})

test('picks the selection a block value stands for', () => {
	expect(selectionFrom(undefined)).toEqual([])
	expect(selectionFrom(7)).toEqual(['7'])
	expect(selectionFrom([7, 9])).toEqual(['7', '9'])
	expect(selectionFrom([7, undefined as unknown as number])).toEqual(['7'])
})
