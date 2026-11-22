// SPDX-License-Identifier: Apache-2.0

import type { Action, View } from '@gophenberg/frontend-sdk/dataviews'
import { listSearch, useListView } from '@gopherium/godmin/router'
import { badgeClasses, renderAdmin, setViewport } from '@gopherium/godmin/testing'
import {
	Outlet,
	RouterProvider,
	createMemoryHistory,
	createRootRoute,
	createRoute,
	createRouter,
} from '@tanstack/react-router'
import { render, screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { plugins as pluginsGlyph } from '@wordpress/icons'
import { expect, test, vi } from 'vitest'

import { PluginsPage, pluginsListDefaults } from '..'
import type { InstalledPlugin, PluginsPageProps, PluginsWords } from '..'

/** The words the app hands the page, as the English catalogue reads them. */
const WORDS: PluginsWords = {
	title: 'Plugins',
	subtitle: 'Every plugin this site holds, and whether it runs.',
	searchLabel: 'Search plugins…',
	loadFailed: 'The plugins could not be loaded.',
	noPlugins: 'No plugins yet',
	emptyTabs: { active: 'No active plugins', paused: 'No paused plugins' },
	labels: {
		name: 'Plugin',
		paused_reason: 'Reason',
		description: 'Description',
		status: 'Status',
		version: 'Version',
	},
	statuses: { active: 'Active', paused: 'Paused', inactive: 'Inactive' },
	pausedAt: (step) =>
		step === 'register' ? 'It failed while the site set it up. Error message: %s' : 'It failed. Error message: %s',
}

/** The error the paused plugin of these cases holds. */
const NOTES_ERROR = 'the limit must be a whole number, got "banana"'

/** A plugin that runs. */
const FEED: InstalledPlugin = {
	id: 'feed',
	name: 'Feed',
	description: 'Serves your published posts as an RSS feed.',
	version: '0.0.1',
	status: 'active',
	paused_step: '',
	paused_reason: '',
}

/** A plugin paused at the step a settings rejection stops it at. */
const NOTES: InstalledPlugin = {
	id: 'notes',
	name: 'Notes',
	description: 'Keeps a note beside each post.',
	version: '0.0.2',
	status: 'paused',
	paused_step: 'register',
	paused_reason: NOTES_ERROR,
}

/** A plugin its record holds as inactive. */
const ARCHIVE: InstalledPlugin = {
	id: 'archive',
	name: 'Archive',
	description: 'Lists the older posts by month.',
	version: '0.0.3',
	status: 'inactive',
	paused_step: '',
	paused_reason: '',
}

/** What a case renders the page with beside the view, and the address it opens at. */
interface Rendered extends Partial<Omit<PluginsPageProps, 'list'>> {
	/** The address the list opens at. */
	at?: string
}

/**
 * Renders the page over the view the address holds, opened on the page's own defaults.
 * @param options - The address and what the app hands the page beside the view.
 */
function renderPlugins({ at = '/plugins', ...props }: Rendered = {}) {
	function Mount() {
		const list = useListView<View>(pluginsListDefaults({ perPage: 20 }))
		return (
			<PluginsPage
				list={list}
				plugins={[FEED, NOTES, ARCHIVE]}
				loading={false}
				failed={false}
				words={WORDS}
				icon={pluginsGlyph}
				headingLevel={2}
				{...props}
			/>
		)
	}
	const root = createRootRoute({ component: Outlet })
	const route = createRoute({
		getParentRoute: () => root,
		path: '/plugins',
		validateSearch: listSearch,
		component: Mount,
	})
	const router = createRouter({
		routeTree: root.addChildren([route]),
		history: createMemoryHistory({ initialEntries: [at] }),
	})
	renderAdmin(<RouterProvider router={router} />)
}

/**
 * Returns the names the table shows, in the order it shows them.
 * @returns The names.
 */
async function shownNames(): Promise<string[]> {
	await screen.findByRole('table')
	return [...document.querySelectorAll('.dataviews-title-field')].map((title) => title.textContent ?? '')
}

/** The arrows DataViews draws beside the heading the table is sorted by. */
const SORT_MARKS = /[↑↓]/g

/**
 * Returns the column headings of the table, in order, without the arrow of the sorted one.
 * @returns The headings.
 */
async function headings(): Promise<string[]> {
	await screen.findByRole('table')
	return screen.getAllByRole('columnheader').map((heading) => (heading.textContent ?? '').replace(SORT_MARKS, ''))
}

/**
 * Returns the table row that names a plugin.
 * @param name - The plugin's name.
 * @returns The row.
 */
async function rowOf(name: string): Promise<HTMLElement> {
	return waitFor(() => {
		const row = screen.getAllByRole('row').find((held) => within(held).queryByText(name, { exact: true }) !== null)
		expect(row).toBeDefined()
		return row as HTMLElement
	})
}

/**
 * Returns the cell of a row under a column heading.
 * @param row - The row.
 * @param heading - The column heading.
 * @returns The cell.
 */
async function cellOf(row: HTMLElement, heading: string): Promise<HTMLElement> {
	const at = (await headings()).indexOf(heading)
	return within(row).getAllByRole('cell')[at]
}

/**
 * Returns the column under a row's name that holds the name and the line under it.
 * @param row - The row.
 * @returns The column.
 */
function primaryOf(row: HTMLElement): HTMLElement {
	return row.querySelector('.dataviews-view-table__primary-column-content') as HTMLElement
}

/**
 * Returns the path the plugins glyph draws, as it renders on its own.
 * @returns The path data.
 */
function glyphPath(): string | null {
	const { container, unmount } = render(pluginsGlyph)
	const path = container.querySelector('path')?.getAttribute('d') ?? null
	unmount()
	return path
}

test('titles the page with the words the app hands in, the sentence under it and a list region named so', async () => {
	renderPlugins()

	expect(await screen.findByRole('heading', { level: 2, name: 'Plugins' })).toBeInTheDocument()
	expect(screen.getByText('Every plugin this site holds, and whether it runs.')).toHaveClass('godmin-page__subtitle')
	expect(screen.getByRole('region', { name: 'Plugins' })).toHaveClass('godmin-page__list')
})

test('draws the tabs the app hands in under the header', async () => {
	renderPlugins({ tabs: <nav aria-label="Filter by status" /> })

	expect(await screen.findByRole('navigation', { name: 'Filter by status' })).toBeInTheDocument()
	expect(screen.getByRole('navigation', { name: 'Filter by status' }).parentElement).toHaveClass('godmin-page__tabs')
})

test('lists each plugin with its description, status and version on All, the name in the first column', async () => {
	renderPlugins()

	expect(await headings()).toEqual(['Plugin', 'Description', 'Status', 'Version'])
	const feed = await rowOf('Feed')
	expect((await cellOf(feed, 'Description')).textContent).toBe('Serves your published posts as an RSS feed.')
	expect((await cellOf(feed, 'Status')).textContent).toBe('Active')
	expect((await cellOf(feed, 'Version')).textContent).toBe('0.0.1')
})

test('opens sorted by name from A, whatever order the rows come in', async () => {
	renderPlugins()

	expect(await shownNames()).toEqual(['Archive', 'Feed', 'Notes'])
})

test('sorts by name from Z when the address asks it', async () => {
	renderPlugins({ at: '/plugins?order=desc' })

	expect(await shownNames()).toEqual(['Notes', 'Feed', 'Archive'])
})

test('marks each name with its plugin id', async () => {
	renderPlugins()

	expect(within(await rowOf('Feed')).getByText('Feed', { exact: true })).toHaveAttribute('data-item-id', 'feed')
})

test.each([
	['Feed', 'Active', 'stable'],
	['Notes', 'Paused', 'high'],
	['Archive', 'Inactive', 'none'],
] as const)('reads the state of %s as a text badge, %s with the %s intent', async (name, words, intent) => {
	renderPlugins()

	const badge = within(await rowOf(name)).getByText(words)

	expect([...badge.classList]).toEqual(badgeClasses(intent))
	expect(badge.querySelector('svg')).toBeNull()
})

test('shows why a plugin paused as the line under its name, and nothing under a plugin that runs', async () => {
	renderPlugins()

	const notes = primaryOf(await rowOf('Notes'))
	expect(notes.textContent).toBe(`NotesIt failed while the site set it up. Error message: ${NOTES_ERROR}`)
	expect(notes.querySelector('code')?.textContent).toBe(NOTES_ERROR)
	expect(primaryOf(await rowOf('Feed')).textContent).toBe('Feed')
})

test('keeps the description in a column of its own, never under the name', async () => {
	renderPlugins()

	const feed = await rowOf('Feed')

	expect(primaryOf(feed)).not.toHaveTextContent('Serves your published posts as an RSS feed.')
	expect(await cellOf(feed, 'Description')).toHaveTextContent('Serves your published posts as an RSS feed.')
})

test('leaves the cells empty for a plugin with no description and no version', async () => {
	renderPlugins({ plugins: [{ ...FEED, description: '', version: '' }] })

	const feed = await rowOf('Feed')

	expect((await cellOf(feed, 'Description')).textContent).toBe('')
	expect((await cellOf(feed, 'Version')).textContent).toBe('')
})

test.each([
	['active', ['Feed']],
	['paused', ['Notes']],
] as const)('shows only the %s plugins on that tab, with no Status column', async (status, names) => {
	renderPlugins({ status })

	expect(await shownNames()).toEqual(names)
	expect(await headings()).toEqual(['Plugin', 'Description', 'Version'])
})

test('shows why a plugin paused under its name on the Paused tab', async () => {
	renderPlugins({ status: 'paused' })

	expect(primaryOf(await rowOf('Notes')).querySelector('code')?.textContent).toBe(NOTES_ERROR)
})

test('says the site holds no plugin, with the glyph the app hands in and no button', async () => {
	renderPlugins({ plugins: [] })

	const heading = await screen.findByRole('heading', { level: 3, name: 'No plugins yet' })
	const empty = heading.closest('.godmin-empty') as HTMLElement
	expect(empty.querySelector('path')?.getAttribute('d')).toBe(glyphPath())
	expect(within(empty).queryByRole('button')).not.toBeInTheDocument()
	expect(within(empty).queryByRole('link')).not.toBeInTheDocument()
})

test.each([
	['active', 'No active plugins', [NOTES, ARCHIVE]],
	['paused', 'No paused plugins', [FEED, ARCHIVE]],
] as const)('says the %s tab holds nothing in its own words', async (status, words, plugins) => {
	renderPlugins({ status, plugins: [...plugins] })

	const heading = await screen.findByRole('heading', { level: 3, name: words })
	expect(heading.closest('.godmin-empty')?.querySelector('path')?.getAttribute('d')).toBe(glyphPath())
	expect(screen.getByRole('searchbox', { name: 'Search plugins…' })).toBeInTheDocument()
})

test.each([undefined, 'paused'] as const)('says nothing matched a search on the tab %s', async (status) => {
	renderPlugins({ at: '/plugins?search=nothing', status })

	expect(await screen.findByText('No results found')).toBeInTheDocument()
	expect(screen.queryByText('No plugins yet')).not.toBeInTheDocument()
	expect(screen.queryByText('No paused plugins')).not.toBeInTheDocument()
})

test('searches the name and the description', async () => {
	renderPlugins()

	await userEvent.type(await screen.findByRole('searchbox', { name: 'Search plugins…' }), 'rss')
	await waitFor(async () => expect(await shownNames()).toEqual(['Feed']))
	await userEvent.clear(screen.getByRole('searchbox', { name: 'Search plugins…' }))
	await userEvent.type(screen.getByRole('searchbox', { name: 'Search plugins…' }), 'archive')

	await waitFor(async () => expect(await shownNames()).toEqual(['Archive']))
})

test.each(['banana', '0.0.2', 'paused'])('never finds a plugin by its error, version or state: %j', async (words) => {
	renderPlugins({ at: `/plugins?search=${words}` })

	expect(await screen.findByText('No results found')).toBeInTheDocument()
})

test('says the plugins could not be loaded in place of the list', async () => {
	renderPlugins({ failed: true })

	expect(await screen.findByRole('alert')).toHaveTextContent('The plugins could not be loaded.')
	expect(screen.queryByRole('searchbox')).not.toBeInTheDocument()
})

test('keeps the search box and says nothing is empty while the plugins are read', async () => {
	renderPlugins({ plugins: [], loading: true })

	expect(await screen.findByRole('searchbox', { name: 'Search plugins…' })).toBeInTheDocument()
	expect(screen.queryByText('No plugins yet')).not.toBeInTheDocument()
	expect(screen.queryByRole('row')).not.toBeInTheDocument()
})

test('shows the name, the reason under it, the description and the badge on a phone, with no version', async () => {
	setViewport({ matches: true })
	renderPlugins()

	const list = within(await screen.findByRole('grid'))

	expect(await list.findByText('Feed', { exact: true })).toBeInTheDocument()
	expect(list.getByText('Serves your published posts as an RSS feed.')).toBeInTheDocument()
	expect(list.getByText('Active')).toBeInTheDocument()
	expect(list.queryByText('0.0.1')).not.toBeInTheDocument()
	expect(list.getByText(NOTES_ERROR, { selector: 'code' })).toBeInTheDocument()
	expect(screen.queryByRole('checkbox')).not.toBeInTheDocument()
})

test('drops a filter the fields do not offer, so every plugin still shows', async () => {
	const filters = encodeURIComponent(JSON.stringify([{ field: 'name', operator: 'is', value: 'Feed' }]))
	renderPlugins({ at: `/plugins?filters=${filters}` })

	expect(await shownNames()).toEqual(['Archive', 'Feed', 'Notes'])
})

test('puts a sort the fields do not offer back to the order by name', async () => {
	renderPlugins({ at: '/plugins?sort=description&order=desc' })

	expect(await shownNames()).toEqual(['Archive', 'Feed', 'Notes'])
})

test.each([
	[[10, 20, 50], true],
	[undefined, false],
] as const)('offers the page sizes %j the app hands in', async (pageSizes, offered) => {
	renderPlugins({ pageSizes: pageSizes === undefined ? undefined : [...pageSizes] })
	await screen.findByRole('table')

	await userEvent.click(screen.getByRole('button', { name: 'View options' }))

	await screen.findByRole('combobox', { name: 'Sort by' })
	expect(screen.queryByRole('radiogroup', { name: 'Items per page' }) !== null).toBe(offered)
})

test('offers the row actions the app hands in on the rows they fit', async () => {
	const callback = vi.fn()
	const settings: Action<InstalledPlugin> = {
		id: 'settings',
		label: 'Settings',
		isEligible: (plugin) => plugin.status === 'active',
		callback,
	}
	renderPlugins({ actions: [settings] })

	await userEvent.click(within(await rowOf('Feed')).getByRole('button', { name: 'Actions' }))
	await userEvent.click(await screen.findByRole('menuitem', { name: 'Settings' }))

	expect(callback).toHaveBeenCalledWith([FEED], expect.anything())
	expect(within(await rowOf('Notes')).queryByRole('button', { name: 'Actions' })).not.toBeInTheDocument()
})

test('offers no row action when the app hands in none', async () => {
	renderPlugins()

	expect(within(await rowOf('Feed')).queryByRole('button', { name: 'Actions' })).not.toBeInTheDocument()
})

test('opens on the description, status and version, the phone on the description and status, by name from A', () => {
	expect(pluginsListDefaults({ perPage: 20 })).toEqual({
		fields: ['description', 'status', 'version'],
		phoneFields: ['description', 'status'],
		titleField: 'name',
		descriptionField: 'paused_reason',
		sort: { field: 'name', direction: 'asc' },
		perPage: 20,
	})
	expect(pluginsListDefaults().perPage).toBeUndefined()
})
