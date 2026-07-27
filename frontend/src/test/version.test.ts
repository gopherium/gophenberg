// SPDX-License-Identifier: Apache-2.0

import { http, HttpResponse, server } from '@gophenberg/frontend-sdk/testing'
import { expect, test } from 'vitest'

import { fetchVersion } from '../version'

test('returns the version reported by the backend', async () => {
	server.use(
		http.get('/api/version', () => HttpResponse.json({ version: '1.2.3' })),
	)

	expect(await fetchVersion()).toBe('1.2.3')
})

test('rejects when the version request fails', async () => {
	server.use(
		http.get('/api/version', () =>
			HttpResponse.json({ error: 'internal error' }, { status: 500 }),
		),
	)

	await expect(fetchVersion()).rejects.toThrow('status 500')
})
