// SPDX-License-Identifier: Apache-2.0

import { http, HttpResponse, server } from '@gophenberg/frontend-sdk/testing'
import { DOMAIN as AUTH_DOMAIN, catalogFor as authCatalogFor } from '@gopherium/react-auth'
import { accountWithRole, roleRefused } from '@gopherium/react-auth/testing'
import { screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { resetLocaleData, setLocaleData } from '@wordpress/i18n'
import { beforeEach, expect, onTestFinished, test } from 'vitest'

import { catalogFor } from '../i18n/catalog'
import { roleLabels } from '../users/roles'
import { adminUser, renderAt } from './render'
import {
	EDITOR,
	REQUEST_CROSS_ORIGIN,
	REQUEST_CROSS_ORIGIN_WORDS,
	SIGNED_IN,
	act,
	rowOf,
	serveAccounts,
	toast,
	warmUsersScreen,
} from './usersFixtures'

/** The words the admin gives a write sent from a page on another site, for a Spanish reader. */
const REQUEST_CROSS_ORIGIN_SPANISH =
	'Esta petición vino de una página de otro sitio, así que se rechazó. ' +
	'Abre la administración en este sitio e inténtalo de nuevo.'

warmUsersScreen()

beforeEach(() => serveAccounts(SIGNED_IN, EDITOR))

/**
 * Returns the queries of the open Change role dialog.
 * @returns The queries bound to the dialog.
 */
async function roleDialog() {
	return within(await screen.findByRole('dialog', { name: 'Change role' }))
}

/**
 * Returns the roles the open Change role dialog offers, in the order it lists them.
 * @returns The labels.
 */
async function offeredRoles() {
	await userEvent.click((await roleDialog()).getByRole('combobox', { name: 'Role' }))
	return (await screen.findAllByRole('option')).map((option) => option.textContent)
}

/**
 * Picks a role in the open Change role dialog and sends it.
 * @param label - The label of the role to pick.
 */
async function changeRoleTo(label: string) {
	await userEvent.click((await roleDialog()).getByRole('combobox', { name: 'Role' }))
	await userEvent.click(await screen.findByRole('option', { name: label }))
	await userEvent.click((await roleDialog()).getByRole('button', { name: 'Change role' }))
}

/**
 * Loads the Spanish catalogues of the admin and of the account screens until the test ends.
 */
async function readInSpanish() {
	setLocaleData(await catalogFor('es-ES'), 'gophenberg')
	setLocaleData(await authCatalogFor('es-ES'), AUTH_DOMAIN)
	onTestFinished(() => {
		resetLocaleData({}, 'gophenberg')
		resetLocaleData({}, AUTH_DOMAIN)
	})
}

test('labels each core role as the admin names it', () => {
	expect(roleLabels()).toEqual({ admin: 'Administrator', editor: 'Editor', author: 'Author', member: 'Member' })
})

test('labels the core roles in the reader s language', async () => {
	setLocaleData(await catalogFor('es-ES'), 'gophenberg')
	onTestFinished(() => resetLocaleData({}, 'gophenberg'))

	expect(roleLabels()).toMatchObject({ admin: 'Administrador', member: 'Miembro' })
})

test('shows the label of the role each account holds', async () => {
	renderAt('/users')

	expect((await rowOf(EDITOR.name)).getByText('Editor')).toBeInTheDocument()
	expect((await rowOf(SIGNED_IN.name)).getByText('Administrator')).toBeInTheDocument()
})

test('shows a role a plugin declares as its stored name with a capital first letter', async () => {
	serveAccounts(SIGNED_IN, accountWithRole('archivist'))
	renderAt('/users')

	expect((await rowOf('Maria Perez the archivist')).getByText('Archivist')).toBeInTheDocument()
})

test('shows None for an account holding no role, as WordPress does', async () => {
	serveAccounts(SIGNED_IN, { ...EDITOR, role: '' })
	renderAt('/users')

	expect((await rowOf(EDITOR.name)).getByText('None')).toBeInTheDocument()
})

test('shows the role labels in the reader s language', async () => {
	await readInSpanish()
	renderAt('/users')

	expect((await rowOf(SIGNED_IN.name)).getByText('Administrador')).toBeInTheDocument()
})

/** Refuses every role change as sent from a page on another site. */
function refuseRoleAsCrossOrigin() {
	server.use(http.put('/api/users/:id/role', () => HttpResponse.json(REQUEST_CROSS_ORIGIN, { status: 403 })))
}

test('says in a toast in the admin s words that Change role came from a page on another site', async () => {
	refuseRoleAsCrossOrigin()
	renderAt('/users')

	await act(EDITOR.name, 'Change role…')
	await changeRoleTo('Administrator')

	expect(await toast(REQUEST_CROSS_ORIGIN_WORDS)).toBeInTheDocument()
})

test('says in the reader s language that Change role came from a page on another site', async () => {
	setLocaleData(await catalogFor('es-ES'), 'gophenberg')
	onTestFinished(() => resetLocaleData({}, 'gophenberg'))
	refuseRoleAsCrossOrigin()
	renderAt('/users')

	await act(EDITOR.name, 'Change role…')
	await changeRoleTo('Administrador')

	expect(await toast(REQUEST_CROSS_ORIGIN_SPANISH)).toBeInTheDocument()
})

test('offers in Change role the roles the session may give, the narrowest first', async () => {
	renderAt('/users', { ...adminUser, grantable: ['admin', 'moderator', 'editor', 'author'] })

	await act(EDITOR.name, 'Change role…')

	expect(await offeredRoles()).toEqual(['Author', 'Editor', 'Moderator', 'Administrator'])
})

test('changes the role of an account and names it in a toast', async () => {
	const sent: unknown[] = []
	server.use(
		http.put('/api/users/:id/role', async ({ params, request }) => {
			sent.push({ id: params.id, ...((await request.json()) as object) })
			return new HttpResponse(null, { status: 204 })
		}),
	)
	renderAt('/users')

	await act(EDITOR.name, 'Change role…')
	await changeRoleTo('Administrator')

	expect(await toast(`Role changed for "${EDITOR.name}".`)).toBeInTheDocument()
	expect(sent).toEqual([{ id: EDITOR.id, role: 'admin' }])
	await waitFor(() => expect(screen.queryByRole('dialog')).toBeNull())
})

test('says in WordPress s words why the server refused a role', async () => {
	server.use(roleRefused('role_not_grantable'))
	renderAt('/users')

	await act(EDITOR.name, 'Change role…')
	await changeRoleTo('Administrator')

	expect(await toast('Sorry, you are not allowed to give users that role.')).toBeInTheDocument()
})

test('says why the server refused a role in the reader s language', async () => {
	await readInSpanish()
	server.use(roleRefused('role_not_grantable'))
	renderAt('/users')

	await act(EDITOR.name, 'Cambiar el rol…')
	await userEvent.click(within(await screen.findByRole('dialog')).getByRole('combobox', { name: 'Rol' }))
	await userEvent.click(await screen.findByRole('option', { name: 'Administrador' }))
	await userEvent.click(within(screen.getByRole('dialog')).getByRole('button', { name: 'Cambiar el rol' }))

	expect(await toast('Lo siento, no tienes permisos para dar ese rol a los usuarios.')).toBeInTheDocument()
})
