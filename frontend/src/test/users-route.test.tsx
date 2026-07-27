// SPDX-License-Identifier: Apache-2.0

import { http, HttpResponse, server } from '@gophenberg/frontend-sdk/testing'
import { screen, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { beforeEach, expect, test } from 'vitest'

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
