// SPDX-License-Identifier: Apache-2.0

import { http, server } from '@gophenberg/frontend-sdk/testing'
import { listSearch } from '@gopherium/godmin/router'
import { accountWithRole, usersFailure } from '@gopherium/react-auth/testing'
import { screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { beforeEach, expect, test } from 'vitest'

import { createAppRouter } from '../router'
import { renderAt, renderRoutedAt } from './render'
import { EDITOR, rowOf, serveAccounts, warmUsersScreen } from './usersFixtures'

const AUTHOR = accountWithRole('author')

warmUsersScreen()

beforeEach(() => serveAccounts(EDITOR, AUTHOR))

test('serves the users list at /users', async () => {
	renderAt('/users')

	expect(await screen.findByRole('heading', { level: 2, name: 'Users' })).toBeInTheDocument()
	expect((await rowOf(EDITOR.name)).getByText(EDITOR.email)).toBeInTheDocument()
})

test('navigates to the users list from the main menu', async () => {
	renderAt('/')

	await userEvent.click(await screen.findByRole('link', { name: 'Users' }))

	expect(await screen.findByRole('heading', { name: 'Users' })).toBeInTheDocument()
})

test('keeps the heading and the search while the accounts are on their way', async () => {
	server.use(http.get('/api/users', () => new Promise(() => {})))
	renderAt('/users')

	const main = await screen.findByRole('main')

	expect(await within(main).findByRole('heading', { level: 2 })).toHaveTextContent('Users')
	expect(await within(main).findByRole('searchbox', { name: 'Search users…' })).toBeInTheDocument()
	expect(within(main).queryByText(EDITOR.name)).toBeNull()
})

test('keeps the heading alone when the accounts cannot be loaded', async () => {
	server.use(usersFailure())
	renderAt('/users')

	expect(await screen.findByText('Users could not be loaded.')).toBeInTheDocument()
	const main = screen.getByRole('main')
	expect(within(main).getByRole('heading', { level: 2 })).toHaveTextContent('Users')
	expect(within(main).getAllByRole('heading', { level: 2 })).toHaveLength(1)
})

test('reads the list view the address holds the way every other list does', () => {
	const users = Object.values(createAppRouter().routesById).find((route) => route.fullPath === '/users')

	expect(users?.options.validateSearch).toBe(listSearch)
})

test('opens on the search the address holds', async () => {
	renderAt('/users?search=author')

	expect(await rowOf(AUTHOR.name)).toBeDefined()
	expect(screen.queryByText(EDITOR.name)).toBeNull()
})

test('writes the search a reader types into the address and narrows the list to it', async () => {
	const { router } = renderRoutedAt('/users')
	await rowOf(EDITOR.name)

	await userEvent.type(screen.getByRole('searchbox', { name: 'Search users…' }), 'author')

	await waitFor(() => expect(router.state.location.search).toMatchObject({ search: 'author' }))
	await waitFor(() => expect(screen.queryByText(EDITOR.name)).toBeNull())
})
