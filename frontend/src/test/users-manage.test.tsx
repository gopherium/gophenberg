// SPDX-License-Identifier: Apache-2.0

import { http, HttpResponse, server } from '@gophenberg/frontend-sdk/testing'
import { DOMAIN as AUTH_DOMAIN, catalogFor as authCatalogFor } from '@gopherium/react-auth'
import { accountWithRole, disabledRefused } from '@gopherium/react-auth/testing'
import { badgeClasses } from '@gopherium/godmin/testing'
import { screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { resetLocaleData, setLocaleData } from '@wordpress/i18n'
import { beforeEach, expect, onTestFinished, test } from 'vitest'

import { adminBasepath } from '../basepath'
import { renderAt } from './render'
import { siteSettings } from './siteSettings'
import {
	EDITOR,
	REQUEST_CROSS_ORIGIN,
	REQUEST_CROSS_ORIGIN_WORDS,
	SIGNED_IN,
	act,
	rowOf,
	rowShowing,
	serveAccounts,
	toast,
	warmUsersScreen,
} from './usersFixtures'

const AUTHOR = accountWithRole('author', { created_at: new Date('2026-10-20T09:00:00Z') })

const MEMBER = accountWithRole('member', { created_at: new Date('2026-10-20T10:00:00Z') })

const PICTURED = { ...EDITOR, avatar: 'https://example.com/avatar/editor.png' }

warmUsersScreen()

beforeEach(() => serveAccounts(SIGNED_IN, EDITOR))

/**
 * Serves the list settings with the given values changed.
 * @param changes - The settings a test sets apart from the defaults.
 */
function serveSettings(changes: Record<string, unknown>) {
	server.use(http.get('/api/settings', () => HttpResponse.json({ ...siteSettings, ...changes })))
}

/**
 * Records every Enable or Disable the list sends, by account id.
 * @returns The writes, each the account id beside the body sent.
 */
function recordSwitches(): unknown[] {
	const sent: unknown[] = []
	server.use(
		http.patch('/api/users/:id', async ({ params, request }) => {
			sent.push({ id: params.id, ...((await request.json()) as object) })
			return new HttpResponse(null, { status: 204 })
		}),
	)
	return sent
}

/**
 * Returns the page sizes View options offers, opening it first.
 * @returns The sizes as the radio buttons read.
 */
async function offeredSizes() {
	await userEvent.click(await screen.findByRole('button', { name: 'View options' }))
	return within(await screen.findByRole('radiogroup', { name: 'Items per page' }))
}

test('draws Add User in the header as a link to the add user screen', async () => {
	renderAt('/users')

	const add = await within(await screen.findByRole('main')).findByRole('link', { name: 'Add User' })

	expect(add).toHaveAttribute('href', `${adminBasepath}/users/new`)
})

test('names the header button of the users list as WordPress es_ES does for a Spanish reader', async () => {
	setLocaleData(await authCatalogFor('es-ES'), AUTH_DOMAIN)
	onTestFinished(() => resetLocaleData({}, AUTH_DOMAIN))
	renderAt('/users')

	const main = within(await screen.findByRole('main'))

	expect(await main.findByRole('link', { name: 'Añadir usuario' })).toBeInTheDocument()
})

test('disables an account from its row menu and names it in a toast', async () => {
	const sent = recordSwitches()
	renderAt('/users')

	await act(EDITOR.name, 'Disable')

	expect(await toast(`"${EDITOR.name}" disabled.`)).toBeInTheDocument()
	expect(sent).toEqual([{ id: EDITOR.id, disabled: true }])
})

test('says in a toast why the server refused to disable an account', async () => {
	server.use(disabledRefused('last_privileged_refused'))
	renderAt('/users')

	await act(EDITOR.name, 'Disable')

	expect(await toast('Somebody must be able to manage users.')).toBeInTheDocument()
})

test('says in a toast in the admin s words that a Disable came from a page on another site', async () => {
	server.use(http.patch('/api/users/:id', () => HttpResponse.json(REQUEST_CROSS_ORIGIN, { status: 403 })))
	renderAt('/users')

	await act(EDITOR.name, 'Disable')

	expect(await toast(REQUEST_CROSS_ORIGIN_WORDS)).toBeInTheDocument()
})

test('offers Enable on a disabled account, drawn with the outline badge of an inactive state', async () => {
	serveAccounts(SIGNED_IN, { ...EDITOR, disabled: true })
	const sent = recordSwitches()
	renderAt('/users')

	const row = await rowOf(EDITOR.name)
	expect([...row.getByText('Disabled').classList]).toEqual(badgeClasses('none'))
	await act(EDITOR.name, 'Enable')

	await waitFor(() => expect(sent).toEqual([{ id: EDITOR.id, disabled: false }]))
})

test('leaves the signed in account without an action', async () => {
	renderAt('/users')

	const row = await rowOf(SIGNED_IN.name)

	expect(row.queryByRole('button', { name: 'Actions' })).toBeNull()
})

test('leaves an account the server marks not editable without an action', async () => {
	serveAccounts(SIGNED_IN, { ...EDITOR, editable: false })
	renderAt('/users')

	const row = await rowOf(EDITOR.name)

	expect(row.queryByRole('button', { name: 'Actions' })).toBeNull()
})

test('draws beside each name the avatar the server sends', async () => {
	serveAccounts(SIGNED_IN, PICTURED)
	renderAt('/users')

	const row = await rowShowing(EDITOR.name)

	expect(row.querySelector('.godmin-avatar__image')).toHaveAttribute('src', PICTURED.avatar)
})

test('draws no avatar when the site shows none', async () => {
	serveSettings({ show_avatars: false })
	serveAccounts(SIGNED_IN, PICTURED)
	renderAt('/users')

	const row = await rowShowing(EDITOR.name)

	expect(row.querySelector('.godmin-avatar')).toBeNull()
})

test('opens newest first with Created left in View options, as WordPress draws no such column', async () => {
	serveAccounts(AUTHOR, MEMBER)
	renderAt('/users')

	await rowOf(MEMBER.name)
	const [, newest, oldest] = screen.getAllByRole('row')

	expect(within(newest).getByText(MEMBER.name)).toBeInTheDocument()
	expect(within(oldest).getByText(AUTHOR.name)).toBeInTheDocument()
	expect(screen.queryByRole('columnheader', { name: /^Created/ })).toBeNull()

	await userEvent.click(screen.getByRole('button', { name: 'View options' }))
	const properties = (await screen.findByText('Properties')).closest('.dataviews-field-control') as HTMLElement
	await userEvent.click(within(properties).getByRole('button', { name: 'Created' }))

	expect(await screen.findByRole('columnheader', { name: /^Created/ })).toBeInTheDocument()
})

test('pages the accounts by the size the settings name', async () => {
	serveSettings({ list_page_size: 2, list_page_sizes: [2, 5] })
	serveAccounts(SIGNED_IN, EDITOR, AUTHOR, MEMBER)
	renderAt('/users')

	await rowOf(MEMBER.name)

	expect(screen.getAllByRole('row')).toHaveLength(3)
	expect(screen.queryByText(EDITOR.name)).toBeNull()
	expect((await offeredSizes()).getAllByRole('radio').map((size) => size.textContent)).toEqual(['2', '5'])
})

test('opens on the page size the settings name for an address naming one they do not offer', async () => {
	renderAt('/users?perPage=7')
	await rowOf(EDITOR.name)

	expect((await offeredSizes()).getByRole('radio', { name: '20' })).toBeChecked()
})

test('opens on the page and the offered page size the address holds', async () => {
	serveSettings({ list_page_size: 5, list_page_sizes: [2, 5] })
	serveAccounts(SIGNED_IN, EDITOR, AUTHOR, MEMBER)
	renderAt('/users?page=2&perPage=2')

	await rowOf(EDITOR.name)

	expect(screen.getAllByRole('row')).toHaveLength(3)
	expect(screen.queryByText(MEMBER.name)).toBeNull()
	expect((await offeredSizes()).getByRole('radio', { name: '2' })).toBeChecked()
})

test('shows every account on one page when the settings cannot be read', async () => {
	server.use(http.get('/api/settings', () => HttpResponse.json({}, { status: 500 })))
	serveAccounts(SIGNED_IN, EDITOR, AUTHOR, MEMBER)
	renderAt('/users')

	await rowOf(MEMBER.name)

	expect(screen.getAllByRole('row')).toHaveLength(5)
})
