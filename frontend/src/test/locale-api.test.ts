// SPDX-License-Identifier: AGPL-3.0-or-later

import { http, HttpResponse, server } from '@gophenberg/frontend-sdk/testing'
import { expect, test } from 'vitest'

import { chooseLocale, fetchLocale, fetchSiteLocale, chooseSiteLocale } from '../i18n/api'

test('reads the language the server answers in', async () => {
	server.use(
		http.get('/api/locale', () =>
			HttpResponse.json({ locale: 'es-ES', supported: ['en-US', 'es-ES'] }),
		),
	)

	const held = await fetchLocale()

	expect(held.locale).toBe('es-ES')
	expect(held.supported).toEqual(['en-US', 'es-ES'])
})

test('falls back to the source language when the server cannot be reached', async () => {
	server.use(http.get('/api/locale', () => HttpResponse.error()))

	const held = await fetchLocale()

	expect(held.locale).toBe('en-US')
})

test('falls back when the server refuses to name a language', async () => {
	server.use(http.get('/api/locale', () => HttpResponse.json({}, { status: 500 })))

	expect((await fetchLocale()).locale).toBe('en-US')
})

test('stores the language a reader chose', async () => {
	const sent: string[] = []
	server.use(
		http.patch('/api/locale', async ({ request }) => {
			sent.push(JSON.stringify(await request.json()))
			return HttpResponse.json({ locale: 'es-ES', supported: ['en-US', 'es-ES'] })
		}),
	)

	await chooseLocale('es-ES')

	expect(sent[0]).toBe('{"locale":"es-ES"}')
})

test('answers the language the server stored', async () => {
	server.use(
		http.patch('/api/locale', () =>
			HttpResponse.json({ locale: 'es-ES', supported: ['en-US', 'es-ES'] }),
		),
	)

	expect((await chooseLocale('es-ES')).locale).toBe('es-ES')
})

test('reports a refusal that carries no readable body', async () => {
	server.use(http.patch('/api/locale', () => new HttpResponse(null, { status: 500 })))

	await expect(chooseLocale('es-ES')).rejects.toThrow(/500/)
})

test('reports a refused language', async () => {
	server.use(
		http.patch('/api/locale', () =>
			HttpResponse.json({ error: 'content: locale unknown', code: 'locale_unknown' }, { status: 422 }),
		),
	)

	await expect(chooseLocale('xx-XX')).rejects.toThrow(/locale/)
})

test('reads the language the site chose for itself', async () => {
	server.use(http.get('/api/settings', () => HttpResponse.json({ locale_default: 'es-ES' })))

	expect(await fetchSiteLocale()).toBe('es-ES')
})

test('reads no site language when the site chose none', async () => {
	server.use(http.get('/api/settings', () => HttpResponse.json({ locale_default: '' })))

	expect(await fetchSiteLocale()).toBe('')
})

test('stores the language the site chose for itself', async () => {
	const sent: string[] = []
	server.use(
		http.patch('/api/settings', async ({ request }) => {
			sent.push(JSON.stringify(await request.json()))
			return HttpResponse.json({ locale_default: 'es-ES' })
		}),
	)

	await chooseSiteLocale('es-ES')

	expect(sent[0]).toBe('{"locale_default":"es-ES"}')
})

test('reads no site language when the setting cannot be read', async () => {
	server.use(http.get('/api/settings', () => HttpResponse.error()))

	expect(await fetchSiteLocale()).toBe('')
})

test('reads no site language when the server refuses the setting', async () => {
	server.use(http.get('/api/settings', () => HttpResponse.json({}, { status: 500 })))

	expect(await fetchSiteLocale()).toBe('')
})

test('reports a refused site language', async () => {
	server.use(
		http.patch('/api/settings', () =>
			HttpResponse.json({ error: 'content: locale unknown' }, { status: 422 }),
		),
	)

	await expect(chooseSiteLocale('xx-XX')).rejects.toThrow(/locale/)
})
