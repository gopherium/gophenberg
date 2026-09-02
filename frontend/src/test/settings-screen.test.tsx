// SPDX-License-Identifier: Apache-2.0

import { http, HttpResponse, server } from '@gophenberg/frontend-sdk/testing'
import { defaultUser } from '@gopherium/react-auth/testing'
import { screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { beforeAll, beforeEach, expect, test } from 'vitest'

import { adminUser, renderAt } from './render'

const PATH = '/settings'

beforeAll(async () => {
	await import('../settings/SettingsScreen')
}, 120000)

beforeEach(() => {
	server.use(
		http.get('/api/settings', () =>
			HttpResponse.json({ locale_default: '', content_per_page: 20, jpeg_quality: 82 }),
		),
	)
})

test('shows what the site chose for itself', async () => {
	renderAt(PATH, adminUser)

	expect(await screen.findByLabelText('Posts per page')).toHaveValue(20)
	expect(await screen.findByLabelText('Picture quality')).toHaveValue(82)
})

test('stores a page size an administrator typed', async () => {
	const sent: string[] = []
	server.use(
		http.patch('/api/settings', async ({ request }) => {
			sent.push(JSON.stringify(await request.json()))
			return HttpResponse.json({ locale_default: '', content_per_page: 5, jpeg_quality: 82 })
		}),
	)
	renderAt(PATH, adminUser)

	const field = await screen.findByLabelText('Posts per page')
	await userEvent.clear(field)
	expect(field).toHaveValue(null)
	await userEvent.type(field, '5')
	await userEvent.click(screen.getByRole('button', { name: 'Save' }))

	await waitFor(() => expect(sent[0]).toContain('"content_per_page":5'))
})

test('stores a picture quality an administrator typed', async () => {
	const sent: string[] = []
	server.use(
		http.patch('/api/settings', async ({ request }) => {
			sent.push(JSON.stringify(await request.json()))
			return HttpResponse.json({ locale_default: '', content_per_page: 20, jpeg_quality: 30 })
		}),
	)
	renderAt(PATH, adminUser)

	const field = await screen.findByLabelText('Picture quality')
	await userEvent.clear(field)
	await userEvent.type(field, '30')
	await userEvent.click(screen.getByRole('button', { name: 'Save' }))

	await waitFor(() => expect(sent[0]).toContain('"jpeg_quality":30'))
})

test('leaves a setting alone when its box was emptied', async () => {
	const sent: string[] = []
	server.use(
		http.patch('/api/settings', async ({ request }) => {
			sent.push(JSON.stringify(await request.json()))
			return HttpResponse.json({ locale_default: '', content_per_page: 20, jpeg_quality: 82 })
		}),
	)
	renderAt(PATH, adminUser)

	await userEvent.clear(await screen.findByLabelText('Posts per page'))
	await userEvent.click(screen.getByRole('button', { name: 'Save' }))

	await waitFor(() => expect(sent[0]).toBe('{"jpeg_quality":82}'))
})

test('says why a refused value was not stored', async () => {
	server.use(
		http.patch('/api/settings', () =>
			HttpResponse.json(
				{ error: 'per page invalid', code: 'per_page_invalid', meta: { value: '500', max: 100 } },
				{ status: 422 },
			),
		),
	)
	renderAt(PATH, adminUser)

	const field = await screen.findByLabelText('Posts per page')
	await userEvent.clear(field)
	await userEvent.type(field, '500')
	await userEvent.click(screen.getByRole('button', { name: 'Save' }))

	expect(await screen.findByRole('alert')).toHaveTextContent(/100/)
})

test('keeps an author out of the settings', async () => {
	renderAt(PATH, { ...defaultUser, role: 'author' })

	await waitFor(() => expect(screen.queryByLabelText('Posts per page')).not.toBeInTheDocument())
})
