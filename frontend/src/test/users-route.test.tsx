// SPDX-License-Identifier: Apache-2.0

import { http, HttpResponse, server } from '@gophenberg/frontend-sdk/testing'
import { screen, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { beforeEach, expect, test, vi } from 'vitest'

import { renderAt } from './render'

beforeEach(() =>
	server.use(
		http.get('/api/users', () =>
			HttpResponse.json([
				{
					id: '0198b2f0-0000-7000-8000-000000000001',
					email: 'grace@example.com',
					name: 'Grace Hopper',
					disabled: false,
					created_at: '2026-07-06T10:00:00Z',
				},
			]),
		),
	),
)

test('keeps the users heading while the accounts are on their way', async () => {
	server.use(http.get('/api/users', () => new Promise(() => {})))
	renderAt('/users')

	const main = await screen.findByRole('main')

	expect(await within(main).findByRole('heading', { level: 1 })).toHaveTextContent('Users')
	const status = await within(main).findByRole('status')
	expect(status).toHaveTextContent('Loading users.')
	expect(status.closest('.godmin-loading-rows')).not.toBeNull()
})

test('fades the account table in when it replaces the ghost', async () => {
	renderAt('/users')

	const region = await screen.findByRole('region', { name: 'Users' })
	expect([...region.classList]).toContain('godmin-arrival')
})

test('keeps the users heading when the accounts cannot be loaded', async () => {
	vi.spyOn(console, 'error').mockImplementation(() => {})
	server.use(http.get('/api/users', () => HttpResponse.json({}, { status: 500 })))
	renderAt('/users')

	const alert = await screen.findByRole('alert')

	expect(alert).toHaveTextContent(/could not be loaded/i)
	const main = screen.getByRole('main')
	expect(within(main).getByRole('heading', { level: 1 })).toHaveTextContent('Users')
	expect(within(main).getAllByRole('heading', { level: 1 })).toHaveLength(1)
})

test('names the new user screen with its only first level heading', async () => {
	renderAt('/users/new')

	const main = await screen.findByRole('main')

	expect(within(main).getByRole('heading', { level: 1 })).toHaveTextContent('New user')
	expect(within(main).getAllByRole('heading', { level: 1 })).toHaveLength(1)
})

test('serves the users screen at /users', async () => {
	renderAt('/users')

	expect(
		await screen.findByRole('heading', { name: 'Users' }),
	).toBeInTheDocument()
	expect(
		within(await screen.findByRole('row', { name: /Grace Hopper/ })).getByText(
			'grace@example.com',
		),
	).toBeInTheDocument()
})

test('navigates to the users screen from the main menu', async () => {
	renderAt('/')

	await userEvent.click(
		await screen.findByRole('link', { name: 'Users' }),
	)

	expect(
		await screen.findByRole('heading', { name: 'Users' }),
	).toBeInTheDocument()
})

test('returns to the user list after creating a user', async () => {
	server.use(
		http.post('/api/users', () =>
			HttpResponse.json(
				{
					id: '0198b2f0-0000-7000-8000-000000000002',
					email: 'ada@example.com',
					name: 'Ada Lovelace',
					disabled: false,
					created_at: '2026-07-16T10:00:00Z',
				},
				{ status: 201 },
			),
		),
	)
	renderAt('/users/new')

	await userEvent.type(
		await screen.findByLabelText('Email'),
		'ada@example.com',
	)
	await userEvent.type(screen.getByLabelText('Name'), 'Ada Lovelace')
	await userEvent.type(
		screen.getByLabelText('Password'),
		'correct horse battery',
	)
	await userEvent.click(screen.getByRole('button', { name: 'Create user' }))

	expect(
		await screen.findByRole('heading', { name: 'Users' }),
	).toBeInTheDocument()
})
