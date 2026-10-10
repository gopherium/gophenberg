// SPDX-License-Identifier: Apache-2.0

import { http, HttpResponse, server } from '@gophenberg/frontend-sdk/testing'
import { dateI18n, getDate, getSettings, setSettings } from '@wordpress/date'
import { __, getLocaleData, resetLocaleData } from '@wordpress/i18n'
import { afterEach, beforeEach, expect, test } from 'vitest'

import { usersNavItem } from '@gopherium/react-auth/wpds'

import { displayLocale } from '@gopherium/gottext'
import { DOMAIN, startLocale } from '../i18n/start'

/** The date settings in place before any test boots a language. */
const DATE_DEFAULTS = getSettings()

beforeEach(() => {
	resetLocaleData({}, DOMAIN)
	resetLocaleData({})
})

afterEach(() => {
	setSettings(DATE_DEFAULTS)
	process.env.TZ = 'UTC'
})

/**
 * Answers the language the server resolves.
 * @param locale - The language to answer.
 */
function serveLocale(locale: string) {
	server.use(http.get('/api/locale', () => HttpResponse.json({ locale, supported: ['en-US', locale] })))
}

test('hands the dates the month and weekday names of the reader language and its first weekday', async () => {
	serveLocale('es-ES')

	await startLocale()

	expect(getSettings().l10n).toMatchObject({
		locale: 'es-ES',
		months: ['enero', 'febrero', 'marzo', 'abril', 'mayo', 'junio', 'julio', 'agosto', 'septiembre', 'octubre',
			'noviembre', 'diciembre'],
		monthsShort: ['Ene', 'Feb', 'Mar', 'Abr', 'May', 'Jun', 'Jul', 'Ago', 'Sep', 'Oct', 'Nov', 'Dic'],
		weekdays: ['domingo', 'lunes', 'martes', 'miércoles', 'jueves', 'viernes', 'sábado'],
		weekdaysShort: ['Dom', 'Lun', 'Mar', 'Mié', 'Jue', 'Vie', 'Sáb'],
		startOfWeek: 1,
	})
})

test('hands the dates the words the sources are written in, the weeks starting on Monday', async () => {
	serveLocale('en-US')

	await startLocale()

	expect(getSettings().l10n).toMatchObject({
		locale: 'en-US',
		months: ['January', 'February', 'March', 'April', 'May', 'June', 'July', 'August', 'September', 'October',
			'November', 'December'],
		monthsShort: ['Jan', 'Feb', 'Mar', 'Apr', 'May', 'Jun', 'Jul', 'Aug', 'Sep', 'Oct', 'Nov', 'Dec'],
		weekdays: ['Sunday', 'Monday', 'Tuesday', 'Wednesday', 'Thursday', 'Friday', 'Saturday'],
		weekdaysShort: ['Sun', 'Mon', 'Tue', 'Wed', 'Thu', 'Fri', 'Sat'],
		startOfWeek: 1,
	})
})

test.each([
	{ reader: 'es-ES', abbreviated: 'j M Y H:i', full: 'j \\d\\e F \\d\\e Y H:i' },
	{ reader: 'fr-FR', abbreviated: 'd F Y G\\hi', full: 'd F Y G\\hi' },
	{ reader: 'en-US', abbreviated: 'M j, Y g:i a', full: 'F j, Y g:i a' },
])('hands the dates the abbreviated and the full date and time formats of $reader', async (formats) => {
	serveLocale(formats.reader)

	await startLocale()

	expect(getSettings().formats).toMatchObject({ datetimeAbbreviated: formats.abbreviated, datetime: formats.full })
})

test.each([
	{ named: '0', first: 0 },
	{ named: '6', first: 6 },
	{ named: 'lunes', first: 1 },
])('starts the weeks on day $first when the catalogue names $named as the first day', async ({ named, first }) => {
	serveLocale('es-ES')
	const own = { '': { 'plural-forms': 'nplurals=2; plural=(n != 1);' }, 'start of week\u00041': [named] }
	const none = async () => undefined

	await startLocale({ own: async () => own, editor: none, brick: none, chrome: none })

	expect(getSettings().l10n.startOfWeek).toBe(first)
})

test('writes and reads the moments of the dates on the reader clock, the clock the date cells use', async () => {
	process.env.TZ = 'Europe/Paris'
	serveLocale('en-US')
	const summer = '2026-07-20T10:00:00.000Z'

	await startLocale()

	expect(dateI18n('H:i', summer)).toBe(new Date(summer).toTimeString().slice(0, 5))
	expect(getDate('2026-01-15T09:30').toISOString()).toBe(new Date('2026-01-15T09:30').toISOString())
	expect(getSettings().timezone).toMatchObject({
		string: Intl.DateTimeFormat().resolvedOptions().timeZone,
		offset: 0 - new Date().getTimezoneOffset() / 60,
	})
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
		chrome: async () => undefined,
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
		chrome: async () => undefined,
	})

	expect(held).toBe('es-ES')
})

test('loads the list chrome so the list controls speak the reader language', async () => {
	server.use(
		http.get('/api/locale', () =>
			HttpResponse.json({ locale: 'es-ES', supported: ['en-US', 'es-ES'] }),
		),
	)

	await startLocale()

	expect(__('Add filter')).toBe('Añadir filtro')
})

test('reads the list chrome words where the editor catalogue holds the same message', async () => {
	server.use(
		http.get('/api/locale', () =>
			HttpResponse.json({ locale: 'es-ES', supported: ['en-US', 'es-ES'] }),
		),
	)
	const editor = { '': { 'plural-forms': 'nplurals=2; plural=(n != 1);' }, Close: ['Cerrar el editor'] }
	const chrome = { '': { 'plural-forms': 'nplurals=2; plural=(n != 1);' }, Close: ['Cerrar'] }

	await startLocale({
		own: async () => undefined,
		editor: async () => editor,
		brick: async () => undefined,
		chrome: async () => chrome,
	})

	expect(__('Close')).toBe('Cerrar')
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
