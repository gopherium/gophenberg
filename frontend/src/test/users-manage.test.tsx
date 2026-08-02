// SPDX-License-Identifier: Apache-2.0

import { http, HttpResponse, server } from '@gophenberg/frontend-sdk/testing'
import { EmailTakenError, ValidationError } from '@gopherium/react-auth/admin'
import { screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { beforeEach, expect, test, vi } from 'vitest'

import { createFailureMessage } from '../users/NewUserScreen'
import { renderAt } from './render'

const SIGNED_IN = '0198b2f0-0000-7000-8000-000000000001'

const OTHER = '0198b2f0-0000-7000-8000-000000000002'

/**
 * Returns a stored account row.
 * @param id - The account identifier.
 * @param name - The account holder's name.
 * @param disabled - Whether the account is shut out.
 * @returns The account row.
 */
function account(id: string, name: string, disabled: boolean) {
	return {
		id,
		email: `${name.split(' ')[0]?.toLowerCase()}@example.com`,
		name,
		disabled,
		created_at: '2026-07-06T10:00:00Z',
	}
}

beforeEach(() =>
	server.use(
		http.get('/api/users', () =>
			HttpResponse.json([account(OTHER, 'Maria Perez', false)]),
		),
	),
)

test('shuts an account out at the reader s request', async () => {
	const asked: unknown[] = []
	server.use(
		http.patch(`/api/users/${OTHER}`, async ({ request }) => {
			asked.push(await request.json())
			return new HttpResponse(null, { status: 204 })
		}),
	)
	renderAt('/users')

	await userEvent.click(await screen.findByRole('button', { name: 'Disable Maria Perez' }))

	await waitFor(() => expect(asked).toHaveLength(1))
	expect(asked[0]).toMatchObject({ disabled: true })
})

test('says so when an account could not be changed', async () => {
	vi.spyOn(console, 'error').mockImplementation(() => {})
	server.use(http.patch(`/api/users/${OTHER}`, () => HttpResponse.json({}, { status: 500 })))
	renderAt('/users')

	await userEvent.click(await screen.findByRole('button', { name: 'Disable Maria Perez' }))

	expect(await screen.findByRole('alert')).toHaveTextContent('Update failed.')
})

test('offers to let a shut out account back in', async () => {
	server.use(
		http.get('/api/users', () => HttpResponse.json([account(OTHER, 'Maria Perez', true)])),
	)
	renderAt('/users')

	const row = await screen.findByRole('row', { name: /Maria Perez/ })

	expect(within(row).getByRole('button', { name: 'Enable Maria Perez' })).toBeInTheDocument()
	expect(within(row).getByText('Disabled')).toBeInTheDocument()
})

test('leaves the signed in account without a way to shut itself out', async () => {
	server.use(
		http.get('/api/users', () =>
			HttpResponse.json([account(SIGNED_IN, 'Grace Hopper', false)]),
		),
	)
	renderAt('/users')

	const row = await screen.findByRole('row', { name: /Grace Hopper/ })

	expect(within(row).queryByRole('button')).toBeNull()
})

test('announces a creation the server turned down', async () => {
	vi.spyOn(console, 'error').mockImplementation(() => {})
	server.use(http.post('/api/users', () => HttpResponse.json({}, { status: 500 })))
	renderAt('/users/new')

	await userEvent.type(await screen.findByLabelText('Email'), 'maria@example.com')
	await userEvent.type(screen.getByLabelText('Name'), 'Maria Perez')
	await userEvent.type(screen.getByLabelText('Password'), 'correct horse battery')
	await userEvent.click(screen.getByRole('button', { name: 'Create user' }))

	expect(await screen.findByRole('alert')).toHaveTextContent('The user could not be created.')
})

test('reports why an account could not be created', () => {
	expect(createFailureMessage(new EmailTakenError('taken'))).toBe(
		'That email is already in use.',
	)
	expect(createFailureMessage(new ValidationError('Password is too short.'))).toBe(
		'Password is too short.',
	)
	expect(createFailureMessage(new Error('boom'))).toBe('The user could not be created.')
})
