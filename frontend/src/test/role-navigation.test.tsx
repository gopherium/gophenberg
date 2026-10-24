// SPDX-License-Identifier: Apache-2.0

import { CHANGE_OTHERS_WORK, MANAGE_THEMES, UPDATE_CORE } from '@gophenberg/frontend-sdk'
import { http, HttpResponse, server } from '@gophenberg/frontend-sdk/testing'
import { defaultUser } from '@gopherium/react-auth/testing'
import { screen, waitFor } from '@testing-library/react'
import { beforeEach, expect, test } from 'vitest'

import { adminCapabilities, renderAt } from './render'
import { warmPostsScreen } from './warm'

warmPostsScreen()

/** The gated screens, each behind the capability it asks for. */
const GATED = ['Users', 'Themes', 'Content Types', 'Settings', 'Updates']

/** The screens every signed in role reaches. */
const OPEN = ['Media', 'Language']

/**
 * Returns the seeded account holding the given role and the capabilities it carries.
 * @param role - The role the account holds.
 * @param capabilities - The capabilities the role carries.
 * @returns The account.
 */
function holding(role: string, capabilities: string[]) {
	return { ...defaultUser, role, capabilities }
}

beforeEach(() =>
	server.use(
		http.get('/api/users', () => HttpResponse.json([])),
		http.get('/api/themes', () => HttpResponse.json({ themes: [] })),
		http.get('/api/content', () => HttpResponse.json({ items: [], total: 0 })),
	),
)

test('hides the screens an author cannot reach', async () => {
	renderAt('/', holding('author', []))

	await screen.findByRole('link', { name: 'Media' })

	for (const label of GATED) {
		expect(screen.queryByRole('link', { name: label })).toBeNull()
	}
	for (const label of OPEN) {
		expect(screen.getByRole('link', { name: label })).toBeInTheDocument()
	}
})

test('hides the screens an editor cannot reach', async () => {
	renderAt('/', holding('editor', [CHANGE_OTHERS_WORK]))

	await screen.findByRole('link', { name: 'Media' })

	for (const label of GATED) {
		expect(screen.queryByRole('link', { name: label })).toBeNull()
	}
})

test('shows every screen to an administrator', async () => {
	renderAt('/', holding('admin', adminCapabilities))

	await screen.findByRole('link', { name: 'Media' })

	for (const label of [...GATED, ...OPEN]) {
		expect(screen.getByRole('link', { name: label })).toBeInTheDocument()
	}
})

test('hides every gated screen from a role named admin that carries no capability', async () => {
	renderAt('/', holding('admin', []))

	await screen.findByRole('link', { name: 'Media' })

	for (const label of GATED) {
		expect(screen.queryByRole('link', { name: label })).toBeNull()
	}
})

test('shows a role carrying only the themes capability the themes screen alone', async () => {
	renderAt('/', holding('designer', [MANAGE_THEMES]))

	await screen.findByRole('link', { name: 'Media' })

	expect(screen.getByRole('link', { name: 'Themes' })).toBeInTheDocument()
	for (const label of GATED.filter((gated) => gated !== 'Themes')) {
		expect(screen.queryByRole('link', { name: label })).toBeNull()
	}
})

test('leaves a role carrying only the themes capability on the themes screen it asked for', async () => {
	renderAt('/themes', holding('designer', [MANAGE_THEMES]))

	expect(await screen.findByRole('heading', { name: 'Themes' })).toBeInTheDocument()
})

test('answers a deep link into a gated screen by opening the default list', async () => {
	renderAt('/users', holding('author', []))

	expect(await screen.findByRole('heading', { name: 'Posts', level: 2 })).toBeInTheDocument()
	await waitFor(() => expect(screen.queryByRole('heading', { name: 'Users' })).toBeNull())
})

test('returns a role named admin that carries no capability from a deep link the same way', async () => {
	renderAt('/users', holding('admin', []))

	expect(await screen.findByRole('heading', { name: 'Posts', level: 2 })).toBeInTheDocument()
	await waitFor(() => expect(screen.queryByRole('heading', { name: 'Users' })).toBeNull())
})

test('answers a deep link into the new user form the same way', async () => {
	renderAt('/users/new', holding('editor', [CHANGE_OTHERS_WORK]))

	expect(await screen.findByRole('heading', { name: 'Posts', level: 2 })).toBeInTheDocument()
})

test('leaves an administrator on the gated screen it asked for', async () => {
	renderAt('/users', holding('admin', adminCapabilities))

	expect(await screen.findByRole('heading', { name: 'Users' })).toBeInTheDocument()
})

test('shows a role carrying only the update capability the Updates screen alone', async () => {
	renderAt('/', holding('maintainer', [UPDATE_CORE]))

	await screen.findByRole('link', { name: 'Media' })

	expect(screen.getByRole('link', { name: 'Updates' })).toHaveAttribute('href', '/admin/update-core')
	for (const label of GATED.filter((gated) => gated !== 'Updates')) {
		expect(screen.queryByRole('link', { name: label })).toBeNull()
	}
})

test('leaves a role carrying the update capability on the Updates screen it asked for', async () => {
	renderAt('/update-core', holding('maintainer', [UPDATE_CORE]))

	expect(await screen.findByRole('heading', { level: 2, name: 'Gophenberg Updates' })).toBeInTheDocument()
})

test('answers a deep link into the Updates screen without update_core by opening the default list', async () => {
	renderAt('/update-core', holding('editor', [CHANGE_OTHERS_WORK]))

	expect(await screen.findByRole('heading', { name: 'Posts', level: 2 })).toBeInTheDocument()
	await waitFor(() => expect(screen.queryByRole('heading', { name: 'Gophenberg Updates' })).toBeNull())
})
