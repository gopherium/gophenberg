// SPDX-License-Identifier: Apache-2.0

import { http, HttpResponse, server } from '@gophenberg/frontend-sdk/testing'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { act, render, screen } from '@testing-library/react'
import type { RenderOptions } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import type { ReactNode } from 'react'

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

export const MANUAL = {
	...HARBOR,
	id: 8,
	type: 'file',
	file: '2026/08/manual.pdf',
	title: 'Manual',
	mime_type: 'application/pdf',
}

export const OWN_AUTHOR = '019fb000-0000-7000-8000-0000000000ff'

/** A picture the server stores as the harbor row. */
export const PHOTO = new File(['bytes'], 'harbor.jpg', { type: 'image/jpeg' })

/** A picture the server stores as the cliff row. */
export const SECOND = new File(['bytes'], 'cliff.jpg', { type: 'image/jpeg' })

/** A text file, which no image block takes and the server refuses. */
export const NOTES = new File(['notes'], 'notes.txt', { type: 'text/plain' })

/** A picture with nothing in it. */
export const HOLLOW = new File([], 'hollow.jpg', { type: 'image/jpeg' })

/** A file the browser knows no type for, which the server stores as the manual row. */
export const TYPELESS = new File(['bytes'], 'manual', { type: '' })

/** The reason the server refuses a file it holds no row for. */
export const REASON = 'the file type is not allowed'

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
 * @param options - How to render it, a wrapper around it among them.
 */
export function renderPicker(props: Record<string, unknown> = {}, options: RenderOptions = {}) {
	const client = new QueryClient({ defaultOptions: { queries: { retry: false } } })
	render(
		<QueryClientProvider client={client}>
			<MediaLibraryPicker
				onSelect={() => {}}
				render={({ open }) => <button onClick={open}>Media Library</button>}
				{...props}
			/>
		</QueryClientProvider>,
		options,
	)
}

/**
 * Renders two pickers behind the triggers First picker and Second picker, inside the query client they read through.
 * @param options - How to render them, a wrapper around them among them.
 * @returns What takes the first picker off the screen, as a block deleted with its dialog open goes.
 */
export function renderTwoPickers(options: RenderOptions = {}) {
	const client = new QueryClient({ defaultOptions: { queries: { retry: false } } })
	const tree = (first: ReactNode) => (
		<QueryClientProvider client={client}>
			{first}
			<MediaLibraryPicker
				onSelect={() => {}}
				render={({ open }) => <button onClick={open}>Second picker</button>}
			/>
		</QueryClientProvider>
	)
	const { rerender } = render(
		tree(
			<MediaLibraryPicker
				onSelect={() => {}}
				render={({ open }) => <button onClick={open}>First picker</button>}
			/>,
		),
		options,
	)
	return () => rerender(tree(null))
}

/**
 * Renders the picker behind its trigger and opens it.
 * @param props - The props the block editor would pass.
 * @param options - How to render it, a wrapper around it among them.
 * @returns What the picker reported to onSelect.
 */
export async function openPicker(props: Record<string, unknown> = {}, options: RenderOptions = {}) {
	const chosen: unknown[] = []
	renderPicker({ onSelect: (media: unknown) => chosen.push(media), ...props }, options)
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

/**
 * Returns the frame of the open picker, told apart from the panel its status opens.
 * @returns The dialog named by the default title.
 */
export function frame(): HTMLElement {
	return screen.getByRole('dialog', { name: 'Select Media' })
}

/**
 * Picks the files through the hidden input behind Upload media, as a reader who widens the file dialog's offer may.
 * @param files - The files to pick.
 */
export async function choose(...files: File[]) {
	await userEvent.setup({ applyAccept: false }).upload(screen.getByLabelText('Upload media'), files)
}

/**
 * Drops the files on the zone laid over the open picker, as a drag from the desktop would.
 * @param files - The files the drag carries.
 */
export function drop(...files: File[]) {
	const zone = document.querySelector('.gophenberg-media-picker .components-drop-zone') as HTMLElement
	const dataTransfer = { files, items: [], types: ['Files'], getData: () => '' }
	act(() => {
		document.dispatchEvent(Object.assign(new Event('dragenter', { bubbles: true }), { dataTransfer }))
		zone.dispatchEvent(Object.assign(new Event('drop', { bubbles: true, cancelable: true }), { dataTransfer }))
	})
}

/**
 * Returns what the server answers an upload by the name of its file, the row it stores or the reason it refuses.
 * @param sent - The file sent.
 * @returns The answer.
 */
export function stored(sent: { name: string }): Response {
	const row = [HARBOR, CLIFF, MANUAL].find((candidate) => candidate.file.includes(sent.name))
	if (row === undefined) {
		return HttpResponse.json({ error: REASON }, { status: 422 })
	}
	return HttpResponse.json(row, { status: 201 })
}

/**
 * Answers every upload as the given function does, recording the name of each file sent.
 * @param answer - Gives the answer by the file sent, storing the rows this file knows when absent.
 * @returns The names of the files sent so far.
 */
export function uploadsAnswering(answer: (file: { name: string }) => Response | Promise<Response> = stored): string[] {
	const sent: string[] = []
	server.use(
		http.post('/api/media', async ({ request }) => {
			const name = /filename="([^"]+)"/.exec(await request.text())?.[1] ?? ''
			sent.push(name)
			return answer({ name })
		}),
	)
	return sent
}
