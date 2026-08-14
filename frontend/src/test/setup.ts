// SPDX-License-Identifier: Apache-2.0

import { configure } from '@testing-library/react'
import { http, HttpResponse, installTestEnvironment, server } from '@gophenberg/frontend-sdk/testing'
import { beforeAll, beforeEach } from 'vitest'

installTestEnvironment()
configure({ asyncUtilTimeout: 2000 })

/** The built-in type every installation carries, which the migration seeds. */
const builtInType = {
	key: 'post',
	singular_label: 'Post',
	plural_label: 'Posts',
	route_word: '',
	hierarchical: false,
	revisions: true,
	revision_cap: 100,
	page_kind: 'single',
	default: true,
	active: true,
	created_at: '2026-08-01T10:00:00Z',
	updated_at: '2026-08-01T10:00:00Z',
}

beforeEach(() => {
	server.use(http.get('/api/types', () => HttpResponse.json({ items: [builtInType] })))
})

beforeAll(async () => {
	await Promise.all([
		import('../content/PostsScreen'),
		import('../content/TypesScreen'),
		import('../media/MediaScreen'),
		import('../users/UsersScreen'),
		import('../users/NewUserScreen'),
		import('../themes/ThemesScreen'),
	])
})
