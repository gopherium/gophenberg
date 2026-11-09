// SPDX-License-Identifier: Apache-2.0

import { http, HttpResponse, server } from '@gophenberg/frontend-sdk/testing'

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

/**
 * Serves the given groups from the listing endpoint.
 * @param groups - The groups the listing answers with.
 */
export function listing(groups: unknown[]) {
	server.use(http.get('/api/groups', () => HttpResponse.json({ items: groups })))
}
