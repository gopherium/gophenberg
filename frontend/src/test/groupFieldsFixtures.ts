// SPDX-License-Identifier: Apache-2.0

import { http, HttpResponse, server } from '@gophenberg/frontend-sdk/testing'
import { screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { beforeEach } from 'vitest'

/** The title a group holding no fields shows where its fields table would be. */
export const NO_FIELDS = 'No fields yet'

/** The sentence a group holding no fields shows under that title. */
export const NO_FIELDS_SENTENCE = 'Add a field to store more about the items this group shows on.'

/** The built-in type the field groups are placed on, as the registry answers it. */
export const POST_TYPE = {
	key: 'post',
	singular_label: 'Post',
	plural_label: 'Posts',
	description: '',
	route_word: '',
	hierarchical: false,
	revisions: true,
	revision_cap: 100,
	page_kind: 'single',
	default: true,
	active: true,
	fields: [],
}

/** The optional text field the article details group holds first. */
export const SUBTITLE = {
	key: 'subtitle',
	label: 'Subtitle',
	kind: 'text',
	many: false,
	required: false,
	created_at: '2026-08-01T10:00:00Z',
	updated_at: '2026-08-01T10:00:00Z',
}

/** The required number field the article details group holds second. */
export const READING_TIME = {
	...SUBTITLE,
	key: 'reading-time',
	label: 'Reading time',
	kind: 'number',
	required: true,
}

/** The group placed on the posts that holds the subtitle and the reading time. */
export const DETAILS = {
	id: 3,
	key: 'article-details',
	title: 'Article details',
	location: [[{ source: 'content_type', operator: '==', value: 'post' }]],
	position: 1,
	active: true,
	fields: [SUBTITLE, READING_TIME],
	created_at: '2026-08-01T10:00:00Z',
	updated_at: '2026-08-01T10:00:00Z',
}

/** The second group placed on the posts, holding no fields yet. */
export const EXTRAS = {
	...DETAILS,
	id: 4,
	key: 'extras',
	title: 'Extras',
	position: 2,
	fields: [],
}

/** The flexible content field holding no layouts yet. */
export const FEATURES = {
	...SUBTITLE,
	key: 'features',
	label: 'Features',
	kind: 'flexible',
	fields: [],
}

/** The layout holding no fields yet. */
export const HERO = {
	...SUBTITLE,
	key: 'hero',
	label: 'Hero',
	kind: 'layout',
	fields: [],
}

/**
 * Serves the given groups from the listing endpoint.
 * @param groups - The groups the listing answers with.
 */
export function listing(groups: unknown[]) {
	server.use(http.get('/api/groups', () => HttpResponse.json({ items: groups })))
}

/**
 * Opens the page of the named group from its title on the list.
 * @param named - The group whose page to open.
 * @param list - The name the list region carries.
 * @returns The main work of the page.
 */
export async function openFields(named = 'Article details', list = 'Field Groups') {
	const table = await screen.findByRole('region', { name: list })
	await userEvent.click(within(table).getByRole('link', { name: named }))
	const main = screen.getByRole('main')
	await within(main).findByRole('heading', { level: 2, name: named })
	return main
}

/**
 * Returns the region the fields table of a group page scrolls in.
 * @param main - The main work of the page.
 * @returns The region.
 */
export function fieldsOf(main: HTMLElement) {
	return within(main).getByRole('region', { name: 'Fields' })
}

/**
 * Reports whether a row of the fields table is a field's own row and shows the label in its Label cell.
 * @param row - The row of the table.
 * @param label - The label looked for.
 * @returns Whether the row is the field's, not the header nor the row listing the fields inside a container.
 */
function labelledRow(row: HTMLElement, label: string) {
	const [first, second] = within(row).queryAllByRole('cell')
	return second !== undefined && within(first).queryByText(label, { exact: true }) !== null
}

/**
 * Returns the row of the fields table showing a field's label, once it shows.
 * @param main - The main work of the page.
 * @param label - The label the row shows.
 * @returns The row.
 */
export async function fieldRow(main: HTMLElement, label: string) {
	return waitFor(() => {
		const row = within(fieldsOf(main))
			.getAllByRole('row')
			.find((held) => labelledRow(held, label))
		if (row === undefined) {
			throw new Error(`No row of the fields table shows ${label}`)
		}
		return row
	})
}

/** Serves the post type and the article details and extras groups before each test of the calling file. */
export function serveFieldGroups() {
	beforeEach(() => {
		server.use(http.get('/api/types', () => HttpResponse.json({ items: [POST_TYPE] })))
		listing([DETAILS, EXTRAS])
	})
}
