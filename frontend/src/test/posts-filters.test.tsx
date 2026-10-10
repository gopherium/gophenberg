// SPDX-License-Identifier: Apache-2.0

import { http, HttpResponse, server } from '@gophenberg/frontend-sdk/testing'
import { rememberLocale } from '@gopherium/gottext'
import { fireEvent, screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { getSettings, setSettings } from '@wordpress/date'
import { resetLocaleData, setLocaleData } from '@wordpress/i18n'
import { afterEach, beforeEach, expect, onTestFinished, test, vi } from 'vitest'

import { DEFAULT_LOCALE, catalogFor } from '../i18n/catalog'
import { startDates } from '../i18n/dates'
import { startLocale } from '../i18n/start'
import { renderAt } from './render'
import { warmPostsScreen } from './warm'

warmPostsScreen()

const OWN_AUTHOR = '019fb000-0000-7000-8000-0000000000ff'

const FOREIGN_AUTHOR = '019fb000-0000-7000-8000-0000000000ee'

const ROW = {
	id: '019fb000-0000-7000-8000-000000000001',
	type: 'post',
	slug: 'welcome',
	title: 'Welcome to Gophenberg',
	excerpt: '',
	status: 'published',
	author_id: OWN_AUTHOR,
	author_name: 'Maria Perez',
	published_at: '2026-07-20T10:00:00Z',
	created_at: '2026-07-19T10:00:00Z',
	updated_at: '2026-07-20T10:00:00Z',
}

const asked: URLSearchParams[] = []
let authorReads = 0

beforeEach(() => {
	asked.length = 0
	authorReads = 0
	server.use(
		http.get('/api/content', ({ request }) => {
			asked.push(new URL(request.url).searchParams)
			return HttpResponse.json({ items: [ROW], total: 1, per_page: 20 })
		}),
		http.get('/api/authors', () => {
			authorReads += 1
			return HttpResponse.json({
				items: [
					{ id: FOREIGN_AUTHOR, name: 'Foreign Author' },
					{ id: OWN_AUTHOR, name: 'Maria Perez' },
				],
			})
		}),
	)
})

/**
 * Returns the address of the posts list narrowed by the given filters.
 * @param filters - The filters the address holds.
 * @returns The address.
 */
function narrowedAt(filters: unknown[]): string {
	return `/content/post?filters=${encodeURIComponent(JSON.stringify(filters))}`
}

/**
 * Picks a filter through the Add filter menu, which opens its chip.
 * @param name - The label of the column to filter by.
 */
async function pickFilter(name: string) {
	await screen.findByText('Welcome to Gophenberg')
	await userEvent.click(screen.getByRole('button', { name: 'Add filter' }))
	await userEvent.click(await screen.findByRole('menuitem', { name }))
}

test('asks for the posts of the authors picked under the author chip', async () => {
	renderAt('/content/post')
	await pickFilter('Author')

	await userEvent.click(await screen.findByRole('option', { name: 'Maria Perez' }))

	await waitFor(() => expect(asked.at(-1)?.get('author')).toBe(OWN_AUTHOR))
})

test('asks for nothing from an author chip opened with no author picked', async () => {
	renderAt('/content/post')
	await pickFilter('Author')

	await screen.findByRole('option', { name: 'Maria Perez' })

	expect(asked.every((search) => !search.has('author'))).toBe(true)
})

test('reads the authors when the author chip opens, not again on each render of the list', async () => {
	renderAt('/content/post')
	await pickFilter('Author')
	await userEvent.click(await screen.findByRole('option', { name: 'Maria Perez' }))
	await waitFor(() => expect(asked.at(-1)?.get('author')).toBe(OWN_AUTHOR))
	const opened = authorReads
	await userEvent.keyboard('{Escape}')

	await userEvent.type(screen.getByRole('searchbox', { name: 'Search posts…' }), 'welcome')

	await waitFor(() => expect(asked.at(-1)?.get('search')).toBe('welcome'))
	expect(opened).toBeGreaterThan(0)
	expect(authorReads).toBe(opened)
})

test('offers no author when the authors cannot be read', async () => {
	server.use(http.get('/api/authors', () => HttpResponse.json({}, { status: 500 })))
	renderAt('/content/post')
	await pickFilter('Author')

	await waitFor(() => expect(screen.queryByRole('option', { name: 'Maria Perez' })).not.toBeInTheDocument())
})

test('asks the server to leave out the authors the address names under Is none of', async () => {
	renderAt(narrowedAt([{ field: 'author', operator: 'isNone', value: [FOREIGN_AUTHOR, OWN_AUTHOR] }]))

	await screen.findByText('Welcome to Gophenberg')

	expect(asked[0].get('author_exclude')).toBe(`${FOREIGN_AUTHOR},${OWN_AUTHOR}`)
	expect(asked[0].has('author')).toBe(false)
})

test('asks for the posts of every author the address names', async () => {
	renderAt(narrowedAt([{ field: 'author', operator: 'isAny', value: [FOREIGN_AUTHOR, OWN_AUTHOR] }]))

	await screen.findByText('Welcome to Gophenberg')

	expect(asked[0].get('author')).toBe(`${FOREIGN_AUTHOR},${OWN_AUTHOR}`)
})

test.each([
	{ shape: 'no account id', value: ['nope'] },
	{ shape: 'an account id beside one that is none', value: [OWN_AUTHOR, 'nope'] },
])('asks no author the address names when it holds $shape', async ({ value }) => {
	renderAt(`${narrowedAt([{ field: 'author', operator: 'isAny', value }])}&search=welcome`)

	await screen.findByText('Welcome to Gophenberg')

	expect(asked[0].get('search')).toBe('welcome')
	expect(asked[0].has('author')).toBe(false)
})

test('asks for the posts dated before a date chip as an RFC 3339 time', async () => {
	renderAt(narrowedAt([{ field: 'date', operator: 'before', value: '2026-08-16' }]))

	await screen.findByText('Welcome to Gophenberg')

	expect(asked[0].get('before')).toBe('2026-08-16T00:00:00.000Z')
	expect(asked[0].has('after')).toBe(false)
})

test('asks for the posts dated after a date chip as an RFC 3339 time', async () => {
	renderAt(narrowedAt([{ field: 'date', operator: 'after', value: '2026-08-16T12:00:00.000Z' }]))

	await screen.findByText('Welcome to Gophenberg')

	expect(asked[0].get('after')).toBe('2026-08-16T12:00:00.000Z')
})

test('writes the moment of a date chip in the full date and time format, as the WordPress chip does', async () => {
	renderAt(narrowedAt([{ field: 'date', operator: 'after', value: '2026-08-16T12:00:00.000Z' }]))
	await screen.findByText('Welcome to Gophenberg')

	await userEvent.click(screen.getByRole('button', { name: 'Filter' }))

	expect(await screen.findByText('August 16, 2026 12:00 pm')).toBeInTheDocument()
})

test('writes the moment of a date chip in the full date and time format of the reader language', async () => {
	setLocaleData(await catalogFor('es-ES'), 'gophenberg')
	onTestFinished(() => resetLocaleData({}, 'gophenberg'))
	startDates('es-ES')
	renderAt(narrowedAt([{ field: 'date', operator: 'after', value: '2026-08-16T12:00:00.000Z' }]))
	await screen.findByText('Welcome to Gophenberg')

	await userEvent.click(screen.getByRole('button', { name: 'Filter' }))

	expect(await screen.findByText('16 de agosto de 2026 12:00')).toBeInTheDocument()
})

test.each(['someday', 0, '20261-10-08', '10000-01-01T00:00:00.000Z', '-000001-12-31T00:00:00.000Z'])(
	'asks no date the address names that is no moment the server reads, such as %s',
	async (value) => {
		renderAt(`${narrowedAt([{ field: 'date', operator: 'before', value }])}&search=welcome`)

		await screen.findByText('Welcome to Gophenberg')

		expect(asked[0].get('search')).toBe('welcome')
		expect(asked[0].has('before')).toBe(false)
	},
)

test.each(['0000-01-01T00:00:00.000Z', '9999-12-31T23:59:00.000Z'])(
	'asks for a date the address names in the first or the last year the server reads, such as %s',
	async (value) => {
		renderAt(narrowedAt([{ field: 'date', operator: 'before', value }]))

		await screen.findByText('Welcome to Gophenberg')

		expect(asked[0].get('before')).toBe(value)
	},
)

/**
 * Opens the date chip the address holds, its field and its calendar.
 * @returns The date and time field of the chip.
 */
async function openDateChip(): Promise<HTMLElement> {
	await screen.findByText('Welcome to Gophenberg')
	await userEvent.click(screen.getByRole('button', { name: 'Filter' }))
	await userEvent.click(await screen.findByRole('button', { name: /^Date/, pressed: false }))
	return screen.findByLabelText(/^Date/, { selector: 'input' })
}

/** The month abbreviations a date cell writes in the language the sources are written in, January first. */
const MONTHS = ['Jan', 'Feb', 'Mar', 'Apr', 'May', 'Jun', 'Jul', 'Aug', 'Sep', 'Oct', 'Nov', 'Dec']

/**
 * Returns a number written with two digits.
 * @param value - The number.
 * @returns The digits, a zero before a single one.
 */
function twoDigits(value: number): string {
	return String(value).padStart(2, '0')
}

/**
 * Returns the date and the time a date cell reads, as a datetime-local field writes them.
 * @param cell - The date cell, such as one reading Published: Jul 20, 2026 12:00 pm.
 * @returns The date and the time, such as 2026-07-20T12:00.
 */
function cellClock(cell: HTMLElement): string {
	const written = /(\w{3}) (\d{1,2}), (\d{4}) (\d{1,2}):(\d{2}) (am|pm)/.exec(cell.textContent ?? '') ?? []
	const [, month, day, year, hour, minute, half] = written
	const hours = (Number(hour) % 12) + (half === 'pm' ? 12 : 0)
	return `${year}-${twoDigits(MONTHS.indexOf(month) + 1)}-${twoDigits(Number(day))}T${twoDigits(hours)}:${minute}`
}

/** The date settings in place before any test boots a language. */
const DATE_DEFAULTS = getSettings()

/**
 * Starts the admin in a language as the boot does, reading no catalogue.
 * @param locale - The language the server answers.
 */
async function bootIn(locale: string) {
	server.use(http.get('/api/locale', () => HttpResponse.json({ locale, supported: [DEFAULT_LOCALE, locale] })))
	const none = async () => undefined
	await startLocale({ own: none, editor: none, brick: none, chrome: none })
}

/** Sets the reader's clock to Paris, two hours ahead of UTC in summer, wherever the runner lets Node move the zone. */
async function readInParis() {
	process.env.TZ = 'Europe/Paris'
	await bootIn(DEFAULT_LOCALE)
}

afterEach(() => {
	vi.useRealTimers()
	process.env.TZ = 'UTC'
	setSettings(DATE_DEFAULTS)
	rememberLocale(DEFAULT_LOCALE)
})

test('shows the moment of a date chip at the clock time the date cells show', async () => {
	await readInParis()
	renderAt(narrowedAt([{ field: 'date', operator: 'after', value: ROW.published_at }]))
	const cell = await screen.findByText(/^Published: /)

	const field = await openDateChip()

	expect(field).toHaveValue(cellClock(cell))
	expect(screen.getByRole('application')).toHaveStyle({ width: '100%' })
})

test('asks for the moment a reader types in a date chip as the cell reading that clock time dates it', async () => {
	await readInParis()
	renderAt(narrowedAt([{ field: 'date', operator: 'before', value: '2026-08-16T12:00:00.000Z' }]))
	const cell = await screen.findByText(/^Published: /)
	const field = await openDateChip()

	fireEvent.change(field, { target: { value: cellClock(cell) } })

	await waitFor(() => expect(asked.at(-1)?.get('before')).toBe('2026-07-20T10:00:00.000Z'))
})

test('marks the day a moment falls on as the cells date it, and keeps its time and focus on another day', async () => {
	await readInParis()
	const late = '2026-08-16T23:30:00.000Z'
	server.use(
		http.get('/api/content', ({ request }) => {
			asked.push(new URL(request.url).searchParams)
			return HttpResponse.json({ items: [{ ...ROW, published_at: late }], total: 1, per_page: 20 })
		}),
	)
	renderAt(narrowedAt([{ field: 'date', operator: 'before', value: late }]))
	const clock = cellClock(await screen.findByText(/^Published: /))
	await openDateChip()

	expect(screen.getByRole('gridcell', { selected: true })).toHaveTextContent(
		new RegExp(`^${Number(clock.slice(8, 10))}$`),
	)
	await userEvent.click(within(screen.getByRole('grid')).getByText('20'))

	const picked = new Date(`${clock.slice(0, 8)}20T${clock.slice(11)}`).toISOString()
	await waitFor(() => expect(asked.at(-1)?.get('before')).toBe(picked))
	expect(within(screen.getByRole('gridcell', { selected: true })).getByRole('button')).toHaveFocus()
})

test('asks for the midnight on the reader clock of a day picked in a date chip holding no moment', async () => {
	await readInParis()
	vi.useFakeTimers({ toFake: ['Date'] })
	vi.setSystemTime(new Date('2026-08-10T12:00:00.000Z'))
	renderAt(narrowedAt([{ field: 'date', operator: 'before' }]))
	await openDateChip()

	await userEvent.click(within(screen.getByRole('grid')).getByText('20'))

	await waitFor(() => expect(asked.at(-1)?.get('before')).toBe(new Date(2026, 7, 20).toISOString()))
})

test('turns the calendar of a date chip to the month of the moment a reader types', async () => {
	renderAt(narrowedAt([{ field: 'date', operator: 'after', value: '2026-07-20T10:00:00.000Z' }]))
	const field = await openDateChip()

	fireEvent.change(field, { target: { value: '2026-09-03T10:00' } })

	await waitFor(() => expect(screen.getByRole('gridcell', { selected: true })).toHaveTextContent(/^3$/))
	expect(screen.getByRole('grid')).toHaveAccessibleName(/September 2026/)
})

test('names the month of a date chip calendar in the reader language and starts on Monday in es-ES', async () => {
	await bootIn('es-ES')
	renderAt(narrowedAt([{ field: 'date', operator: 'after', value: ROW.published_at }]))

	await openDateChip()

	const calendar = screen.getByRole('grid')
	expect(calendar).toHaveAccessibleName('julio de 2026')
	expect(within(calendar).getAllByRole('columnheader', { hidden: true })[0]).toHaveAccessibleName('lunes')
})

test.each([
	{
		how: 'a reader clears its field',
		clear: (field: HTMLElement) => fireEvent.change(field, { target: { value: '' } }),
	},
	{
		how: 'a reader picks its day again',
		clear: () => userEvent.click(within(screen.getByRole('gridcell', { selected: true })).getByRole('button')),
	},
])('asks for no moment from a date chip once $how', async ({ clear }) => {
	renderAt(narrowedAt([{ field: 'date', operator: 'after', value: '2026-07-20T10:00:00.000Z' }]))
	const field = await openDateChip()

	await clear(field)

	await waitFor(() => expect(asked.at(-1)?.has('after')).toBe(false))
})

test('asks for nothing from a date chip the address opened with no moment', async () => {
	renderAt(`${narrowedAt([{ field: 'date', operator: 'before' }])}&search=welcome`)

	await screen.findByText('Welcome to Gophenberg')

	expect(asked[0].get('search')).toBe('welcome')
	expect(asked[0].has('before')).toBe(false)
})
