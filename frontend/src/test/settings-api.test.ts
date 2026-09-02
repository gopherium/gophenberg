// SPDX-License-Identifier: Apache-2.0

import { http, HttpResponse, server } from '@gophenberg/frontend-sdk/testing'
import { expect, test } from 'vitest'

import { chooseSiteSettings, fetchSiteSettings } from '../settings/api'

test('reads what the site chose for itself', async () => {
	server.use(
		http.get('/api/settings', () =>
			HttpResponse.json({ locale_default: 'es-ES', content_per_page: 5, jpeg_quality: 30 }),
		),
	)

	const held = await fetchSiteSettings()

	expect(held).toEqual({ locale_default: 'es-ES', content_per_page: 5, jpeg_quality: 30 })
})

test('falls back to the defaults when the settings cannot be read', async () => {
	server.use(http.get('/api/settings', () => HttpResponse.json({ error: 'boom' }, { status: 500 })))

	const held = await fetchSiteSettings()

	expect(held).toEqual({ locale_default: '', content_per_page: 20, jpeg_quality: 82 })
})

test('falls back to the defaults when the settings cannot be reached', async () => {
	server.use(http.get('/api/settings', () => HttpResponse.error()))

	const held = await fetchSiteSettings()

	expect(held.content_per_page).toBe(20)
})

test('stores only the values it was given', async () => {
	const sent: string[] = []
	server.use(
		http.patch('/api/settings', async ({ request }) => {
			sent.push(JSON.stringify(await request.json()))
			return HttpResponse.json({ locale_default: '', content_per_page: 5, jpeg_quality: 82 })
		}),
	)

	await chooseSiteSettings({ content_per_page: 5 })

	expect(sent[0]).toBe('{"content_per_page":5}')
})

test('reports a refusal that carries no reason as a general failure', async () => {
	server.use(
		http.patch('/api/settings', () => new HttpResponse('not json at all', { status: 500 })),
	)

	await expect(chooseSiteSettings({ jpeg_quality: 30 })).rejects.toThrow()
})

test('reports a refused setting in words a reader understands', async () => {
	server.use(
		http.patch('/api/settings', () =>
			HttpResponse.json(
				{ error: 'per page invalid', code: 'per_page_invalid', meta: { value: '500', max: 100 } },
				{ status: 422 },
			),
		),
	)

	await expect(chooseSiteSettings({ content_per_page: 500 })).rejects.toThrow(/100/)
})
