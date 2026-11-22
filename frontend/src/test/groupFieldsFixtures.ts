// SPDX-License-Identifier: Apache-2.0

import { http, HttpResponse, server } from '@gophenberg/frontend-sdk/testing'
import { screen, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { beforeEach } from 'vitest'

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

/** Serves the post type and the article details and extras groups before each test of the calling file. */
export function serveFieldGroups() {
	beforeEach(() => {
		server.use(http.get('/api/types', () => HttpResponse.json({ items: [POST_TYPE] })))
		listing([DETAILS, EXTRAS])
	})
}
