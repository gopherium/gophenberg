// SPDX-License-Identifier: Apache-2.0

export const storedPost = {
	id: '019fb000-0000-7000-8000-000000000001',
	type: 'post',
	slug: 'welcome',
	title: 'Welcome to Gophenberg',
	excerpt: '',
	content: '<!-- wp:paragraph -->\n<p>Stored words.</p>\n<!-- /wp:paragraph -->',
	status: 'draft',
	author_id: '019fb000-0000-7000-8000-0000000000ff',
	author_name: 'Maria Perez',
	published_at: null,
	created_at: '2026-07-19T10:00:00Z',
	updated_at: '2026-07-28T09:00:00Z',
}

/**
 * Returns a stored post detail carrying the given id.
 * @param id - The id the post should report.
 * @returns The post detail.
 */
export function storedPostWithId(id: string) {
	return { ...storedPost, id }
}
