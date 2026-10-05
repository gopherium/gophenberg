// SPDX-License-Identifier: Apache-2.0

import { configure } from '@testing-library/react'
import { http, HttpResponse, installTestEnvironment, server } from '@gophenberg/frontend-sdk/testing'
import { DOMAIN as BRICK_DOMAIN } from '@gopherium/react-auth'
import { rememberFormatLocale, rememberLocale } from '@gopherium/gottext'
import { resetLocaleData } from '@wordpress/i18n'
import { afterAll, beforeEach } from 'vitest'

import { DEFAULT_LOCALE } from '../i18n/catalog'
import { DOMAIN } from '../i18n/start'
import { siteSettings } from './siteSettings'

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
	fields: [],
}

beforeEach(() => {
	rememberFormatLocale(siteSettings.format_locale)
	server.use(http.get('/api/settings', () => HttpResponse.json(siteSettings)))
	server.use(http.get('/api/types', () => HttpResponse.json({ items: [builtInType] })))
	server.use(http.get('/api/groups', () => HttpResponse.json({ items: [] })))
	server.use(http.get('/api/groups/params', () => HttpResponse.json({ items: [] })))
	server.use(http.get('/api/content/:id/autosave', () => HttpResponse.json({}, { status: 404 })))
	server.use(
		http.post('/api/content/:id/autosave', async ({ request, params }) =>
			HttpResponse.json({
				...((await request.json()) as Record<string, unknown>),
				target: 'autosave',
				content_id: params.id,
				saved_at: '2026-08-01T12:00:00Z',
			}),
		),
	)
})

afterAll(() => {
	resetLocaleData({}, DOMAIN)
	resetLocaleData({}, BRICK_DOMAIN)
	resetLocaleData({})
	rememberLocale(DEFAULT_LOCALE)
})
