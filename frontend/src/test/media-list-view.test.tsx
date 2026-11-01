// SPDX-License-Identifier: Apache-2.0

import { http, HttpResponse, server } from '@gophenberg/frontend-sdk/testing'
import { setViewport } from '@gopherium/godmin/testing'
import { act, cleanup, screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { resetLocaleData, setLocaleData } from '@wordpress/i18n'
import { beforeAll, beforeEach, expect, onTestFinished, test, vi } from 'vitest'

import { mediaQueryKey } from '../media/api'
import { gate } from './gate'
import { leaveStale } from './reopen'
import { adminUser, renderAt, renderRoutedAt } from './render'
import { siteSettings } from './siteSettings'

beforeAll(async () => {
	await import('../media/MediaScreen')
})

const OWN_AUTHOR = '019fb000-0000-7000-8000-0000000000ff'

const FOREIGN_AUTHOR = '019fb000-0000-7000-8000-0000000000ee'

const HARBOR = {
	id: 7,
	type: 'image',
	file: '2026/08/harbor.jpg',
	title: 'Harbor at dawn',
	alt_text: 'Boats at sunrise',
	mime_type: 'image/jpeg',
	width: 1600,
	height: 1000,
	filesize: 254000,
	sizes: {},
	author_id: OWN_AUTHOR,
	author_name: 'Maria Perez',
	author_avatar: 'https://example.com/maria.png',
	created_at: '2026-08-10T10:00:00Z',
	updated_at: '2026-08-12T10:00:00Z',
}

const MANUAL = {
	...HARBOR,
	id: 8,
	type: 'file',
	file: '2026/08/manual.pdf',
	title: 'Manual',
	alt_text: '',
	mime_type: 'application/pdf',
	width: 0,
	height: 0,
	author_avatar: '',
}

/** An author, who may change only the files it uploaded, none of the ones this library holds. */
const AUTHOR = { ...adminUser, role: 'author', capabilities: [] }

/** The queries bound to one card of the grid. */
type Card = ReturnType<typeof within>

const asked: URLSearchParams[] = []

/**
 * Serves the given rows as every page of the library, recording what each request asked for.
 * @param items - The rows each page holds.
 * @param total - The number of matches the API reports.
 */
function serveRows(items: unknown[], total = items.length) {
	server.use(
		http.get('/api/media', ({ request }) => {
			asked.push(new URL(request.url).searchParams)
			return HttpResponse.json({ items, total, per_page: siteSettings.list_page_size })
		}),
	)
}

beforeEach(() => {
	asked.length = 0
	serveRows([HARBOR, MANUAL])
	server.use(
		http.get('/api/authors', () =>
			HttpResponse.json({
				items: [
					{ id: FOREIGN_AUTHOR, name: 'Foreign Author' },
					{ id: OWN_AUTHOR, name: 'Maria Perez' },
				],
			}),
		),
	)
})

/**
 * Returns the address of the library narrowed by the given filters.
 * @param filters - The filters the address holds.
 * @returns The address.
 */
function narrowedAt(filters: unknown[]): string {
	return `/media?filters=${encodeURIComponent(JSON.stringify(filters))}`
}

/**
 * Returns the parameters the latest request for the library carried.
 * @returns The parameters.
 */
function lastAsked(): Record<string, string> {
	return Object.fromEntries(asked.at(-1) ?? new URLSearchParams())
}

/**
 * Returns the card of the grid holding the given title.
 * @param title - The title the card shows.
 * @returns The queries bound to the card.
 */
async function cardOf(title: string) {
	const shown = await screen.findByText(title, { selector: '.dataviews-title-field *, .dataviews-title-field' })
	return within(shown.closest('.dataviews-view-grid__card') as HTMLElement)
}

test('asks for the search, sort, page and filters the address holds', async () => {
	const filters = [{ field: 'media_type', operator: 'is', value: 'image' }]
	renderAt(`${narrowedAt(filters)}&search=harbor&sort=title&order=asc&page=2`)

	await screen.findByText('Harbor at dawn')

	expect(Object.fromEntries(asked[0])).toEqual({
		search: 'harbor',
		orderby: 'title',
		order: 'asc',
		page: '2',
		per_page: '20',
		mime: 'image/',
	})
})

test('keeps the sort a reader picks in the address, so the list opens on it again', async () => {
	const { router } = renderRoutedAt('/media')
	await screen.findByText('Harbor at dawn')
	await userEvent.click(screen.getByRole('button', { name: 'View options' }))

	await userEvent.selectOptions(await screen.findByRole('combobox', { name: 'Sort by' }), 'title')
	await waitFor(() => expect(router.state.location.search).toMatchObject({ sort: 'title' }))
	const kept = router.state.location.searchStr
	cleanup()
	asked.length = 0
	renderAt(`/media${kept}`)

	await screen.findByText('Harbor at dawn')
	expect(asked[0].get('orderby')).toBe('title')
})

test('draws a grid of cards from 640px, with no Layout control', async () => {
	setViewport({ width: 640 })
	renderAt('/media')

	await screen.findByText('Harbor at dawn')

	expect(document.querySelector('.dataviews-view-grid')).not.toBeNull()
	expect(screen.queryByRole('button', { name: /layout/i })).not.toBeInTheDocument()
})

test('draws a plain list below 640px, with no Layout control', async () => {
	setViewport({ width: 639 })
	renderAt('/media')

	await screen.findByText('Harbor at dawn')

	expect(document.querySelector('.dataviews-view-list')).not.toBeNull()
	expect(document.querySelector('.dataviews-view-grid')).toBeNull()
	expect(screen.queryByRole('button', { name: /layout/i })).not.toBeInTheDocument()
})

test('shows on a card the title and the author with the avatar, and nothing else under it', async () => {
	renderAt('/media')

	const card = await cardOf('Harbor at dawn')

	const author = card.getByText('Maria Perez').closest('.gophenberg-author') as HTMLElement
	expect(author.querySelector('.gophenberg-author__avatar')).not.toBeNull()
	expect(card.queryByText('1600 × 1000')).not.toBeInTheDocument()
	expect(card.queryByText('harbor.jpg')).not.toBeInTheDocument()
})

test('shows the author beside the title on a phone row', async () => {
	setViewport({ width: 639 })
	renderAt('/media')

	await screen.findByText('Harbor at dawn')

	const rows = within(document.querySelector('.dataviews-view-list') as HTMLElement)
	expect(rows.getAllByText('Maria Perez')).toHaveLength(2)
})

test('sorts by Title, Date added, Date modified and Author alone', async () => {
	renderAt('/media')
	await screen.findByText('Harbor at dawn')

	await userEvent.click(screen.getByRole('button', { name: 'View options' }))

	const sorts = within(await screen.findByRole('combobox', { name: 'Sort by' })).getAllByRole('option')
	expect(sorts.map((option) => option.textContent)).toEqual(['Title', 'Date added', 'Date modified', 'Author'])
})

test.each([
	{ field: 'title', from: 'date' },
	{ field: 'date', from: 'title' },
	{ field: 'modified', from: 'title' },
	{ field: 'author', from: 'title' },
])('asks the server to sort by $field, then in the order the reader picks', async ({ field, from }) => {
	renderAt(`/media?sort=${from}&order=asc`)
	await screen.findByText('Harbor at dawn')
	await userEvent.click(screen.getByRole('button', { name: 'View options' }))

	await userEvent.selectOptions(await screen.findByRole('combobox', { name: 'Sort by' }), field)
	await waitFor(() => expect(lastAsked()).toMatchObject({ orderby: field, order: 'asc' }))
	await userEvent.click(screen.getByRole('radio', { name: 'Sort descending' }))

	await waitFor(() => expect(lastAsked()).toMatchObject({ orderby: field, order: 'desc' }))
	expect(asked[0].get('orderby')).toBe(from)
})

test('opens on Date added, newest first', async () => {
	renderAt('/media')

	await screen.findByText('Harbor at dawn')

	expect(lastAsked()).toMatchObject({ orderby: 'date', order: 'desc' })
})

test('offers the attachment fields WordPress shows under View options, and no Type, which only filters', async () => {
	renderAt('/media')
	await screen.findByText('Harbor at dawn')

	await userEvent.click(screen.getByRole('button', { name: 'View options' }))

	const properties = await screen.findByText('Properties')
	const listed = [...(properties.parentElement as HTMLElement).querySelectorAll('[role="listitem"]')]
	expect(listed.map((property) => property.textContent)).toEqual([
		'Title',
		'Thumbnail',
		'Alt text',
		'Author',
		'Caption',
		'Date added',
		'Date modified',
		'Description',
		'Dimensions',
		'File name',
		'File size',
		'File type',
	])
})

test('offers Author, Date added, Date modified and Type as filters, none shown before a reader adds it', async () => {
	renderAt('/media')
	await screen.findByText('Harbor at dawn')

	expect(screen.queryByRole('button', { name: /^Type/ })).not.toBeInTheDocument()
	await userEvent.click(screen.getByRole('button', { name: 'Add filter' }))

	const offered = await screen.findAllByRole('menuitem')
	expect(offered.map((item) => item.textContent)).toEqual(['Author', 'Date added', 'Date modified', 'Type'])
})

test('writes the type a reader picks into the address under the id WordPress filters media by', async () => {
	const { router } = renderRoutedAt('/media')
	await screen.findByText('Harbor at dawn')
	await userEvent.click(screen.getByRole('button', { name: 'Add filter' }))
	await userEvent.click(await screen.findByRole('menuitem', { name: 'Type' }))

	await userEvent.click(await screen.findByRole('option', { name: 'Documents' }))

	await waitFor(() =>
		expect(router.state.location.search).toMatchObject({
			filters: [{ field: 'media_type', operator: 'is', value: 'document' }],
		}),
	)
	await waitFor(() => expect(lastAsked()).toMatchObject({ mime: 'application/pdf' }))
})

test.each([
	{ value: 'image', mime: 'image/' },
	{ value: 'audio', mime: 'audio/' },
	{ value: 'video', mime: 'video/' },
	{ value: 'document', mime: 'application/pdf' },
	{ value: 'archive', mime: 'application/zip' },
])('asks for the $value kind as the content type prefix $mime, never as a type', async ({ value, mime }) => {
	renderAt(narrowedAt([{ field: 'media_type', operator: 'is', value }]))

	await screen.findByText('Harbor at dawn')

	expect(asked[0].get('mime')).toBe(mime)
	expect(asked[0].has('type')).toBe(false)
})

test.each([
	{ operator: 'isAny', key: 'author' },
	{ operator: 'isNone', key: 'author_exclude' },
])('asks for the authors of an author filter under $key', async ({ operator, key }) => {
	renderAt(narrowedAt([{ field: 'author', operator, value: [OWN_AUTHOR, FOREIGN_AUTHOR] }]))

	await screen.findByText('Harbor at dawn')

	expect(asked[0].get(key)).toBe(`${OWN_AUTHOR},${FOREIGN_AUTHOR}`)
})

test.each([
	{ field: 'date', operator: 'before', key: 'before' },
	{ field: 'date', operator: 'after', key: 'after' },
	{ field: 'modified', operator: 'before', key: 'modified_before' },
	{ field: 'modified', operator: 'after', key: 'modified_after' },
])('asks for a $field $operator filter under $key', async ({ field, operator, key }) => {
	renderAt(narrowedAt([{ field, operator, value: '2026-08-16T12:00:00.000Z' }]))

	await screen.findByText('Harbor at dawn')

	expect(asked[0].get(key)).toBe('2026-08-16T12:00:00.000Z')
})

test.each([
	{ shape: 'a sort the list never offers', address: '/media?sort=filesize&order=asc' },
	{
		shape: 'a filter on a field the list never filters by',
		address: narrowedAt([{ field: 'title', operator: 'is', value: 'Harbor' }]),
	},
	{
		shape: 'a kind outside the groups',
		address: narrowedAt([{ field: 'media_type', operator: 'is', value: 'spreadsheet' }]),
	},
	{
		shape: 'a date that is no moment',
		address: narrowedAt([{ field: 'modified', operator: 'after', value: 'someday' }]),
	},
	{
		shape: 'an author that is no account',
		address: narrowedAt([{ field: 'author', operator: 'isAny', value: ['nope'] }]),
	},
])('drops $shape and opens on Date added, newest first', async ({ address }) => {
	renderAt(`${address}&search=harbor`)

	await screen.findByText('Harbor at dawn')

	expect(Object.fromEntries(asked[0])).toEqual({ search: 'harbor', orderby: 'date', order: 'desc', per_page: '20' })
})

test('offers as page sizes the ones the settings name', async () => {
	renderAt('/media')
	await screen.findByText('Harbor at dawn')

	await userEvent.click(screen.getByRole('button', { name: 'View options' }))

	const sizes = within(await screen.findByRole('radiogroup', { name: 'Items per page' }))
	expect(sizes.getAllByRole('radio').map((size) => size.textContent)).toEqual(['10', '20', '50', '100'])
	expect(sizes.getByRole('radio', { name: '20' })).toBeChecked()
})

test('leaves the page size to the server and offers no sizes when the settings could not be read', async () => {
	server.use(http.get('/api/settings', () => HttpResponse.json({}, { status: 500 })))
	renderAt('/media?perPage=50')
	await screen.findByText('Harbor at dawn')

	await userEvent.click(screen.getByRole('button', { name: 'View options' }))

	expect(await screen.findByRole('combobox', { name: 'Sort by' })).toBeInTheDocument()
	expect(screen.queryByRole('radiogroup', { name: 'Items per page' })).not.toBeInTheDocument()
	expect(asked.every((search) => !search.has('per_page'))).toBe(true)
})

test('asks the page size the settings open on for an address naming one they do not offer', async () => {
	renderAt('/media?perPage=7')

	await screen.findByText('Harbor at dawn')

	expect(asked.map((search) => search.get('per_page'))).toEqual(['20'])
})

test('asks for a page size the reader picks among the offered ones', async () => {
	renderAt('/media')
	await screen.findByText('Harbor at dawn')
	await userEvent.click(screen.getByRole('button', { name: 'View options' }))

	const sizes = within(await screen.findByRole('radiogroup', { name: 'Items per page' }))
	await userEvent.click(sizes.getByRole('radio', { name: '50' }))

	await waitFor(() => expect(lastAsked()).toMatchObject({ per_page: '50' }))
})

test('searches media through a box reading Search media…', async () => {
	renderAt('/media')
	await screen.findByText('Harbor at dawn')

	await userEvent.type(screen.getByRole('searchbox', { name: 'Search media…' }), 'harbor')

	await waitFor(() => expect(lastAsked()).toMatchObject({ search: 'harbor' }))
})

test('writes only the search into the address while the list keeps the order it opens on', async () => {
	const { router } = renderRoutedAt('/media')
	await screen.findByText('Harbor at dawn')

	await userEvent.type(screen.getByRole('searchbox', { name: 'Search media…' }), 'harbor')

	await waitFor(() => expect(router.state.location.search).toEqual({ search: 'harbor' }))
})

test('names the search box after the library in the words the catalogue gives its admin section', async () => {
	setLocaleData(
		{ '': { 'plural-forms': 'nplurals=2; plural=(n != 1);' }, 'admin section\u0004Media': ['Files'] },
		'gophenberg',
	)
	onTestFinished(() => resetLocaleData({}, 'gophenberg'))
	renderAt('/media')

	await screen.findByText('Harbor at dawn')

	expect(screen.getByRole('searchbox', { name: 'Search files…' })).toBeInTheDocument()
})

test('keeps the cards of a page and its page count while the next one loads', async () => {
	serveRows([HARBOR], 45)
	renderAt('/media')
	await screen.findByText('Harbor at dawn')
	const next = gate()
	let asking = false
	server.use(
		http.get('/api/media', async () => {
			asking = true
			await next.held
			return HttpResponse.json({ items: [MANUAL], total: 45, per_page: 20 })
		}),
	)

	await userEvent.click(screen.getByRole('button', { name: 'Next page' }))
	await waitFor(() => expect(asking).toBe(true))

	expect(screen.getByText('Harbor at dawn')).toBeInTheDocument()
	expect(document.querySelector('.dataviews-view-grid')).toHaveAttribute('aria-busy', 'true')
	const pages = within(screen.getByRole('combobox', { name: 'Current page' })).getAllByRole('option')
	expect(pages.map((page) => page.textContent)).toEqual(['1', '2', '3'])
	next.release()
	expect(await screen.findByText('Manual')).toBeInTheDocument()
})

test('shows the cards it holds at once when the library opens again, while it reads them anew', async () => {
	const { client, router } = renderRoutedAt('/media')
	await screen.findByText('Harbor at dawn')
	await leaveStale(router, client, mediaQueryKey)
	const again = gate()
	let reading = false
	server.use(
		http.get('/api/media', async () => {
			reading = true
			await again.held
			return HttpResponse.json({
				items: [HARBOR, { ...MANUAL, title: 'Field manual' }],
				total: 2,
				per_page: siteSettings.list_page_size,
			})
		}),
	)

	await act(async () => {
		await router.navigate({ to: '/media' })
	})

	expect(await screen.findByText('Harbor at dawn')).toBeInTheDocument()
	expect(document.querySelector('.dataviews-view-grid')).toHaveAttribute('aria-busy', 'false')
	await waitFor(() => expect(reading).toBe(true))
	again.release()
	expect(await screen.findByText('Field manual')).toBeInTheDocument()
	expect(screen.queryByText('Manual')).not.toBeInTheDocument()
})

test('opens Edit from a tap on a phone row of the page on show while the next page loads', async () => {
	setViewport({ width: 639 })
	serveRows([HARBOR], 45)
	renderAt('/media')
	await screen.findByText('Harbor at dawn')
	const next = gate()
	server.use(
		http.get('/api/media', async () => {
			await next.held
			return HttpResponse.json({ items: [MANUAL], total: 45, per_page: 20 })
		}),
	)
	await userEvent.click(screen.getByRole('button', { name: 'Next page' }))

	await userEvent.click(await screen.findByRole('button', { name: /^Harbor at dawn/ }))

	expect(await screen.findByRole('dialog', { name: 'Edit media' })).toBeInTheDocument()
	next.release()
})

test('says the library holds nothing yet, and offers the add button, when nothing narrows it', async () => {
	serveRows([])
	renderAt('/media')

	const empty = (await screen.findByText('No media items yet')).closest('.godmin-empty') as HTMLElement

	expect(within(empty).getByRole('button', { name: 'Add Media File' })).toBeInTheDocument()
})

test.each([
	{ part: 'title', click: (card: Card) => card.getByText('Harbor at dawn') },
	{ part: 'picture', click: (card: Card) => card.getByAltText('Boats at sunrise') },
])('opens Edit from the $part of a file the reader may change', async ({ click }) => {
	renderAt('/media')

	await userEvent.click(click(await cardOf('Harbor at dawn')))

	expect(await screen.findByRole('dialog', { name: 'Edit media' })).toBeInTheDocument()
})

test('draws the title of a file another account uploaded as plain text for an author, opening nothing', async () => {
	renderAt('/media', AUTHOR)
	const card = await cardOf('Harbor at dawn')

	await userEvent.click(card.getByText('Harbor at dawn'))
	await userEvent.click(card.getByAltText('Boats at sunrise'))

	expect(card.queryByRole('button', { name: 'Harbor at dawn' })).not.toBeInTheDocument()
	expect(screen.queryByRole('dialog')).not.toBeInTheDocument()
})

test('opens Edit from a tap on a phone row of a file the reader may change', async () => {
	setViewport({ width: 639 })
	renderAt('/media')

	await userEvent.click(await screen.findByRole('button', { name: /^Harbor at dawn/ }))

	expect(await screen.findByRole('dialog', { name: 'Edit media' })).toBeInTheDocument()
})

test('opens a file another account uploaded in a new tab from a tap on its phone row, as View does', async () => {
	setViewport({ width: 639 })
	const opened = vi.spyOn(window, 'open').mockImplementation(() => null)
	renderAt('/media', AUTHOR)

	await userEvent.click(await screen.findByRole('button', { name: /^Manual/ }))

	await act(() => new Promise((resolve) => setTimeout(resolve, 0)))
	expect(opened).toHaveBeenCalledExactlyOnceWith('/media/2026/08/manual.pdf', '_blank')
	expect(screen.queryByRole('dialog')).not.toBeInTheDocument()
})

test.each([
	{ how: 'a search', address: '/media?search=lighthouse' },
	{ how: 'an author filter', address: narrowedAt([{ field: 'author', operator: 'isAny', value: [FOREIGN_AUTHOR] }]) },
	{
		how: 'a date modified filter',
		address: narrowedAt([{ field: 'modified', operator: 'before', value: '2026-01-01T00:00:00.000Z' }]),
	},
	{ how: 'a kind filter', address: narrowedAt([{ field: 'media_type', operator: 'is', value: 'archive' }]) },
])('says no results are found when $how narrows the library to nothing', async ({ address }) => {
	serveRows([])
	renderAt(address)

	const empty = (await screen.findByText('No results found')).closest('.godmin-empty') as HTMLElement

	expect(within(empty).queryByRole('button')).not.toBeInTheDocument()
})
