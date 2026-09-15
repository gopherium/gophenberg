// SPDX-License-Identifier: Apache-2.0

import { http, HttpResponse, server } from '@gophenberg/frontend-sdk/testing'
import { screen } from '@testing-library/react'
import { setLocaleData } from '@wordpress/i18n'
import { beforeEach, expect, test } from 'vitest'

import { renderAt } from './render'

beforeEach(() => {
	server.use(
		http.get('/api/content', () => HttpResponse.json({ items: [], total: 0 })),
		http.get('/api/content/counts', () => HttpResponse.json({})),
	)
})

test('shapes the version line the way the catalogue the reader loaded says', async () => {
	setLocaleData({ 'v%(version)s': ['version %(version)s'] }, 'gophenberg')
	renderAt('/', undefined, '1.2.3')

	expect(await screen.findByText('version 1.2.3')).toBeInTheDocument()
})
