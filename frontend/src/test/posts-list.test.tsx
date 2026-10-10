// SPDX-License-Identifier: Apache-2.0

import { http, HttpResponse, server } from '@gophenberg/frontend-sdk/testing'
import { badgeClasses, setViewport } from '@gopherium/godmin/testing'
import { act, screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { resetLocaleData, setLocaleData } from '@wordpress/i18n'
import { beforeAll, beforeEach, expect, onTestFinished, test } from 'vitest'

import '../index.css'
import { catalogFor } from '../i18n/catalog'
import { gate } from './gate'
import { storedPostWithId } from './postFixture'
import { renderAt, renderRoutedAt } from './render'
import { siteSettings } from './siteSettings'
import { warmPostsScreen } from './warm'

warmPostsScreen()

beforeAll(async () => {
	await import('../content/EditorScreen')
}, 120000)

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

const DRAFT = {
	...PUBLISHED,
	id: '019fb000-0000-7000-8000-000000000002',
	slug: 'notes',
	title: 'Notes on the Next Release',
	status: 'draft',
	published_at: null,
	updated_at: '2026-07-28T09:00:00Z',
}

const asked: URLSearchParams[] = []

/**
 * Serves the given rows as every page of the listing, recording what each request asked for.
 * @param items - The rows each page holds.
 * @param total - The number of matches the API reports.
 * @param perPage - The page size the API says it used, none to leave it out.
 */
function serveRows(items: unknown[], total: number, perPage?: number) {
	server.use(
		http.get('/api/content', ({ request }) => {
			asked.push(new URL(request.url).searchParams)
			return HttpResponse.json({ items, total, per_page: perPage })
		}),
	)
}

beforeEach(() => {
	asked.length = 0
	serveRows([PUBLISHED, DRAFT], 2, 20)
	server.use(http.get('/api/content/:id', ({ params }) => HttpResponse.json(storedPostWithId(String(params.id)))))
})

const PAGE_TYPE = {
	key: 'page',
	singular_label: 'Page',
	plural_label: 'Pages',
	description: '',
	route_word: 'pages',
	hierarchical: true,
	revisions: true,
	revision_cap: 100,
	page_kind: 'single',
	default: false,
	active: true,
	fields: [],
}

const POST_TYPE = {
	...PAGE_TYPE,
	key: 'post',
	singular_label: 'Post',
	plural_label: 'Posts',
	route_word: '',
	hierarchical: false,
	default: true,
}

test('opens on the sort, search and page the address holds', async () => {
	serveRows([PUBLISHED], 45, 20)
	renderAt('/content/post?search=notes&page=2&sort=title&order=asc')

	await screen.findByText('Welcome to Gophenberg')

	expect(asked[0].get('search')).toBe('notes')
	expect(asked[0].get('page')).toBe('2')
	expect(asked[0].get('orderby')).toBe('title')
	expect(asked[0].get('order')).toBe('asc')
})

test('writes the search a reader types into the address', async () => {
	const { router } = renderRoutedAt('/content/post')
	await screen.findByText('Welcome to Gophenberg')

	await userEvent.type(screen.getByRole('searchbox'), 'notes')

	await waitFor(() => expect(router.state.location.search).toMatchObject({ search: 'notes' }))
})

test('asks the page size the settings name', async () => {
	server.use(http.get('/api/settings', () => HttpResponse.json({ ...siteSettings, list_page_size: 50 })))
	renderAt('/content/post')

	await screen.findByText('Welcome to Gophenberg')

	expect(asked[0].get('per_page')).toBe('50')
})

test('offers as page sizes the ones the settings name', async () => {
	server.use(http.get('/api/settings', () => HttpResponse.json({ ...siteSettings, list_page_sizes: [5, 15] })))
	renderAt('/content/post')
	await screen.findByText('Welcome to Gophenberg')

	await userEvent.click(screen.getByRole('button', { name: 'View options' }))

	const sizes = within(await screen.findByRole('radiogroup', { name: 'Items per page' }))
	expect(sizes.getAllByRole('radio').map((size) => size.textContent)).toEqual(['5', '15'])
})

test('pages by the size the server served when the settings could not be read', async () => {
	server.use(http.get('/api/settings', () => HttpResponse.json({}, { status: 500 })))
	serveRows([PUBLISHED, DRAFT], 3, 2)
	renderAt('/content/post?perPage=10')
	await screen.findByText('Welcome to Gophenberg')

	await userEvent.click(screen.getByRole('button', { name: 'Next page' }))

	await waitFor(() => expect(asked.at(-1)?.get('page')).toBe('2'))
	expect(asked.every((search) => !search.has('per_page'))).toBe(true)
})

test('asks the page size the address names while the settings offer it', async () => {
	renderAt('/content/post?perPage=50')

	await screen.findByText('Welcome to Gophenberg')

	expect(asked.map((search) => search.get('per_page'))).toEqual(['50'])
})

test('asks the page size the settings name for an address naming one they do not offer', async () => {
	renderAt('/content/post?perPage=7')
	await screen.findByText('Welcome to Gophenberg')

	await userEvent.click(screen.getByRole('button', { name: 'View options' }))

	const sizes = within(await screen.findByRole('radiogroup', { name: 'Items per page' }))
	expect(sizes.getByRole('radio', { name: '20' })).toBeChecked()
	expect(asked.map((search) => search.get('per_page'))).toEqual(['20'])
})

test('offers no page sizes when the settings could not be read', async () => {
	server.use(http.get('/api/settings', () => HttpResponse.json({}, { status: 500 })))
	renderAt('/content/post')
	await screen.findByText('Welcome to Gophenberg')

	await userEvent.click(screen.getByRole('button', { name: 'View options' }))

	expect(await screen.findByRole('combobox', { name: 'Sort by' })).toBeInTheDocument()
	expect(screen.queryByRole('radiogroup', { name: 'Items per page' })).not.toBeInTheDocument()
})

test('keeps the rows of a page while the next one loads', async () => {
	serveRows([PUBLISHED], 45, 20)
	renderAt('/content/post')
	await screen.findByText('Welcome to Gophenberg')
	const next = gate()
	server.use(
		http.get('/api/content', async () => {
			await next.held
			return HttpResponse.json({ items: [DRAFT], total: 45, per_page: 20 })
		}),
	)

	await userEvent.click(screen.getByRole('button', { name: 'Next page' }))

	expect(screen.getByText('Welcome to Gophenberg')).toBeInTheDocument()
	next.release()
	expect(await screen.findByText('Notes on the Next Release')).toBeInTheDocument()
})

test('never shows the rows of one type while the list of another loads', async () => {
	server.use(http.get('/api/types', () => HttpResponse.json({ items: [POST_TYPE, PAGE_TYPE] })))
	const { router } = renderRoutedAt('/content/post')
	await screen.findByText('Welcome to Gophenberg')
	const pages = gate()
	server.use(
		http.get('/api/content', async () => {
			await pages.held
			return HttpResponse.json({ items: [{ ...DRAFT, type: 'page', title: 'About Us' }], total: 1, per_page: 20 })
		}),
	)

	await act(async () => {
		await router.navigate({ to: '/content/$typeKey', params: { typeKey: 'page' } })
	})

	expect(await screen.findByRole('heading', { level: 1, name: 'Pages' })).toBeInTheDocument()
	expect(screen.queryByText('Welcome to Gophenberg')).not.toBeInTheDocument()
	pages.release()
	expect(await screen.findByText('About Us')).toBeInTheDocument()
})

test('opens the editor from a tap on a row on a phone', async () => {
	setViewport({ matches: true })
	const { router } = renderRoutedAt('/content/post')

	await userEvent.click(await screen.findByRole('button', { name: /^Notes on the Next Release/ }))

	await waitFor(() => expect(router.state.location.pathname).toBe(`/content/post/${DRAFT.id}/edit`))
})

test('offers no tick boxes on a phone', async () => {
	setViewport({ matches: true })
	renderAt('/content/post')

	await screen.findByText('Welcome to Gophenberg')

	expect(screen.queryByRole('checkbox')).not.toBeInTheDocument()
})

test('keeps the list region out of the tab order while a run can hand it the focus', async () => {
	renderAt('/content/post')

	await screen.findByText('Welcome to Gophenberg')

	expect(screen.getByRole('region', { name: 'Posts' })).toHaveAttribute('tabindex', '-1')
})

test('asks no sort the list does not offer, whatever the address holds', async () => {
	renderAt('/content/post?sort=nope&search=notes')

	await screen.findByText('Welcome to Gophenberg')

	expect(asked[0].get('search')).toBe('notes')
	expect(asked[0].get('orderby')).toBe('date')
})

test('asks no filter naming a field the list does not filter by, whatever the address holds', async () => {
	const filters = encodeURIComponent(
		JSON.stringify([
			{ field: 'field.nope', operator: 'is', value: 'x' },
			{ field: 'title', operator: 'is', value: 'x' },
		]),
	)
	renderAt(`/content/post?filters=${filters}&search=notes`)

	await screen.findByText('Welcome to Gophenberg')

	expect(asked[0].get('search')).toBe('notes')
	expect([...asked[0].keys()].filter((key) => key.startsWith('field'))).toEqual([])
})

const BEFORE_2020 = encodeURIComponent('[{"field":"date","operator":"before","value":"2020-01-01"}]')

const BY_ONE_AUTHOR = encodeURIComponent(`[{"field":"author","operator":"isAny","value":["${PUBLISHED.author_id}"]}]`)

test.each([
	{ narrowing: 'a search', address: '/content/post?search=nothing' },
	{ narrowing: 'a filter', address: `/content/post?filters=${BEFORE_2020}` },
	{ narrowing: 'an author picked', address: `/content/post?filters=${BY_ONE_AUTHOR}` },
	{ narrowing: 'a tab other than All', address: '/content/post?status=draft' },
])('says no items are found when $narrowing narrows the list to none', async ({ address }) => {
	serveRows([], 0, 20)
	renderAt(address)

	expect(await screen.findByText('No items found.')).toBeInTheDocument()
	expect(screen.queryByText('Add one with Add New.')).not.toBeInTheDocument()
})

test('says no items are found in the words of WordPress es_ES for a Spanish reader', async () => {
	setLocaleData(await catalogFor('es-ES'), 'gophenberg')
	onTestFinished(() => resetLocaleData({}, 'gophenberg'))
	serveRows([], 0, 20)
	renderAt('/content/post?search=nothing')

	expect(await screen.findByText('No se han encontrado elementos.')).toBeInTheDocument()
})

test('says no items are there yet, and how to add one, when nothing narrows the empty list', async () => {
	serveRows([], 0, 20)
	renderAt('/content/post')

	expect(await screen.findByText('No items yet.')).toBeInTheDocument()
	expect(screen.getByText('Add one with Add New.')).toBeInTheDocument()
})

test.each([
	{ chip: 'a date chip is open with no moment picked', filters: '[{"field":"date","operator":"before"}]' },
	{
		chip: 'an author chip is open with every author unticked',
		filters: '[{"field":"author","operator":"isAny","value":[]}]',
	},
])('says no items are there yet when $chip', async ({ filters }) => {
	serveRows([], 0, 20)
	renderAt(`/content/post?filters=${encodeURIComponent(filters)}`)

	expect(await screen.findByText('No items yet.')).toBeInTheDocument()
})

test('names the search after the type it lists, its label in lower case', async () => {
	renderAt('/content/post')

	expect(await screen.findByRole('searchbox', { name: 'Search posts…' })).toBeInTheDocument()
})

test('lists every post the API returned', async () => {
	renderAt('/content/post')

	expect(await screen.findByText('Welcome to Gophenberg')).toBeInTheDocument()
	expect(screen.getByText('Notes on the Next Release')).toBeInTheDocument()
})

test('links each title to its editor', async () => {
	renderAt('/content/post')

	const title = await screen.findByRole('link', { name: /Welcome to Gophenberg/ })

	expect(title).toHaveAttribute('href', `/admin/content/post/${PUBLISHED.id}/edit`)
})

/**
 * Returns the row of the table listing the title.
 * @param title - The title the row lists, or a pattern its link name matches.
 * @returns The row, to query within.
 */
async function rowOf(title: string | RegExp) {
	return within((await screen.findByRole('link', { name: title })).closest('tr') as HTMLElement)
}

test('draws no status badge beside a title', async () => {
	renderAt('/content/post')

	const title = (await screen.findByRole('link', { name: 'Notes on the Next Release' })).closest('td') as HTMLElement

	expect(within(title).queryByText('Draft')).not.toBeInTheDocument()
})

test('shows a title as it is stored, entities and all', async () => {
	serveRows([{ ...PUBLISHED, title: 'Fish &amp; Chips' }], 1, 20)
	renderAt('/content/post')

	expect(await screen.findByRole('link', { name: 'Fish &amp; Chips' })).toBeInTheDocument()
})

test('shows the author of each post', async () => {
	renderAt('/content/post')

	expect(await screen.findAllByText('Maria Perez')).toHaveLength(2)
})

test('draws the initials of the author at 16px in a 24px box before the name', async () => {
	renderAt('/content/post')

	const author = (await rowOf('Welcome to Gophenberg')).getByText('Maria Perez')
	const box = author.previousElementSibling as HTMLElement

	expect(getComputedStyle(box).width).toBe('24px')
	expect(box.firstElementChild).toHaveTextContent('M')
	expect(box.firstElementChild).toHaveStyle({ inlineSize: '16px' })
})

test('shows the excerpt under the title', async () => {
	serveRows([{ ...PUBLISHED, excerpt: 'A short tour of the editor.' }], 1, 20)
	renderAt('/content/post')

	expect(await screen.findByText('A short tour of the editor.')).toBeInTheDocument()
})

test('dates a published post by its publication, with the time', async () => {
	renderAt('/content/post')

	expect((await rowOf('Welcome to Gophenberg')).getByText('Published: Jul 20, 2026 10:00 am')).toBeInTheDocument()
})

test.each(['draft', 'pending', 'private'])('dates a %s post by its list date as modified', async (status) => {
	serveRows([{ ...DRAFT, status }], 1, 20)
	renderAt('/content/post')

	expect((await rowOf('Notes on the Next Release')).getByText('Modified: Jul 28, 2026 9:00 am')).toBeInTheDocument()
})

test('dates a scheduled post by the moment it goes out', async () => {
	serveRows([{ ...DRAFT, status: 'scheduled', published_at: '2026-09-01T08:30:00Z' }], 1, 20)
	renderAt('/content/post')

	expect((await rowOf('Notes on the Next Release')).getByText('Scheduled: Sep 1, 2026 8:30 am')).toBeInTheDocument()
})

test('dates a post in the trash with no caption', async () => {
	serveRows([{ ...DRAFT, status: 'trash' }], 1, 20)
	renderAt('/content/post?status=trash')

	expect((await rowOf('Notes on the Next Release')).getByText('Jul 28, 2026 9:00 am')).toBeInTheDocument()
})

test.each([
	{ status: 'draft', label: 'Draft', intent: 'low' },
	{ status: 'scheduled', label: 'Scheduled', intent: 'informational' },
	{ status: 'pending', label: 'Pending Review', intent: 'informational' },
	{ status: 'private', label: 'Private', intent: 'draft' },
	{ status: 'published', label: 'Published', intent: 'stable' },
	{ status: 'trash', label: 'Trash', intent: 'none' },
] as const)('draws the $status status as a $label badge in the $intent colour, kept on one line', async ({
	status,
	label,
	intent,
}) => {
	serveRows([{ ...DRAFT, status }], 1, 20)
	renderAt('/content/post')

	const badge = (await rowOf('Notes on the Next Release')).getByText(label)

	expect([...badge.classList]).toEqual([...badgeClasses(intent), 'gophenberg-status'])
})

test('shows the status beside the date on a phone', async () => {
	setViewport({ matches: true })
	renderAt('/content/post')

	await screen.findByRole('button', { name: 'Notes on the Next Release' })

	const list = within(screen.getByRole('grid'))
	expect(list.getByText('Modified: Jul 28, 2026 9:00 am')).toBeInTheDocument()
	expect(list.getByText('Draft')).toBeInTheDocument()
	expect(list.queryByText('Maria Perez')).not.toBeInTheDocument()
})

test('draws the levels of a hierarchical type, a dash for each level under the top', async () => {
	server.use(http.get('/api/types', () => HttpResponse.json({ items: [POST_TYPE, PAGE_TYPE] })))
	serveRows(
		[
			{ ...PUBLISHED, id: 'p1', type: 'page', title: 'About', path: 'pages/about' },
			{ ...PUBLISHED, id: 'p2', type: 'page', title: 'Team', path: 'pages/about/team', parent_id: 'p1' },
		],
		2,
		20,
	)
	renderAt('/content/page')

	expect((await rowOf('About')).queryByText(/—/)).not.toBeInTheDocument()
	expect((await rowOf(/Team$/)).getByText(/—/)).toBeInTheDocument()
	expect(asked[0].get('orderby_hierarchy')).toBe('true')
})

/** The pages of a tree, About at the top and Team under it. */
const PAGES = [
	{ ...PUBLISHED, id: 'p1', type: 'page', title: 'About', path: 'pages/about' },
	{ ...PUBLISHED, id: 'p2', type: 'page', title: 'Team', path: 'pages/about/team', parent_id: 'p1' },
]

test.each([
	{ sorted: 'by title', orderBy: 'title', order: 'asc' },
	{ sorted: 'by title, Z to A', orderBy: 'title', order: 'desc' },
	{ sorted: 'oldest first', orderBy: 'date', order: 'asc' },
])('lists a nesting type flat while the address sorts it $sorted, as on a reload', async ({ orderBy, order }) => {
	server.use(http.get('/api/types', () => HttpResponse.json({ items: [POST_TYPE, PAGE_TYPE] })))
	serveRows(PAGES, 2, 20)
	renderAt(`/content/page?sort=${orderBy}&order=${order}`)

	expect((await rowOf('Team')).queryByText(/—/)).not.toBeInTheDocument()
	expect(asked[0].get('orderby')).toBe(orderBy)
	expect(asked[0].get('order')).toBe(order)
	expect(asked[0].has('orderby_hierarchy')).toBe(false)
})

test('draws the levels again once a reader sorts a nesting type back to its opening order', async () => {
	server.use(http.get('/api/types', () => HttpResponse.json({ items: [POST_TYPE, PAGE_TYPE] })))
	serveRows(PAGES, 2, 20)
	renderAt('/content/page')
	await rowOf(/Team$/)

	await userEvent.click(screen.getByRole('button', { name: 'Title' }))
	await userEvent.click(await screen.findByRole('menuitemradio', { name: 'Sort ascending' }))
	await waitFor(() => expect(asked.at(-1)?.get('orderby')).toBe('title'))
	await userEvent.click(screen.getByRole('button', { name: 'Date' }))
	await userEvent.click(await screen.findByRole('menuitemradio', { name: 'Sort descending' }))

	await waitFor(() => expect(screen.getByRole('button', { name: 'Date' })).toHaveTextContent('↓'))
	expect((await rowOf(/Team$/)).getByText(/—/)).toBeInTheDocument()
	expect(asked.filter((search) => search.get('orderby') === 'date' && !search.has('orderby_hierarchy'))).toEqual([])
})

test('asks no tree order of a type that does not nest', async () => {
	renderAt('/content/post')

	await screen.findByText('Welcome to Gophenberg')

	expect(asked[0].has('orderby_hierarchy')).toBe(false)
})

test('reports a listing that could not be read', async () => {
	server.use(http.get('/api/content', () => HttpResponse.json({}, { status: 500 })))
	renderAt('/content/post')

	expect(await screen.findByRole('alert')).toHaveTextContent(/could not/i)
})

test('names a post that has no title yet', async () => {
	server.use(
		http.get('/api/content', () =>
			HttpResponse.json({ items: [{ ...DRAFT, title: '' }], total: 1 }),
		),
	)
	renderAt('/content/post')

	expect(await screen.findByRole('link', { name: '(no title)' })).toBeInTheDocument()
})

test('dates nothing on a post that carries no timestamps', async () => {
	serveRows([{ ...DRAFT, published_at: null, updated_at: undefined }], 1, 20)
	renderAt('/content/post')

	await screen.findByRole('link', { name: 'Notes on the Next Release' })

	expect(screen.queryByText(/Modified/)).not.toBeInTheDocument()
})
