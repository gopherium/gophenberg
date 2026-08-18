// SPDX-License-Identifier: Apache-2.0

import { http, HttpResponse, server } from '@gophenberg/frontend-sdk/testing'
import { __, getLocaleData, resetLocaleData } from '@wordpress/i18n'
import { beforeEach, expect, test } from 'vitest'

import { usersNavItem } from '@gopherium/react-auth/wpds'

import { displayLocale } from '../i18n/display'
import { DOMAIN, startLocale } from '../i18n/start'

beforeEach(() => {
	resetLocaleData({}, DOMAIN)
	resetLocaleData({})
})

test('answers the language the server resolved', async () => {
	server.use(
		http.get('/api/locale', () =>
			HttpResponse.json({ locale: 'es-ES', supported: ['en-US', 'es-ES'] }),
		),
	)

	expect(await startLocale()).toBe('es-ES')
})

test('remembers the language so dates and numbers follow it too', async () => {
	server.use(
		http.get('/api/locale', () =>
			HttpResponse.json({ locale: 'es-ES', supported: ['en-US', 'es-ES'] }),
		),
	)

	await startLocale()

	expect(displayLocale()).toBe('es-ES')
})

test('loads the catalogue before it returns, so the editor reads it', async () => {
	server.use(
		http.get('/api/locale', () =>
			HttpResponse.json({ locale: 'es-ES', supported: ['en-US', 'es-ES'] }),
		),
	)

	await startLocale()

	expect(getLocaleData(DOMAIN)).toHaveProperty('')
})

test('loads no catalogue for the language the sources are written in', async () => {
	server.use(
		http.get('/api/locale', () =>
			HttpResponse.json({ locale: 'en-US', supported: ['en-US'] }),
		),
	)

	expect(await startLocale()).toBe('en-US')
	expect(__('Add new', DOMAIN)).toBe('Add new')
})

test('loads the editor catalogue beside its own when a build produced one', async () => {
	server.use(
		http.get('/api/locale', () =>
			HttpResponse.json({ locale: 'es-ES', supported: ['en-US', 'es-ES'] }),
		),
	)
	const editor = { '': { 'plural-forms': 'nplurals=2; plural=(n != 1);' }, Bold: ['Negrita'] }

	await startLocale({
		own: async () => undefined,
		editor: async () => editor,
		brick: async () => undefined,
	})

	expect(__('Bold')).toBe('Negrita')
})

test('loads nothing when a build produced no catalogue at all', async () => {
	server.use(
		http.get('/api/locale', () =>
			HttpResponse.json({ locale: 'es-ES', supported: ['en-US', 'es-ES'] }),
		),
	)

	const held = await startLocale({
		own: async () => undefined,
		editor: async () => undefined,
		brick: async () => undefined,
	})

	expect(held).toBe('es-ES')
})

test('loads the brick catalogue so its screens speak the same language', async () => {
	server.use(
		http.get('/api/locale', () =>
			HttpResponse.json({ locale: 'es-ES', supported: ['en-US', 'es-ES'] }),
		),
	)

	await startLocale()

	expect(usersNavItem.label).toBe('Usuarios')
})
