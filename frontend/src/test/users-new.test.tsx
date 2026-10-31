// SPDX-License-Identifier: Apache-2.0

import { http, HttpResponse, server } from '@gophenberg/frontend-sdk/testing'
import { DOMAIN as AUTH_DOMAIN, catalogFor as authCatalogFor } from '@gopherium/react-auth'
import { accountWithRole } from '@gopherium/react-auth/testing'
import { screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { resetLocaleData, setLocaleData } from '@wordpress/i18n'
import { beforeAll, beforeEach, expect, onTestFinished, test } from 'vitest'

import { catalogFor } from '../i18n/catalog'
import { startingRole } from '../users/roles'
import { adminUser, renderAt } from './render'
import { siteSettings } from './siteSettings'
import { SIGNED_IN, rowOf, serveAccounts, toast, warmUsersScreen } from './usersFixtures'

/** The account a send creates, as the server answers it. */
const CREATED = accountWithRole('editor')

/** The refusal of an email another account already holds. */
const EMAIL_TAKEN = { error: 'email already in use', code: 'email_taken' }

/** The refusal of an email the server cannot read as one. */
const EMAIL_INVALID = { error: 'invalid email address', code: 'email_invalid' }

/** The refusal of a name longer than the server keeps. */
const NAME_TOO_LONG = { error: 'name too long', code: 'name_too_long', meta: { max: 256 } }

/** What a test types in each field it fills one by one, keyed by the field's label. */
const TYPED_IN: Record<string, string> = { 'Email (Required)': 'maria@example.com', 'Name (Required)': 'Maria Perez' }

/** A word no dictionary holds, which the meter rates as strong on its own. */
const UNLISTED_WORD = 'qvzkjwplmxrt'

/** The words under a weak password Add User was pressed on before its use was confirmed. */
const CONFIRM_WEAK = 'Please confirm the use of a weak password.'

/** The refusal of a role the reader may not give. */
const ROLE_NOT_GRANTABLE = { error: 'role not grantable', code: 'role_not_grantable' }

/** The refusal of a role nothing declares. */
const ROLE_UNKNOWN = { error: 'unknown role', code: 'role_unknown', meta: { field: 'role' } }

warmUsersScreen()

beforeAll(async () => {
	await import('../users/NewUserScreen')
})

beforeEach(() => serveAccounts(SIGNED_IN))

/**
 * Serves the settings naming another role new accounts start on.
 * @param role - The role the settings name.
 */
function serveDefaultRole(role: string) {
	server.use(http.get('/api/settings', () => HttpResponse.json({ ...siteSettings, default_role: role })))
}

/**
 * Records every account sent and answers each with the account created.
 * @returns The bodies sent.
 */
function recordCreates(): unknown[] {
	const sent: unknown[] = []
	server.use(
		http.post('/api/users', async ({ request }) => {
			sent.push(await request.json())
			return HttpResponse.json(CREATED, { status: 201 })
		}),
	)
	return sent
}

/**
 * Answers every account sent with a refusal.
 * @param status - The status of the refusal.
 * @param body - The refusal the server sends.
 */
function refuseCreates(status: number, body: object) {
	server.use(http.post('/api/users', () => HttpResponse.json(body, { status })))
}

/**
 * Opens Add User and types an email and a name.
 * @param email - The email to type.
 * @param name - The name to type.
 */
async function fillIn(email = 'maria@example.com', name = 'Maria Perez') {
	renderAt('/users/new')
	await userEvent.type(await screen.findByLabelText('Email (Required)'), email)
	await userEvent.type(screen.getByLabelText('Name (Required)'), name)
}

/** Presses Add User at the foot of the form. */
async function send() {
	await userEvent.click(screen.getByRole('button', { name: 'Add User' }))
}

/**
 * Returns the password field.
 * @returns The field.
 */
function passwordField() {
	return screen.getByLabelText<HTMLInputElement>('Password')
}

/** Loads the Spanish catalogue of the account screens until the test ends. */
async function readAccountWordsInSpanish() {
	setLocaleData(await authCatalogFor('es-ES'), AUTH_DOMAIN)
	onTestFinished(() => resetLocaleData({}, AUTH_DOMAIN))
}

test('names the add user screen with its only second level heading and WordPress s line under it', async () => {
	renderAt('/users/new')

	const main = within(await screen.findByRole('main'))

	expect(await main.findByRole('heading', { level: 2 })).toHaveTextContent('Add User')
	expect(main.getAllByRole('heading', { level: 2 })).toHaveLength(1)
	expect(main.getByText('Create a brand new user and add them to this site.')).toBeInTheDocument()
})

test('names the add user screen as WordPress es_ES does for a Spanish reader', async () => {
	setLocaleData(await catalogFor('es-ES'), 'gophenberg')
	onTestFinished(() => resetLocaleData({}, 'gophenberg'))
	renderAt('/users/new')

	expect(await screen.findByRole('heading', { level: 2, name: 'Añadir usuario' })).toBeInTheDocument()
})

test('marks Email and Name as required in their labels', async () => {
	renderAt('/users/new')

	expect(await screen.findByLabelText('Email (Required)')).toBeRequired()
	expect(screen.getByLabelText('Name (Required)')).toBeRequired()
})

test('shows no words under Email and Name before the reader leaves them or sends the form', async () => {
	renderAt('/users/new')

	await screen.findByRole('button', { name: 'Generate password' })

	expect(screen.getByLabelText('Email (Required)')).toHaveAccessibleDescription('')
	expect(screen.getByLabelText('Name (Required)')).toHaveAccessibleDescription('')
})

test('leaves Add User on before anything is typed', async () => {
	renderAt('/users/new')

	expect(await screen.findByRole('button', { name: 'Add User' })).toBeEnabled()
})

test('starts the password on a shown generated one of 24 characters, Generate password before it', async () => {
	renderAt('/users/new')

	const generate = await screen.findByRole('button', { name: 'Generate password' })
	const field = passwordField()

	expect(field.value).toHaveLength(24)
	expect(field).toHaveAttribute('type', 'text')
	expect(generate.compareDocumentPosition(field) & Node.DOCUMENT_POSITION_FOLLOWING).toBeTruthy()
})

test('stacks the password label above its field as the other labels of the form', async () => {
	renderAt('/users/new')

	await screen.findByRole('button', { name: 'Generate password' })

	expect(passwordField().closest('.gopherium-setup__row')).toBeNull()
})

test('starts a new account on the role the settings name', async () => {
	serveDefaultRole('editor')
	renderAt('/users/new')

	await waitFor(() => expect(screen.getByRole('combobox', { name: 'Role' })).toHaveTextContent('Editor'))
})

test('starts on the role with the fewest capabilities the reader may give when it may not give that one', async () => {
	renderAt('/users/new', { ...adminUser, grantable: ['moderator', 'reviewer'] })

	await waitFor(() => expect(screen.getByRole('combobox', { name: 'Role' })).toHaveTextContent('Reviewer'))
})

test('lists the roles the reader may give, the fewest capabilities first, core roles by their labels', async () => {
	renderAt('/users/new', { ...adminUser, grantable: ['admin', 'moderator', 'author'] })

	await userEvent.click(await screen.findByRole('combobox', { name: 'Role' }))
	const offered = (await screen.findAllByRole('option')).map((option) => option.textContent)

	expect(offered).toEqual(['Author', 'Moderator', 'Administrator'])
})

test('offers no role when the session names none the reader may give', async () => {
	renderAt('/users/new', { ...adminUser, grantable: undefined })

	await userEvent.click(await screen.findByRole('combobox', { name: 'Role' }))

	expect(screen.queryAllByRole('option')).toEqual([])
})

test('starts on the role the settings name when the reader may give it', () => {
	expect(startingRole(['admin', 'editor', 'member'], 'editor')).toBe('editor')
})

test('starts on the role with the fewest capabilities the reader may give while the settings name none', () => {
	expect(startingRole(['admin', 'editor'], undefined)).toBe('editor')
})

test('starts on the role with the fewest capabilities of three when the reader may not give the default', () => {
	expect(startingRole(['admin', 'editor', 'author'], 'member')).toBe('author')
})

test('starts on no role when the reader may give none', () => {
	expect(startingRole([], 'member')).toBe('')
})

test('sends the email and the name without the spaces around them, the password shown and the role', async () => {
	const sent = recordCreates()
	await fillIn('  maria@example.com ', '  Maria Perez ')
	const password = passwordField().value

	await send()

	await waitFor(() => expect(sent).toHaveLength(1))
	expect(sent[0]).toEqual({ email: 'maria@example.com', name: 'Maria Perez', password, role: 'member' })
})

test('sends the role the reader picks', async () => {
	const sent = recordCreates()
	await fillIn()
	await userEvent.click(screen.getByRole('combobox', { name: 'Role' }))
	await userEvent.click(await screen.findByRole('option', { name: 'Editor' }))

	await send()

	await waitFor(() => expect(sent).toHaveLength(1))
	expect(sent[0]).toMatchObject({ role: 'editor' })
})

test('reads the list again once the account exists and goes back to it with a toast', async () => {
	recordCreates()
	renderAt('/users')
	await rowOf(SIGNED_IN.name)
	await userEvent.click(within(screen.getByRole('main')).getByRole('link', { name: 'Add User' }))
	await userEvent.type(await screen.findByLabelText('Email (Required)'), CREATED.email)
	await userEvent.type(screen.getByLabelText('Name (Required)'), CREATED.name)
	serveAccounts(SIGNED_IN, CREATED)

	await send()

	expect(await toast('New user created.')).toBeInTheDocument()
	expect(await rowOf(CREATED.name)).toBeDefined()
})

test('sends the account once while the first send is on its way', async () => {
	const sent: unknown[] = []
	server.use(
		http.post('/api/users', async ({ request }) => {
			sent.push(await request.json())
			return new Promise<Response>(() => {})
		}),
	)
	await fillIn()

	await send()
	await waitFor(() => expect(sent).toHaveLength(1))
	await send()

	expect(sent).toHaveLength(1)
	expect(passwordField()).toHaveAttribute('readonly')
})

test('hides the password once the form is sent', async () => {
	refuseCreates(500, { error: 'boom' })
	await fillIn()

	await send()

	await waitFor(() => expect(passwordField()).toHaveAttribute('type', 'password'))
})

test('shows WordPress s words under an empty Email and Name and sends nothing', async () => {
	const sent = recordCreates()
	renderAt('/users/new')

	await userEvent.click(await screen.findByRole('button', { name: 'Add User' }))

	await waitFor(() =>
		expect(screen.getByLabelText('Email (Required)')).toHaveAccessibleDescription('Please enter an email address.'),
	)
	expect(screen.getByLabelText('Name (Required)')).toHaveAccessibleDescription('Please provide a name.')
	expect(sent).toEqual([])
})

test('takes a name of spaces only for an empty one', async () => {
	const sent = recordCreates()
	await fillIn('maria@example.com', '   ')

	await send()

	await waitFor(() =>
		expect(screen.getByLabelText('Name (Required)')).toHaveAccessibleDescription('Please provide a name.'),
	)
	expect(sent).toEqual([])
})

test.each(['{Backspace}', '   '])(
	'shows WordPress s words under a password emptied with %j and sends nothing',
	async (keys) => {
		const sent = recordCreates()
		await fillIn()
		await userEvent.clear(passwordField())
		await userEvent.type(passwordField(), keys)

		await send()

		await waitFor(() =>
			expect(passwordField()).toHaveAccessibleDescription(expect.stringContaining('Please enter a password.')),
		)
		expect(sent).toEqual([])
	},
)

test('drops the words under the password once the reader types one', async () => {
	await fillIn()
	await userEvent.clear(passwordField())
	await send()
	await waitFor(() =>
		expect(passwordField()).toHaveAccessibleDescription(expect.stringContaining('Please enter a password.')),
	)

	await userEvent.type(passwordField(), 'x')

	expect(passwordField()).not.toHaveAccessibleDescription(expect.stringContaining('Please enter a password.'))
})

test('sends a weak password only once its use is confirmed', async () => {
	const sent = recordCreates()
	await fillIn()
	await userEvent.clear(passwordField())
	await userEvent.type(passwordField(), 'abc')
	await send()

	await userEvent.click(await screen.findByRole('checkbox', { name: 'Confirm use of weak password' }))
	await send()

	expect(await toast('New user created.')).toBeInTheDocument()
	expect(sent).toEqual([{ email: 'maria@example.com', name: 'Maria Perez', password: 'abc', role: 'member' }])
})

test('asks under a weak password to confirm its use when Add User is pressed first, sending nothing', async () => {
	const sent = recordCreates()
	await fillIn()
	await userEvent.clear(passwordField())
	await userEvent.type(passwordField(), 'abc')

	await send()

	await waitFor(() => expect(passwordField()).toHaveAccessibleDescription(expect.stringContaining(CONFIRM_WEAK)))
	expect(sent).toEqual([])
})

test('drops the words under a weak password once its use is confirmed', async () => {
	await fillIn()
	await userEvent.clear(passwordField())
	await userEvent.type(passwordField(), 'abc')
	await send()
	await waitFor(() => expect(passwordField()).toHaveAccessibleDescription(expect.stringContaining(CONFIRM_WEAK)))

	await userEvent.click(screen.getByRole('checkbox', { name: 'Confirm use of weak password' }))

	expect(passwordField()).not.toHaveAccessibleDescription(expect.stringContaining(CONFIRM_WEAK))
})

test('rates as very weak a password repeating a word of the email typed beside it, as WordPress does', async () => {
	await fillIn(`${UNLISTED_WORD}@example.com`)
	await userEvent.clear(passwordField())

	await userEvent.type(passwordField(), UNLISTED_WORD)

	await waitFor(() => expect(passwordField()).toHaveAccessibleDescription(expect.stringContaining('Very weak')))
	expect(screen.getByRole('checkbox', { name: 'Confirm use of weak password' })).toBeInTheDocument()
})

test.each([
	['Email (Required)', 409, EMAIL_TAKEN, 'This email is already registered. Please choose another one.'],
	['Email (Required)', 422, EMAIL_INVALID, 'The email address is not correct.'],
	['Name (Required)', 422, NAME_TOO_LONG, 'This name is too long.'],
])('shows under %s why the server answered %i', async (label, status, body, words) => {
	refuseCreates(status, body)
	await fillIn()

	await send()

	await waitFor(() => expect(screen.getByLabelText(label)).toHaveAccessibleDescription(words))
})

test.each([
	{
		field: 'Name (Required)',
		first: 'Email (Required)',
		status: 422,
		body: NAME_TOO_LONG,
		words: 'This name is too long.',
	},
	{
		field: 'Email (Required)',
		first: 'Name (Required)',
		status: 409,
		body: EMAIL_TAKEN,
		words: 'This email is already registered. Please choose another one.',
	},
])('shows under $field why the server answered $status when the form is sent with Enter from that field', async ({
	field,
	first,
	status,
	body,
	words,
}) => {
	refuseCreates(status, body)
	renderAt('/users/new')
	await userEvent.type(await screen.findByLabelText(first), TYPED_IN[first])
	await userEvent.type(screen.getByLabelText(field), TYPED_IN[field])

	await userEvent.keyboard('{Enter}')

	await waitFor(() => expect(screen.getByLabelText(field)).toHaveAccessibleDescription(words))
})

test.each([
	['password_too_short', { min: 12 }, 'Hint: The password should be at least 12 characters long.'],
	['password_too_long', { max: 1024 }, 'This password is too long.'],
])('shows under the password why the server answered %s', async (code, meta, words) => {
	refuseCreates(422, { error: 'password refused', code, meta })
	await fillIn()

	await send()

	await waitFor(() => expect(passwordField()).toHaveAccessibleDescription(expect.stringContaining(words)))
})

test('lets the form go again once the reader changes the email the server refused', async () => {
	refuseCreates(409, EMAIL_TAKEN)
	await fillIn()
	await send()
	const email = screen.getByLabelText('Email (Required)')
	await waitFor(() =>
		expect(email).toHaveAccessibleDescription('This email is already registered. Please choose another one.'),
	)
	const sent = recordCreates()

	await userEvent.clear(email)
	await userEvent.type(email, 'maria.perez@example.com')

	expect(email).toHaveAccessibleDescription('')
	await send()
	await waitFor(() => expect(sent).toHaveLength(1))
})

test('shows in the reader s language why the server refused the email', async () => {
	await readAccountWordsInSpanish()
	refuseCreates(409, EMAIL_TAKEN)
	await fillIn()

	await send()

	await waitFor(() =>
		expect(screen.getByLabelText('Email (Required)')).toHaveAccessibleDescription(
			'Este correo electrónico ya está registrado. Por favor, elige otro.',
		),
	)
})

test.each([
	[403, ROLE_NOT_GRANTABLE, 'Sorry, you are not allowed to give users that role.'],
	[422, ROLE_UNKNOWN, 'Sorry, you are not allowed to give users that role.'],
	[422, { error: 'role is required', code: 'body_field_required' }, 'The user could not be created.'],
	[500, { error: 'boom' }, 'The user could not be created.'],
])('says in a toast why the server answered %i, the form kept as typed', async (status, body, words) => {
	refuseCreates(status, body)
	await fillIn()

	await send()

	expect(await toast(words)).toBeInTheDocument()
	expect(screen.getByLabelText('Email (Required)')).toHaveValue('maria@example.com')
	expect(screen.getByLabelText('Name (Required)')).toHaveValue('Maria Perez')
})

test('says in a toast in the reader s language that the reader may not give the role', async () => {
	await readAccountWordsInSpanish()
	refuseCreates(403, ROLE_NOT_GRANTABLE)
	await fillIn()

	await send()

	expect(await toast('Lo siento, no tienes permisos para dar ese rol a los usuarios.')).toBeInTheDocument()
})
