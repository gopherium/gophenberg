// SPDX-License-Identifier: Apache-2.0

import { http, HttpResponse, server } from '@gophenberg/frontend-sdk/testing'
import { screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { beforeAll, beforeEach, expect, test } from 'vitest'

import { adminUser, renderAt } from './render'

const PATH = '/language'

beforeAll(async () => {
	await import('../i18n/LanguageScreen')
}, 120000)

beforeEach(() => {
	server.use(
		http.get('/api/locale', () =>
			HttpResponse.json({ locale: 'en-US', supported: ['en-US', 'es-ES'] }),
		),
		http.get('/api/settings', () => HttpResponse.json({ locale_default: '' })),
	)
})

test('offers every language the site answers in', async () => {
	renderAt(PATH)

	await userEvent.click(await screen.findByLabelText('Your language'))

	expect(await screen.findByRole('option', { name: 'es-ES' })).toBeInTheDocument()
})

test('stores the language a reader chose', async () => {
	const sent: string[] = []
	server.use(
		http.patch('/api/locale', async ({ request }) => {
			sent.push(JSON.stringify(await request.json()))
			return HttpResponse.json({ locale: 'es-ES', supported: ['en-US', 'es-ES'] })
		}),
	)
	renderAt(PATH)

	await userEvent.click(await screen.findByLabelText('Your language'))
	await userEvent.click(await screen.findByRole('option', { name: 'es-ES' }))

	await waitFor(() => expect(sent[0]).toBe('{"locale":"es-ES"}'))
})

test('stores the language the site answers in', async () => {
	const sent: string[] = []
	server.use(
		http.patch('/api/settings', async ({ request }) => {
			sent.push(JSON.stringify(await request.json()))
			return HttpResponse.json({ locale_default: 'es-ES' })
		}),
	)
	renderAt(PATH)

	await userEvent.click(await screen.findByLabelText('The site default'))
	await userEvent.click(await screen.findByRole('option', { name: 'es-ES' }))

	await waitFor(() => expect(sent[0]).toBe('{"locale_default":"es-ES"}'))
})

test('reports a refusal rather than failing quietly', async () => {
	server.use(
		http.patch('/api/locale', () =>
			HttpResponse.json({ error: 'content: locale unknown' }, { status: 422 }),
		),
	)
	renderAt(PATH)

	await userEvent.click(await screen.findByLabelText('Your language'))
	await userEvent.click(await screen.findByRole('option', { name: 'es-ES' }))

	expect(await screen.findByRole('alert')).toHaveTextContent(/locale unknown/)
})

test('reports a refused site default rather than failing quietly', async () => {
	server.use(
		http.patch('/api/settings', () =>
			HttpResponse.json({ error: 'content: locale unknown' }, { status: 422 }),
		),
	)
	renderAt(PATH)

	await userEvent.click(await screen.findByLabelText('The site default'))
	await userEvent.click(await screen.findByRole('option', { name: 'es-ES' }))

	expect(await screen.findByRole('alert')).toHaveTextContent(/locale unknown/)
})

test('keeps the site default in an administrator s hands alone', async () => {
	renderAt(PATH, { ...adminUser, rank: 'author' })

	expect(await screen.findByLabelText('Your language')).toBeInTheDocument()
	expect(screen.queryByLabelText('The site default')).toBeNull()
})
