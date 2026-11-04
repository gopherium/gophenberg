// SPDX-License-Identifier: Apache-2.0

import { http, HttpResponse, server } from '@gophenberg/frontend-sdk/testing'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { act, render, screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { beforeEach, expect, test, vi } from 'vitest'

import { allowedMimes, MediaLibraryPicker, selectionFrom } from '../media/MediaLibraryPicker'
import { SiteFormatGate } from '../settings/SiteFormatGate'
import { gate } from './gate'
import { card, CLIFF, HARBOR, listing, openPicker, OWN_AUTHOR, recording, renderPicker } from './mediaPicker'
import { siteSettings } from './siteSettings'

beforeEach(() => {
	listing()
})

test('renders the trigger the block editor handed it', () => {
	renderPicker()

	expect(screen.getByRole('button', { name: 'Media Library' })).toBeInTheDocument()
	expect(screen.queryByRole('dialog')).not.toBeInTheDocument()
})

test('opens a library holding what the API listed', async () => {
	await openPicker()

	expect(await screen.findByRole('dialog')).toBeInTheDocument()
	expect(await card(HARBOR)).toBeInTheDocument()
	expect(await card(CLIFF)).toBeInTheDocument()
})

test.each([
	{ props: {}, name: 'Select Media', how: 'when the block names no title' },
	{ props: { title: 'Select poster image' }, name: 'Select poster image', how: 'when the block names that title' },
])('reads $name $how', async ({ props, name }) => {
	await openPicker(props)

	expect(await screen.findByRole('dialog', { name })).toBeInTheDocument()
})

test('puts the focus on the dialog frame', async () => {
	await openPicker()
	await card(HARBOR)

	await waitFor(() => expect(screen.getByRole('dialog')).toHaveFocus())
})

test.each([
	{ way: 'its close button', press: () => userEvent.click(screen.getByRole('button', { name: 'Close' })) },
	{ way: 'Escape', press: () => userEvent.keyboard('{Escape}') },
	{ way: 'a press outside it', press: () => userEvent.click(screen.getByTestId('dialog-backdrop')) },
])('closes by $way and chooses nothing', async ({ press }) => {
	const chosen = await openPicker()
	await card(HARBOR)

	await press()

	await waitFor(() => expect(screen.queryByRole('dialog')).not.toBeInTheDocument())
	expect(chosen).toEqual([])
})

test.each(['Cancel', 'Layout'])('draws no %s button', async (name) => {
	await openPicker()
	await card(HARBOR)

	expect(screen.queryByRole('button', { name })).not.toBeInTheDocument()
})

test('names the list of cards', async () => {
	await openPicker()

	expect(await screen.findByRole('listbox', { name: 'Media items' })).toBeInTheDocument()
})

test.each([
	{ row: HARBOR, name: 'Harbor at dawn' },
	{ row: { ...HARBOR, title: '' }, name: '(no title)' },
])('names the card $name and draws no title on it', async ({ row, name }) => {
	listing([row])

	await openPicker()

	expect(await screen.findByRole('option', { name })).toBeInTheDocument()
	expect(screen.queryByText(name)).not.toBeInTheDocument()
})

test('draws the grid dense, each picture whole', async () => {
	await openPicker()
	await card(HARBOR)

	expect(screen.getByRole('listbox')).toHaveClass('has-compact-density', 'has-media-fit-contain')
})

test('turns the whole picture off in View options without asking the server again', async () => {
	const asked = recording()
	await openPicker()
	await card(HARBOR)
	await waitFor(() => expect(asked.at(-1)?.get('per_page')).toBe(String(siteSettings.picker_page_size)))
	const askedBefore = asked.length
	await userEvent.click(screen.getByRole('button', { name: 'View options' }))

	await userEvent.click(await screen.findByRole('checkbox', { name: 'Original aspect ratio' }))

	await waitFor(() => expect(screen.getByRole('listbox')).not.toHaveClass('has-media-fit-contain'))
	expect(asked).toHaveLength(askedBefore)
})

test('never lets a reader hide the thumbnail, a second property showing or not', async () => {
	await openPicker()
	await card(HARBOR)
	await userEvent.click(screen.getByRole('button', { name: 'View options' }))

	await userEvent.click(await screen.findByRole('button', { name: 'Title' }))

	expect(await screen.findByText(HARBOR.title)).toBeInTheDocument()
	expect(screen.queryByRole('button', { name: 'Thumbnail' })).not.toBeInTheDocument()
})

test('asks for the newest files first', async () => {
	const asked = recording()

	await openPicker()

	await waitFor(() => expect(asked).not.toHaveLength(0))
	expect(asked[0].get('orderby')).toBe('date')
	expect(asked[0].get('order')).toBe('desc')
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

	await card(HARBOR)

	expect(asked).toEqual(['100'])
})

test('counts its pages by the size the server served', async () => {
	server.use(
		http.get('/api/media', () => HttpResponse.json({ items: [HARBOR, CLIFF], total: 25, per_page: 10 })),
	)
	await openPicker()
	await card(HARBOR)

	expect(screen.getByText('of 3')).toBeInTheDocument()
	await userEvent.click(screen.getByRole('combobox', { name: 'Current page' }))
	await screen.findByRole('option', { name: '3' })
	const pages = screen.getAllByRole('option').map((option) => option.textContent)
	expect(pages.filter((text) => /^\d+$/.test(text ?? ''))).toEqual(['1', '2', '3'])
})

/**
 * Returns the sort control of View options, drawn inside the picker's dialog where a reader reaches it.
 * @returns The Sort by control.
 */
function sortControl(): Promise<HTMLElement> {
	return screen.findByRole('combobox', { name: 'Sort by' })
}

/**
 * Returns the page sizes View options offers, opening it first.
 * @returns The queries bound to the sizes.
 */
async function offeredSizes() {
	await userEvent.click(screen.getByRole('button', { name: 'View options' }))
	return within(await screen.findByRole('radiogroup', { name: 'Items per page' }))
}

/**
 * Narrows the open picker to the one author the library knows.
 */
async function filterByAuthor() {
	await userEvent.click(screen.getByRole('button', { name: 'Add filter' }))
	await userEvent.click(await screen.findByRole('menuitem', { name: 'Author' }))
	await userEvent.click(await screen.findByRole('option', { name: 'Maria Perez' }))
}

/**
 * Types a search into the open picker.
 */
async function searchHarbor() {
	await userEvent.type(screen.getByRole('searchbox', { name: 'Search media…' }), 'harbor')
}

/**
 * Leaves the open picker as it is.
 */
async function leaveAsIs() {}

/**
 * Waits for the picker's dialog to leave.
 * @returns A promise settling once it left.
 */
function dialogGone() {
	return waitFor(() => expect(screen.queryByRole('dialog')).not.toBeInTheDocument())
}

/**
 * Closes the picker by its close button and opens it again.
 */
async function closeAndReopen() {
	await userEvent.click(screen.getByRole('button', { name: 'Close' }))
	await dialogGone()
	await userEvent.click(await screen.findByRole('button', { name: 'Media Library' }))
}

test.each([
	{ accepts: ['video'], mime: 'video/', name: 'only for the kinds the block accepts' },
	{ accepts: ['image', 'video'], mime: 'image/,video/', name: 'for each kind a mixed block accepts' },
	{ accepts: ['image/*'], mime: 'image/', name: 'for the family a block writes with a star' },
])('asks $name', async ({ accepts, mime }) => {
	const asked = recording()

	await openPicker({ allowedTypes: accepts })

	await waitFor(() => expect(asked.at(-1)?.get('mime')).toBe(mime))
})

test.each([
	{ props: {}, name: 'names no kinds' },
	{ props: { allowedTypes: ['*'] }, name: 'accepts anything' },
])('asks for every kind when the block $name', async ({ props }) => {
	const asked = recording()

	await openPicker(props)

	await waitFor(() => expect(asked).not.toHaveLength(0))
	expect(asked.at(-1)?.has('mime')).toBe(false)
})

test('offers Author, Date added and Date modified as filters, and no Type, as the WordPress picker does', async () => {
	await openPicker()
	await card(HARBOR)

	await userEvent.click(screen.getByRole('button', { name: 'Add filter' }))

	const offered = await screen.findAllByRole('menuitem')
	expect(offered.map((item) => item.textContent)).toEqual(['Author', 'Date added', 'Date modified'])
})

test('sorts by Title, Date added, Date modified and Author alone', async () => {
	await openPicker()
	await card(HARBOR)

	await userEvent.click(screen.getByRole('button', { name: 'View options' }))

	const sorts = within(await sortControl()).getAllByRole('option')
	expect(sorts.map((option) => option.textContent)).toEqual(['Title', 'Date added', 'Date modified', 'Author'])
})

test('offers the served page sizes, opens on the size the site sets, and asks a new size at page one', async () => {
	const asked = recording()
	await openPicker()
	await card(HARBOR)

	const sizes = await offeredSizes()

	expect(sizes.getAllByRole('radio').map((size) => size.textContent)).toEqual(
		siteSettings.list_page_sizes.map(String),
	)
	expect(sizes.getByRole('radio', { name: String(siteSettings.picker_page_size) })).toBeChecked()
	const smaller = sizes.getByRole('radio', { name: '20' })
	expect(smaller).toBeEnabled()
	await userEvent.click(smaller)
	await waitFor(() => expect(asked.at(-1)?.get('per_page')).toBe('20'))
	expect(asked.at(-1)?.has('page')).toBe(false)
})

test('keeps a page size chosen in View options after a close and a reopen', async () => {
	const asked = recording()
	await openPicker()
	await card(HARBOR)
	await userEvent.click((await offeredSizes()).getByRole('radio', { name: '20' }))

	await closeAndReopen()
	await card(HARBOR)

	expect((await offeredSizes()).getByRole('radio', { name: '20' })).toBeChecked()
	expect(asked.at(-1)?.get('per_page')).toBe('20')
})

test('forgets the page and the search on a close, and keeps the sort and a filter', async () => {
	const asked = recording(120)
	await openPicker()
	await card(HARBOR)
	await filterByAuthor()
	await userEvent.click(screen.getByRole('button', { name: 'View options' }))
	await userEvent.selectOptions(await sortControl(), 'modified')
	await searchHarbor()
	await waitFor(() => expect(asked.at(-1)?.get('search')).toBe('harbor'))
	await userEvent.click(screen.getByRole('button', { name: 'Next page' }))
	await waitFor(() => expect(asked.at(-1)?.get('page')).toBe('2'))

	await closeAndReopen()

	await waitFor(() => expect(asked.at(-1)?.has('search')).toBe(false))
	expect(asked.at(-1)?.has('page')).toBe(false)
	expect(asked.at(-1)?.get('orderby')).toBe('modified')
	expect(asked.at(-1)?.get('author')).toBe(OWN_AUTHOR)
	expect(await screen.findByRole('searchbox', { name: 'Search media…' })).toHaveValue('')
})

test('asks again with the sort a reader picks once the first page arrived, beside the block kinds', async () => {
	const asked = recording()
	await openPicker({ allowedTypes: ['image'] })
	await card(HARBOR)
	await userEvent.click(screen.getByRole('button', { name: 'View options' }))

	await userEvent.selectOptions(await sortControl(), 'modified')

	await waitFor(() => expect(asked.at(-1)?.get('orderby')).toBe('modified'))
	expect(asked.at(-1)?.get('mime')).toBe('image/')
	expect(asked.at(-1)?.get('per_page')).toBe(String(siteSettings.picker_page_size))
})

test('asks again with the author a reader picks once the first page arrived', async () => {
	const asked = recording()
	await openPicker({ allowedTypes: ['image'] })
	await card(HARBOR)

	await filterByAuthor()

	await waitFor(() => expect(asked.at(-1)?.get('author')).toBe(OWN_AUTHOR))
	expect(asked.at(-1)?.get('mime')).toBe('image/')
})

test('keeps the cards and the page count while the sort a reader picks loads', async () => {
	server.use(
		http.get('/api/media', () => HttpResponse.json({ items: [HARBOR, CLIFF], total: 25, per_page: 10 })),
	)
	await openPicker()
	await card(HARBOR)
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

	expect(screen.getByRole('option', { name: HARBOR.title })).toBeInTheDocument()
	expect(screen.getByText('of 3')).toBeInTheDocument()
	sorted.release()
})

test('offers what the block already holds as the selection', async () => {
	const chosen = await openPicker({ value: 7 })
	await card(HARBOR)

	await userEvent.click(screen.getByRole('button', { name: /^Select$/ }))

	await waitFor(() => expect(chosen).toHaveLength(1))
	expect(chosen[0]).toMatchObject({ id: 7 })
})

test('hands one chosen item back as an object', async () => {
	const chosen = await openPicker()

	await userEvent.click(await card(CLIFF))
	await userEvent.click(screen.getByRole('button', { name: /^Select$/ }))

	await waitFor(() => expect(chosen).toHaveLength(1))
	expect(chosen[0]).toMatchObject({ id: 9, url: '/media/2026/08/cliff.jpg', alt: 'A dark ridge' })
})

test('hands several chosen items back as a list', async () => {
	const chosen = await openPicker({ multiple: true })

	await userEvent.click(await card(HARBOR))
	await userEvent.click(await card(CLIFF))
	await userEvent.click(screen.getByRole('button', { name: /^Select$/ }))

	await waitFor(() => expect(chosen).toHaveLength(1))
	expect(chosen[0]).toHaveLength(2)
})

const PIER = { ...CLIFF, id: 11, file: '2026/08/pier.jpg', title: 'Pier at dusk', alt_text: 'An empty pier' }

/** The identity of a file the library no longer holds. */
const GONE = 99

/** A read of named files that found the first file. */
const FOUND = () => HttpResponse.json({ items: [HARBOR], total: 1 })

/** A read of named files that found none of them. */
const NOTHING = () => HttpResponse.json({ items: [], total: 0 })

/** A read of named files the server could not answer. */
const FAILED = () => HttpResponse.json({}, { status: 500 })

/**
 * Serves the library one row a page, and the rows an ids query names in the order it asked for them.
 * @returns The parameters of every listing and the ids of every read of named files so far.
 */
function onePerPage() {
	const rows = [HARBOR, CLIFF, PIER]
	const listed: URLSearchParams[] = []
	const named: string[] = []
	server.use(
		http.get('/api/media', ({ request }) => {
			const query = new URL(request.url).searchParams
			const ids = query.get('ids')
			if (ids === null) {
				listed.push(query)
				const page = Number(query.get('page') ?? 1)
				return HttpResponse.json({ items: [rows[page - 1]], total: rows.length, per_page: 1 })
			}
			named.push(ids)
			const found = ids.split(',').flatMap((id) => rows.filter((row) => String(row.id) === id))
			return HttpResponse.json({ items: found, total: found.length })
		}),
	)
	return { listed, named }
}

/**
 * Serves the library's first page, and answers every read of named files as the given function does.
 * @param answer - Gives the answer to a read of named files.
 */
function namedReadsAnswering(answer: () => Response | Promise<Response>) {
	server.use(
		http.get('/api/media', ({ request }) => {
			if (new URL(request.url).searchParams.has('ids')) {
				return answer()
			}
			return HttpResponse.json({ items: [HARBOR, CLIFF], total: 2, per_page: siteSettings.picker_page_size })
		}),
	)
}

/**
 * Holds every read of named files until the gate opens, then answers it as the given function does.
 * @param answer - Gives the answer once the gate opened.
 * @returns The gate.
 */
function heldReads(answer: () => Response) {
	const read = gate()
	namedReadsAnswering(async () => {
		await read.held
		return answer()
	})
	return read
}

/**
 * Presses the Select button of the open picker, once a read before this press let it go.
 */
async function pressSelect() {
	const select = screen.getByRole('button', { name: /^Select$/ })
	await waitFor(() => expect(select).not.toHaveAttribute('aria-disabled', 'true'))
	await userEvent.click(select)
}

/**
 * Ticks the card of a row and presses Select.
 * @param fixture - The row to tick.
 */
async function tickAndSelect(fixture: { title: string }) {
	await userEvent.click(await card(fixture))
	await pressSelect()
}

/**
 * Closes the picker by Escape.
 */
async function escapePicker() {
	await userEvent.keyboard('{Escape}')
	await dialogGone()
}

/**
 * Opens the gate holding a read and gives its answer the time to reach the picker.
 * @param read - The gate holding the read.
 * @returns A promise settling once the time has passed.
 */
function released(read: ReturnType<typeof gate>) {
	read.release()
	return act(() => new Promise((resolve) => setTimeout(resolve, 50)))
}

test('hands a block that takes many each file ticked, on any page, in the order ticked, read once', async () => {
	const { named } = onePerPage()
	const onClose = vi.fn()
	const chosen = await openPicker({ multiple: true, onClose })
	await card(HARBOR)
	await userEvent.click(screen.getByRole('button', { name: 'Next page' }))
	await userEvent.click(await card(CLIFF))
	await userEvent.click(screen.getByRole('button', { name: 'Previous page' }))

	await tickAndSelect(HARBOR)

	await dialogGone()
	expect(chosen).toHaveLength(1)
	expect(chosen[0]).toMatchObject([{ id: CLIFF.id }, { id: HARBOR.id }])
	expect(named).toEqual([`${CLIFF.id},${HARBOR.id}`])
	expect(onClose).toHaveBeenCalledOnce()
})

test.each([
	{ how: 'the file it holds off the page on show', props: { value: CLIFF.id }, handed: { id: CLIFF.id } },
	{
		how: 'what the library still holds of its files',
		props: { multiple: true, value: [CLIFF.id, GONE] },
		handed: [{ id: CLIFF.id }],
	},
])('hands a block $how', async ({ props, handed }) => {
	onePerPage()
	const chosen = await openPicker(props)
	await card(HARBOR)

	await pressSelect()

	await waitFor(() => expect(chosen).toHaveLength(1))
	expect(chosen[0]).toMatchObject(handed)
})

test('reads the ticked files in calls of the largest size the settings serve', async () => {
	server.use(
		http.get('/api/settings', () =>
			HttpResponse.json({ ...siteSettings, list_page_sizes: [1, 2], picker_page_size: 2 }),
		),
	)
	const { listed, named } = onePerPage()
	const chosen = await openPicker({ multiple: true, value: [HARBOR.id, CLIFF.id, PIER.id] })
	await waitFor(() => expect(listed.at(-1)?.get('per_page')).toBe('2'))

	await pressSelect()

	await waitFor(() => expect(chosen).toHaveLength(1))
	expect(named).toEqual([`${HARBOR.id},${CLIFF.id}`, String(PIER.id)])
	expect(chosen[0]).toMatchObject([{ id: HARBOR.id }, { id: CLIFF.id }, { id: PIER.id }])
})

test.each([
	{ how: 'fails', answer: FAILED, words: 'The media library could not be loaded.' },
	{ how: 'finds nothing', answer: NOTHING, words: 'The selected media is no longer in the library.' },
])('says why and keeps its ticks when the read $how, then works on a second press', async ({ answer, words }) => {
	namedReadsAnswering(answer)
	const chosen = await openPicker()

	await tickAndSelect(CLIFF)

	expect(await screen.findByRole('alert')).toHaveTextContent(words)
	expect(chosen).toEqual([])
	expect(await card(CLIFF)).toHaveAttribute('aria-selected', 'true')
	listing()
	await pressSelect()
	await waitFor(() => expect(chosen).toHaveLength(1))
	expect(chosen[0]).toMatchObject({ id: CLIFF.id })
})

test('lets a one file block holding a file the library lost tick another in its place', async () => {
	onePerPage()
	const chosen = await openPicker({ value: GONE })
	await card(HARBOR)
	await pressSelect()
	await screen.findByRole('alert')
	expect(chosen).toEqual([])

	await tickAndSelect(HARBOR)

	await waitFor(() => expect(chosen).toHaveLength(1))
	expect(chosen[0]).toMatchObject({ id: HARBOR.id })
})

test('takes the words of a failed read away while a new press reads again', async () => {
	namedReadsAnswering(FAILED)
	await openPicker()
	await tickAndSelect(CLIFF)
	await screen.findByRole('alert')
	const read = heldReads(FOUND)

	await pressSelect()

	await waitFor(() => expect(screen.queryByRole('alert')).not.toBeInTheDocument())
	await released(read)
})

test('forgets the words of a failed read on a close', async () => {
	namedReadsAnswering(FAILED)
	await openPicker()
	await tickAndSelect(CLIFF)
	await screen.findByRole('alert')

	await closeAndReopen()
	await card(HARBOR)

	expect(screen.queryByRole('alert')).not.toBeInTheDocument()
})

test('fills no block and tells of the close once when Escape outruns the read of the ticked files', async () => {
	const read = heldReads(FOUND)
	const onClose = vi.fn()
	const chosen = await openPicker({ onClose })
	await tickAndSelect(HARBOR)
	await escapePicker()

	await released(read)

	expect(chosen).toEqual([])
	expect(onClose).toHaveBeenCalledOnce()
})

test('shows no alert on the next open when the read of the ticked files fails after Escape', async () => {
	const read = heldReads(FAILED)
	await openPicker()
	await tickAndSelect(HARBOR)
	await escapePicker()
	await released(read)

	await userEvent.click(screen.getByRole('button', { name: 'Media Library' }))
	await card(HARBOR)

	expect(screen.queryByRole('alert')).not.toBeInTheDocument()
})

test.each([
	{ how: 'finds the file', answer: FOUND },
	{ how: 'fails', answer: FAILED },
])('stays open and silent when a read from before a close and a reopen $how', async ({ answer }) => {
	const read = heldReads(answer)
	const chosen = await openPicker()
	await tickAndSelect(HARBOR)
	await closeAndReopen()
	await card(HARBOR)

	await released(read)

	expect(screen.getByRole('dialog')).toBeInTheDocument()
	expect(chosen).toEqual([])
	expect(screen.queryByRole('alert')).not.toBeInTheDocument()
})

test('reports a library it could not read', async () => {
	vi.spyOn(console, 'error').mockImplementation(() => {})
	server.use(http.get('/api/media', () => HttpResponse.json({}, { status: 500 })))

	await openPicker()

	expect(await screen.findByRole('alert')).toHaveTextContent(/could not/i)
})

test.each([
	{ name: 'nothing narrows the list', props: {}, narrow: leaveAsIs, heading: 'No media items yet' },
	{ name: 'a search narrows the list', props: {}, narrow: searchHarbor, heading: 'No results found' },
	{ name: 'the block names kinds', props: { allowedTypes: ['image'] }, narrow: leaveAsIs, heading: 'No results found' },
])('says $heading when $name', async ({ props, narrow, heading }) => {
	listing([])

	await openPicker(props)
	await screen.findByRole('searchbox', { name: 'Search media…' })
	await narrow()

	expect(await screen.findByRole('heading', { name: heading })).toBeInTheDocument()
})

test('narrows the library to what the block accepts', () => {
	expect(allowedMimes(['image'])).toEqual(['image/'])
	expect(allowedMimes(['video'])).toEqual(['video/'])
	expect(allowedMimes(['image/jpeg'])).toEqual(['image/jpeg'])
	expect(allowedMimes(['text/vtt'])).toEqual(['text/vtt'])
	expect(allowedMimes(['image/jpeg', 'image/png'])).toEqual(['image/jpeg', 'image/png'])
	expect(allowedMimes(['image', 'video'])).toEqual(['image/', 'video/'])
	expect(allowedMimes(['image', 'image'])).toEqual(['image/'])
	expect(allowedMimes(['*'])).toEqual([])
	expect(allowedMimes(['image/*'])).toEqual(['image/'])
	expect(allowedMimes(['image/*', 'image'])).toEqual(['image/'])
	expect(allowedMimes([])).toEqual([])
	expect(allowedMimes(undefined)).toEqual([])
})

test('picks the selection a block value stands for', () => {
	expect(selectionFrom(undefined)).toEqual([])
	expect(selectionFrom(7)).toEqual(['7'])
	expect(selectionFrom([7, 9])).toEqual(['7', '9'])
	expect(selectionFrom([7, undefined as unknown as number])).toEqual(['7'])
})
