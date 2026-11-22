// SPDX-License-Identifier: Apache-2.0

import { http, HttpResponse, server } from '@gophenberg/frontend-sdk/testing'
import { listSearch } from '@gopherium/godmin/router'
import { screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { resetLocaleData, setLocaleData } from '@wordpress/i18n'
import { beforeAll, beforeEach, expect, onTestFinished, test } from 'vitest'

import { pluginStatusSearch } from '../pluginsscreen/nav'
import { pluginsWords } from '../pluginsscreen/words'
import { createAppRouter } from '../router'
import { renderAt, renderRoutedAt } from './render'

/** The error the paused plugin of these cases holds. */
const NOTES_ERROR = 'GOPHENBERG_PLUGIN_NOTES__LIMIT: must be a whole number, got "banana"'

/** The feed as the server lists it while it runs. */
const FEED = {
	id: 'feed',
	name: 'Feed',
	description: 'Serves your published posts as an RSS feed.',
	version: '0.0.1',
	status: 'active',
	paused_step: '',
	paused_reason: '',
}

/** A plugin the server lists as paused at the step a settings rejection stops it at. */
const NOTES = {
	id: 'notes',
	name: 'Notes',
	description: 'Keeps a note beside each post.',
	version: '0.0.2',
	status: 'paused',
	paused_step: 'register',
	paused_reason: NOTES_ERROR,
}

/** A plugin the server lists as inactive. */
const ARCHIVE = {
	id: 'archive',
	name: 'Archive',
	description: 'Lists the older posts by month.',
	version: '0.0.3',
	status: 'inactive',
	paused_step: '',
	paused_reason: '',
}

/**
 * Serves the plugins as the installed plugin list.
 * @param plugins - The plugins the list answers.
 */
function servePlugins(...plugins: Record<string, string>[]) {
	server.use(http.get('/api/plugin-admin', () => HttpResponse.json({ plugins })))
}

beforeAll(async () => {
	await import('../pluginsscreen/PluginsScreen')
})

beforeEach(() => servePlugins(FEED, NOTES, ARCHIVE))

/**
 * Returns the names the table shows, in the order it shows them.
 * @returns The names.
 */
async function shownNames(): Promise<string[]> {
	await screen.findByRole('table')
	return [...document.querySelectorAll('.dataviews-title-field')].map((title) => title.textContent ?? '')
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
 * Returns the tab carrying the label.
 * @param label - The label of the tab.
 * @returns The tab link.
 */
function tab(label: string): HTMLElement {
	return within(screen.getByRole('navigation', { name: 'Filter by status' })).getByRole('link', { name: label })
}

test('serves the Plugins screen at /plugins, titled at the second level with its sentence under it', async () => {
	renderAt('/plugins')

	expect(await screen.findByRole('heading', { level: 2, name: 'Plugins' })).toBeInTheDocument()
	expect(screen.getByText('Every plugin this site holds, and whether it runs.')).toHaveClass('godmin-page__subtitle')
	expect(screen.getByRole('region', { name: 'Plugins' })).toBeInTheDocument()
})

test('lists the feed with its description, the Active badge and its version', async () => {
	renderAt('/plugins')

	const feed = within(await rowOf('Feed'))

	expect(feed.getByText('Serves your published posts as an RSS feed.')).toBeInTheDocument()
	expect(feed.getByText('Active')).toBeInTheDocument()
	expect(feed.getByText('0.0.1')).toBeInTheDocument()
})

test('opens the Plugins screen from its row of the menu', async () => {
	renderAt('/')

	const row = await screen.findByRole('link', { name: 'Plugins' })
	expect(row).toHaveAttribute('href', '/admin/plugins')
	await userEvent.click(row)

	expect(await screen.findByRole('heading', { level: 2, name: 'Plugins' })).toBeInTheDocument()
})

test('offers a search box named after the plugins', async () => {
	renderAt('/plugins')

	expect(await screen.findByRole('searchbox', { name: 'Search plugins…' })).toBeInTheDocument()
})

test('tells why a plugin paused under its name, in the sentence of its step with its own error', async () => {
	renderAt('/plugins')

	const notes = await rowOf('Notes')

	expect(notes).toHaveTextContent(`It failed while the site set it up. Error message: ${NOTES_ERROR}`)
	expect(within(notes).getByText(NOTES_ERROR, { selector: 'code' })).toBeInTheDocument()
	expect(within(notes).getByText('Paused')).toBeInTheDocument()
})

test('shows the three tabs with no counts, marking only All', async () => {
	renderAt('/plugins')

	const tabs = within(await screen.findByRole('navigation', { name: 'Filter by status' })).getAllByRole('link')

	expect(tabs.map((link) => link.textContent)).toEqual(['All', 'Active', 'Paused'])
	expect(tabs.map((link) => link.getAttribute('aria-current'))).toEqual(['page', null, null])
})

test.each([
	['active', 'Active', ['Feed']],
	['paused', 'Paused', ['Notes']],
])('opens the %s tab the address names, holding only the plugins in that state', async (status, label, names) => {
	renderAt(`/plugins?status=${status}`)

	await screen.findByRole('navigation', { name: 'Filter by status' })

	expect(tab(label)).toHaveAttribute('aria-current', 'page')
	expect(tab('All')).not.toHaveAttribute('aria-current')
	expect(await shownNames()).toEqual(names)
	expect(screen.queryByRole('columnheader', { name: 'Status' })).not.toBeInTheDocument()
})

test('reads a state no tab names as All, holding every plugin', async () => {
	renderAt('/plugins?status=inactive')

	await screen.findByRole('navigation', { name: 'Filter by status' })

	expect(tab('All')).toHaveAttribute('aria-current', 'page')
	expect(await shownNames()).toEqual(['Archive', 'Feed', 'Notes'])
})

test('drops the page and the search and keeps the order and the page size on a tab', async () => {
	const { router } = renderRoutedAt('/plugins?page=2&search=feed&order=desc&perPage=50')
	await screen.findByRole('navigation', { name: 'Filter by status' })

	await userEvent.click(tab('Paused'))

	await waitFor(() => expect(router.state.location.search).toEqual({ status: 'paused', order: 'desc', perPage: 50 }))
	expect(await shownNames()).toEqual(['Notes'])
})

test('drops the state and keeps every other choice on All', async () => {
	const { router } = renderRoutedAt('/plugins?status=paused&search=notes&order=desc')
	await screen.findByRole('navigation', { name: 'Filter by status' })

	await userEvent.click(tab('All'))

	await waitFor(() => expect(router.state.location.search).toEqual({ order: 'desc' }))
})

test.each(['active', 'paused'])('keeps the state %s an address names, which a tab narrows to', (status) => {
	expect(pluginStatusSearch({ status })).toEqual({ status })
})

/** States no tab of the Plugins list narrows it to, each one an address may still name. */
const OTHER_STATES = ['inactive', 'published', '', 'Active', 3, undefined]

test.each(OTHER_STATES)('drops any other state an address names: %j', (status) => {
	expect(pluginStatusSearch({ status })).toEqual({ status: undefined })
})

test('reads the list view and the tab state from the address of the Plugins route', () => {
	const plugins = Object.values(createAppRouter().routesById).find((route) => route.fullPath === '/plugins')
	const raw = { status: 'paused', search: 'notes', page: 2, order: 'desc', other: 'kept out' }

	expect(plugins?.options.validateSearch?.(raw)).toEqual({ ...listSearch(raw), status: 'paused' })
})

test.each([
	['register', 'It failed while the site set it up. Error message: %s'],
	['roles', 'It failed while it declared its roles. Error message: %s'],
	['types', 'It failed while it declared its content types. Error message: %s'],
	['hooks', 'It failed while the site checked its hooks. Error message: %s'],
	['routes', 'It failed while it handed over its web addresses. Error message: %s'],
	['migrate', 'It failed while it updated its database tables. Error message: %s'],
	['start', 'It failed while it started. Error message: %s'],
	['crash', 'It failed. Error message: %s'],
	['', 'It failed. Error message: %s'],
	['constructor', 'It failed. Error message: %s'],
])('says a plugin paused at the step %j in its own sentence', (step, sentence) => {
	expect(pluginsWords().pausedAt(step)).toBe(sentence)
})

test('names each state of a plugin in its own words', () => {
	expect(pluginsWords().statuses).toEqual({ active: 'Active', paused: 'Paused', inactive: 'Inactive' })
})

test('labels the fields in the words of a list of plugins', () => {
	expect(pluginsWords().labels).toEqual({
		name: 'Plugin',
		paused_reason: 'Reason',
		description: 'Description',
		status: 'Status',
		version: 'Version',
	})
})

test('says the site holds no plugin yet', async () => {
	servePlugins()
	renderAt('/plugins')

	expect(await screen.findByRole('heading', { level: 3, name: 'No plugins yet' })).toBeInTheDocument()
})

test.each([
	['active', 'No active plugins', [NOTES, ARCHIVE]],
	['paused', 'No paused plugins', [FEED, ARCHIVE]],
])('says the %s tab holds nothing in its own words', async (status, words, plugins) => {
	servePlugins(...plugins)
	renderAt(`/plugins?status=${status}`)

	expect(await screen.findByRole('heading', { level: 3, name: words })).toBeInTheDocument()
})

test.each([403, 503, 500])('says the plugins could not be loaded when the server answers %i', async (status) => {
	server.use(http.get('/api/plugin-admin', () => HttpResponse.json({ error: 'not listed' }, { status })))
	renderAt('/plugins')

	expect(await screen.findByRole('alert')).toHaveTextContent('The plugins could not be loaded.')
	const main = screen.getByRole('main')
	expect(within(main).getAllByRole('heading', { level: 2 })).toHaveLength(1)
	expect(within(main).getByRole('heading', { level: 2 })).toHaveTextContent('Plugins')
	expect(screen.queryByRole('searchbox')).not.toBeInTheDocument()
})

test('keeps the heading and the search while the plugins are on their way', async () => {
	server.use(http.get('/api/plugin-admin', () => new Promise(() => {})))
	renderAt('/plugins')

	const main = await screen.findByRole('main')

	expect(await within(main).findByRole('searchbox', { name: 'Search plugins…' })).toBeInTheDocument()
	expect(within(main).getByRole('heading', { level: 2 })).toHaveTextContent('Plugins')
	expect(within(main).queryByText('Feed')).toBeNull()
	expect(within(main).queryByText('No plugins yet')).toBeNull()
})

test('shows the name and the description in the words of the catalogue, the error as the plugin gave it', async () => {
	setLocaleData(
		{
			Notes: ['Post Notes'],
			'Keeps a note beside each post.': ['Holds one note for every post.'],
			[NOTES_ERROR]: ['A translated error'],
		},
		'gophenberg',
	)
	onTestFinished(() => resetLocaleData({}, 'gophenberg'))
	renderAt('/plugins')

	const notes = within(await rowOf('Post Notes'))

	expect(notes.getByText('Holds one note for every post.')).toBeInTheDocument()
	expect(notes.getByText(NOTES_ERROR, { selector: 'code' })).toBeInTheDocument()
})

test('never hands an empty description to the catalogue, where gettext keeps its header', async () => {
	setLocaleData({ '': ['Project-Id-Version: Gophenberg'], Feed: ['Site Feed'] }, 'gophenberg')
	onTestFinished(() => resetLocaleData({}, 'gophenberg'))
	servePlugins({ ...FEED, description: '' })
	renderAt('/plugins')

	const feed = await rowOf('Site Feed')

	expect(feed).not.toHaveTextContent('Project-Id-Version')
})
