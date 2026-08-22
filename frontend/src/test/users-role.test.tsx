// SPDX-License-Identifier: Apache-2.0

import { http, HttpResponse, server } from '@gophenberg/frontend-sdk/testing'
import { screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { beforeEach, expect, test, vi } from 'vitest'

import { roleLabel } from '../users/roles'
import { renderAt } from './render'

const SIGNED_IN = '0198b2f0-0000-7000-8000-000000000001'

const OTHER = '0198b2f0-0000-7000-8000-000000000002'

/**
 * Returns a stored account row holding a role.
 * @param id - The account identifier.
 * @param name - The account holder's name.
 * @param role - The role the account holds.
 * @returns The account row.
 */
function withRole(id: string, name: string, role: string) {
	return {
		id,
		email: `${name.split(' ')[0]?.toLowerCase()}@example.com`,
		name,
		disabled: false,
		role,
		created_at: '2026-07-06T10:00:00Z',
	}
}

beforeEach(() =>
	server.use(
		http.get('/api/users', () => HttpResponse.json([withRole(OTHER, 'Maria Perez', 'editor')])),
	),
)

test('shows the role each account holds', async () => {
	renderAt('/users')

	const row = await screen.findByRole('row', { name: /Maria Perez/ })

	expect(within(row).getByText('Editor')).toBeInTheDocument()
})

test('writes the role the reader picked for another account', async () => {
	const asked: unknown[] = []
	server.use(
		http.put(`/api/users/${OTHER}/role`, async ({ request }) => {
			asked.push(await request.json())
			return new HttpResponse(null, { status: 204 })
		}),
	)
	renderAt('/users')

	await userEvent.click(await screen.findByRole('combobox', { name: 'Role of Maria Perez' }))
	await userEvent.click(await screen.findByRole('option', { name: 'Administrator' }))

	await waitFor(() => expect(asked).toHaveLength(1))
	expect(asked[0]).toMatchObject({ role: 'admin' })
})

test('leaves the signed in account without a way to change its own role', async () => {
	server.use(
		http.get('/api/users', () =>
			HttpResponse.json([withRole(SIGNED_IN, 'Grace Hopper', 'admin')]),
		),
	)
	renderAt('/users')

	const row = await screen.findByRole('row', { name: /Grace Hopper/ })

	expect(within(row).queryByRole('combobox')).toBeNull()
	expect(within(row).getByText('Administrator')).toBeInTheDocument()
})

test('creates an account under the role the form picked', async () => {
	const sent: unknown[] = []
	server.use(
		http.post('/api/users', async ({ request }) => {
			sent.push(await request.json())
			return HttpResponse.json(withRole(OTHER, 'Maria Perez', 'editor'), { status: 201 })
		}),
	)
	renderAt('/users/new')

	await userEvent.type(await screen.findByLabelText('Email'), 'maria@example.com')
	await userEvent.type(screen.getByLabelText('Name'), 'Maria Perez')
	await userEvent.type(screen.getByLabelText('Password'), 'correct horse battery')
	await userEvent.click(screen.getByRole('combobox', { name: 'Role' }))
	await userEvent.click(await screen.findByRole('option', { name: 'Editor' }))
	await userEvent.click(screen.getByRole('button', { name: 'Create user' }))

	await waitFor(() => expect(sent).toHaveLength(1))
	expect(sent[0]).toMatchObject({ role: 'editor' })
})

test('starts a new account at the narrowest role', async () => {
	renderAt('/users/new')

	expect(await screen.findByRole('combobox', { name: 'Role' })).toHaveTextContent('Author')
})

test('says so when a role could not be written', async () => {
	vi.spyOn(console, 'error').mockImplementation(() => {})
	server.use(http.put(`/api/users/${OTHER}/role`, () => HttpResponse.json({}, { status: 500 })))
	renderAt('/users')

	await userEvent.click(await screen.findByRole('combobox', { name: 'Role of Maria Perez' }))
	await userEvent.click(await screen.findByRole('option', { name: 'Administrator' }))

	expect(await screen.findByRole('alert')).toHaveTextContent('Update failed.')
})

test('names the control of an account holding no role', async () => {
	server.use(
		http.get('/api/users', () => HttpResponse.json([withRole(OTHER, 'Maria Perez', '')])),
	)
	renderAt('/users')

	const row = await screen.findByRole('row', { name: /Maria Perez/ })

	expect(within(row).getByText('No role')).toBeInTheDocument()
})

test('reads a role the screen does not know as the server stored it', () => {
	expect(roleLabel('archivist')).toBe('archivist')
	expect(roleLabel('')).toBe('No role')
	expect(roleLabel('editor')).toBe('Editor')
})
