// SPDX-License-Identifier: Apache-2.0

import { http, HttpResponse, server } from '@gophenberg/frontend-sdk/testing'
import { defaultUser } from '@gopherium/react-auth/testing'
import { screen, waitFor } from '@testing-library/react'
import { beforeEach, expect, test } from 'vitest'

import { renderAt } from './render'

/** The gated screens, which only an administrator reaches. */
const GATED = ['Users', 'Themes', 'Content Types']

/** The screens every signed in role reaches. */
const OPEN = ['Media', 'Language']

/**
 * Returns the seeded account holding the given role.
 * @param role - The role the account holds.
 * @returns The account.
 */
function holding(role: string) {
	return { ...defaultUser, role }
}

beforeEach(() =>
	server.use(http.get('/api/users', () => HttpResponse.json([]))),
)

test('hides the screens an author cannot reach', async () => {
	renderAt('/', holding('author'))

	await screen.findByRole('link', { name: 'Media' })

	for (const label of GATED) {
		expect(screen.queryByRole('link', { name: label })).toBeNull()
	}
	for (const label of OPEN) {
		expect(screen.getByRole('link', { name: label })).toBeInTheDocument()
	}
})

test('hides the screens an editor cannot reach', async () => {
	renderAt('/', holding('editor'))

	await screen.findByRole('link', { name: 'Media' })

	for (const label of GATED) {
		expect(screen.queryByRole('link', { name: label })).toBeNull()
	}
})

test('shows every screen to an administrator', async () => {
	renderAt('/', holding('admin'))

	await screen.findByRole('link', { name: 'Media' })

	for (const label of [...GATED, ...OPEN]) {
		expect(screen.getByRole('link', { name: label })).toBeInTheDocument()
	}
})

test('answers a deep link into a gated screen by returning to the dashboard', async () => {
	renderAt('/users', holding('author'))

	expect(await screen.findByRole('heading', { name: 'Home' })).toBeInTheDocument()
	await waitFor(() => expect(screen.queryByRole('heading', { name: 'Users' })).toBeNull())
})

test('answers a deep link into the new user form the same way', async () => {
	renderAt('/users/new', holding('editor'))

	expect(await screen.findByRole('heading', { name: 'Home' })).toBeInTheDocument()
})

test('leaves an administrator on the gated screen it asked for', async () => {
	renderAt('/users', holding('admin'))

	expect(await screen.findByRole('heading', { name: 'Users' })).toBeInTheDocument()
})
