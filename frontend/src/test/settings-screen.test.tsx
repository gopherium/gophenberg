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

test('stores nothing while a box stands empty', async () => {
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

	expect(await screen.findByRole('alert')).toHaveTextContent('whole number')
	expect(sent).toHaveLength(0)
})

test('stores nothing while a quality box stands empty', async () => {
	const sent: string[] = []
	server.use(
		http.patch('/api/settings', async ({ request }) => {
			sent.push(JSON.stringify(await request.json()))
			return HttpResponse.json({ locale_default: '', content_per_page: 20, jpeg_quality: 82 })
		}),
	)
	renderAt(PATH, adminUser)

	await userEvent.clear(await screen.findByLabelText('Picture quality'))
	await userEvent.click(screen.getByRole('button', { name: 'Save' }))

	expect(await screen.findByRole('alert')).toHaveTextContent('whole number')
	expect(sent).toHaveLength(0)
})

test('stores nothing when a page size is not a whole number', async () => {
	const sent: string[] = []
	server.use(
		http.patch('/api/settings', async ({ request }) => {
			sent.push(JSON.stringify(await request.json()))
			return HttpResponse.json({ locale_default: '', content_per_page: 20, jpeg_quality: 82 })
		}),
	)
	renderAt(PATH, adminUser)

	const field = await screen.findByLabelText('Posts per page')
	await userEvent.clear(field)
	await userEvent.type(field, '2.5')
	await userEvent.click(screen.getByRole('button', { name: 'Save' }))

	expect(await screen.findByRole('alert')).toHaveTextContent('whole number')
	expect(sent).toHaveLength(0)
})

test('sends a number outside the range on, so the server names the range', async () => {
	const sent: string[] = []
	server.use(
		http.patch('/api/settings', async ({ request }) => {
			sent.push(JSON.stringify(await request.json()))
			return HttpResponse.json(
				{ error: 'per page invalid', code: 'per_page_invalid', meta: { value: '500', max: 100 } },
				{ status: 422 },
			)
		}),
	)
	renderAt(PATH, adminUser)

	const field = await screen.findByLabelText('Posts per page')
	await userEvent.clear(field)
	await userEvent.type(field, '500')
	await userEvent.click(screen.getByRole('button', { name: 'Save' }))

	await waitFor(() => expect(sent[0]).toContain('"content_per_page":500'))
})

test('offers each box the bounds the server keeps', async () => {
	renderAt(PATH, adminUser)

	const size = await screen.findByLabelText('Posts per page')
	expect(size).toHaveAttribute('min', '1')
	expect(size).toHaveAttribute('max', '100')
	expect(size).toHaveAttribute('step', '1')
	expect(await screen.findByLabelText('Picture quality')).toHaveAttribute('max', '100')
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
