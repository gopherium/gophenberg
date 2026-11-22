// SPDX-License-Identifier: Apache-2.0

import { http, HttpResponse, server } from '@gophenberg/frontend-sdk/testing'
import { DOMAIN as AUTH_DOMAIN, catalogFor as authCatalogFor } from '@gopherium/react-auth'
import { accountWithRole, inviteDelivered, inviteFailure, inviteUndelivered } from '@gopherium/react-auth/testing'
import { screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { resetLocaleData, setLocaleData } from '@wordpress/i18n'
import { beforeAll, beforeEach, expect, onTestFinished, test } from 'vitest'

import { adminBasepath } from '../basepath'
import { catalogFor } from '../i18n/catalog'
import { startingRole } from '../users/roles'
import { listenAt } from './announcements'
import { gate } from './gate'
import { adminUser, renderAt, renderRoutedAt } from './render'
import { siteSettings } from './siteSettings'
import {
	REQUEST_CROSS_ORIGIN,
	REQUEST_CROSS_ORIGIN_WORDS,
	SIGNED_IN,
	rowOf,
	serveAccounts,
	toast,
	warmUsersScreen,
} from './usersFixtures'

/** The account an invitation leaves waiting for its activation, as the list answers it. */
const INVITED = accountWithRole('editor', { confirmed: false })

/** The link an invitation answers when no mail server delivered it. */
const ACTIVATION_LINK = 'https://example.com/admin/activate?token=t-4'

/** The words above the activation link while no mail server delivers invitations. */
const NO_MAIL = 'No mail server is configured. Deliver the activation link by hand.'

/** The words of the toast an undelivered invitation raises once the reader left Add User. */
const LINK_WAITS = 'No mail server is configured. Use Resend invitation in Users to get an activation link.'

/** The words every toast of a failed invitation starts with. */
const FAILED = 'Could not send the invitation.'

/** The rejection of an email the server cannot read as one. */
const EMAIL_INVALID = { error: 'invalid email address', code: 'email_invalid' }

/** The words under an email the server cannot read as one. */
const EMAIL_INVALID_WORDS = 'The email address is not correct.'

/** The rejection of a name longer than the server keeps. */
const NAME_TOO_LONG = { error: 'name too long', code: 'name_too_long', meta: { max: 256 } }

/** The words under a name longer than the server keeps. */
const NAME_TOO_LONG_WORDS = 'This name is too long.'

/** What a test types in each field it fills one by one, keyed by the field's label. */
const TYPED_IN: Record<string, string> = { 'Email (Required)': 'maria@example.com', 'Name (Required)': 'Maria Perez' }

/** The rejection of a role the reader may not give. */
const ROLE_NOT_GRANTABLE = { error: 'role not grantable', code: 'role_not_grantable' }

/** The rejection of a role nothing declares. */
const ROLE_UNKNOWN = { error: 'unknown role', code: 'role_unknown', meta: { field: 'role' } }

/** The rejection of a reader who may not create accounts at all. */
const ROLE_INSUFFICIENT = { error: 'role insufficient', code: 'role_insufficient' }

/** The words of each language for the invitation and its activation link, as the reader sees them. */
const INVITATION_WORDS = [
	{
		locale: 'es-ES',
		send: 'Enviar invitación',
		sent: 'Invitación enviada.',
		noMail: 'No hay servidor de correo configurado. Entrega el enlace de activación a mano.',
		link: 'Enlace de activación',
		done: 'Hecho',
		waits:
			'No hay servidor de correo configurado. Usa Reenviar invitación en Usuarios para obtener un enlace de activación.',
		failed: 'No se ha podido enviar la invitación.',
		mayNotCreate: 'No se ha podido enviar la invitación. Lo siento, no tienes permisos para crear usuarios.',
	},
	{
		locale: 'fr-FR',
		send: 'Envoyer l’invitation',
		sent: 'Invitation envoyée.',
		noMail: 'Aucun serveur de messagerie n’est configuré. Transmettez le lien d’activation vous-même.',
		link: 'Lien d’activation',
		done: 'Terminé',
		waits:
			'Aucun serveur de messagerie n’est configuré. ' +
			'Utilisez Renvoyer l’invitation dans Comptes pour obtenir un lien d’activation.',
		failed: 'Impossible d’envoyer l’invitation.',
		mayNotCreate: 'Impossible d’envoyer l’invitation. Désolé, vous n’avez pas l’autorisation de créer des comptes.',
	},
]

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
 * Records every invitation sent and leaves its answer to the handler given.
 * @param answer - The handler answering each invitation, a delivered one unless a test names another.
 * @returns The bodies sent.
 */
function recordInvites(answer = inviteDelivered()): unknown[] {
	const sent: unknown[] = []
	server.use(
		http.post('/api/users/invite', async ({ request }) => {
			sent.push(await request.clone().json())
		}),
		answer,
	)
	return sent
}

/**
 * Records every invitation sent and never answers it.
 * @returns The bodies sent.
 */
function recordUnanswered(): unknown[] {
	return recordInvites(http.post('/api/users/invite', () => new Promise<Response>(() => {})))
}

/**
 * Answers every invitation with a rejection.
 * @param status - The status of the rejection.
 * @param body - The rejection the server sends.
 */
function rejectInvites(status: number, body: object) {
	server.use(http.post('/api/users/invite', () => HttpResponse.json(body, { status })))
}

/**
 * Holds every invitation until the test opens the gate, then leaves it to the handler given.
 * @param answer - The handler answering each invitation once the gate opens.
 * @returns The opener of the gate.
 */
function holdInvites(answer: ReturnType<typeof http.post>): () => void {
	const opened = gate()
	server.use(
		http.post('/api/users/invite', async () => {
			await opened.held
		}),
		answer,
	)
	return opened.release
}

/**
 * Holds every invitation until the test opens the gate, then answers it with a rejection.
 * @param status - The status of the rejection.
 * @param body - The rejection the server sends.
 * @returns The opener of the gate.
 */
function holdRejection(status: number, body: object): () => void {
	return holdInvites(http.post('/api/users/invite', () => HttpResponse.json(body, { status })))
}

/** Leaves Add User for the Settings screen through the admin's menu. */
async function leaveForSettings() {
	const menu = screen.getByRole('region', { name: 'Navigation' })
	await userEvent.click(within(menu).getByRole('link', { name: 'Settings' }))
	await screen.findByRole('heading', { level: 2, name: 'Settings' })
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

/**
 * Opens Add User and types an email and a name, whatever language the labels read in.
 * @returns The router behind the admin.
 */
async function fillInAnyLanguage() {
	const { router } = renderRoutedAt('/users/new')
	const [email, name] = await screen.findAllByRole('textbox')
	await userEvent.type(email, 'maria@example.com')
	await userEvent.type(name, 'Maria Perez')
	return router
}

/** Presses Send invitation at the foot of the form. */
async function send() {
	await userEvent.click(screen.getByRole('button', { name: 'Send invitation' }))
}

/**
 * Returns the toast showing the words right now, null while none does.
 * @param text - The words of the toast.
 * @returns The toast.
 */
function toastNow(text: string): HTMLElement | null {
	return screen.queryByText(text, { selector: '.godmin-toast' })
}

/**
 * Notes the second level heading of the page as the first toast appears.
 * @returns The headings noted, none until a toast appears.
 */
function headingAtFirstToast(): string[] {
	const noted: string[] = []
	const watch = new MutationObserver(() => {
		if (noted.length === 0 && document.querySelector('.godmin-toast') !== null) {
			noted.push(document.querySelector('main h2')?.textContent ?? '')
		}
	})
	watch.observe(document.body, { childList: true, subtree: true })
	onTestFinished(() => watch.disconnect())
	return noted
}

/** Loads the Spanish catalogue of the account screens until the test ends. */
async function readAccountWordsInSpanish() {
	setLocaleData(await authCatalogFor('es-ES'), AUTH_DOMAIN)
	onTestFinished(() => resetLocaleData({}, AUTH_DOMAIN))
}

/**
 * Loads the catalogues of the admin and of the account screens in a language until the test ends.
 * @param locale - The language, such as es-ES.
 */
async function readIn(locale: string) {
	setLocaleData(await catalogFor(locale), 'gophenberg')
	setLocaleData(await authCatalogFor(locale), AUTH_DOMAIN)
	onTestFinished(() => {
		resetLocaleData({}, 'gophenberg')
		resetLocaleData({}, AUTH_DOMAIN)
	})
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

test('leads Add User back to the users list, never as the current page', async () => {
	renderAt('/users/new')

	const breadcrumbs = await screen.findByRole('navigation', { name: 'Breadcrumbs' })
	const back = within(breadcrumbs).getByRole('link', { name: 'Users' })

	expect(back).toHaveAttribute('href', `${adminBasepath}/users`)
	expect(back).not.toHaveAttribute('aria-current')
})

test('names the link back to the users list in the reader s language', async () => {
	await readAccountWordsInSpanish()
	renderAt('/users/new')

	const breadcrumbs = await screen.findByRole('navigation', { name: 'Breadcrumbs' })

	expect(within(breadcrumbs).getByRole('link', { name: 'Usuarios' })).toBeInTheDocument()
})

test('marks Email and Name as required in their labels', async () => {
	renderAt('/users/new')

	expect(await screen.findByLabelText('Email (Required)')).toBeRequired()
	expect(screen.getByLabelText('Name (Required)')).toBeRequired()
})

test('shows no words under Email and Name before the reader leaves them or sends the form', async () => {
	renderAt('/users/new')

	await screen.findByRole('button', { name: 'Send invitation' })

	expect(screen.getByLabelText('Email (Required)')).toHaveAccessibleDescription('')
	expect(screen.getByLabelText('Name (Required)')).toHaveAccessibleDescription('')
})

test('asks for no password, since the invited person chooses one from the link', async () => {
	renderAt('/users/new')

	await screen.findByRole('button', { name: 'Send invitation' })

	expect(screen.queryByLabelText('Password')).toBeNull()
	expect(screen.queryByRole('button', { name: 'Generate password' })).toBeNull()
})

test('leaves Send invitation on before anything is typed', async () => {
	renderAt('/users/new')

	const button = await screen.findByRole('button', { name: 'Send invitation' })

	expect(button).toBeEnabled()
	expect(button).not.toHaveAttribute('aria-disabled', 'true')
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

test('sends the email and the name without the spaces around them, and the role, with no password', async () => {
	const sent = recordInvites()
	await fillIn('  maria@example.com ', '  Maria Perez ')

	await send()

	await waitFor(() => expect(sent).toHaveLength(1))
	expect(sent[0]).toEqual({ email: 'maria@example.com', name: 'Maria Perez', role: 'member' })
})

test('sends the role the reader picks', async () => {
	const sent = recordInvites()
	await fillIn()
	await userEvent.click(screen.getByRole('combobox', { name: 'Role' }))
	await userEvent.click(await screen.findByRole('option', { name: 'Editor' }))

	await send()

	await waitFor(() => expect(sent).toHaveLength(1))
	expect(sent[0]).toMatchObject({ role: 'editor' })
})

test('reads the list again once the invitation went out and goes back to it', async () => {
	recordInvites()
	renderAt('/users')
	await rowOf(SIGNED_IN.name)
	await userEvent.click(within(screen.getByRole('main')).getByRole('link', { name: 'Add User' }))
	await userEvent.type(await screen.findByLabelText('Email (Required)'), INVITED.email)
	await userEvent.type(screen.getByLabelText('Name (Required)'), INVITED.name)
	serveAccounts(SIGNED_IN, INVITED)

	await send()

	expect(await rowOf(INVITED.name)).toBeDefined()
})

test('moves to the list first, then says in a toast that the invitation went out', async () => {
	server.use(inviteDelivered())
	const noted = headingAtFirstToast()
	await fillIn()

	await send()

	expect(await toast('Invitation sent.')).toBeInTheDocument()
	expect(noted).toEqual(['Users'])
})

test('sends the invitation once while the first send is on its way', async () => {
	const sent = recordUnanswered()
	await fillIn()

	await send()
	await waitFor(() => expect(sent).toHaveLength(1))
	await send()

	expect(sent).toHaveLength(1)
})

test('sends the invitation once when Enter is pressed again while the first send is on its way', async () => {
	const sent = recordUnanswered()
	await fillIn()

	await userEvent.keyboard('{Enter}')
	await waitFor(() => expect(sent).toHaveLength(1))
	await userEvent.keyboard('{Enter}')

	expect(sent).toHaveLength(1)
})

test('shows Send invitation busy with its spinner while the invitation is on its way', async () => {
	recordUnanswered()
	await fillIn()
	const button = screen.getByRole('button', { name: 'Send invitation' })

	await userEvent.click(button)

	await waitFor(() => expect(button).toHaveAttribute('aria-disabled', 'true'))
	expect(button.className).toMatch(/__is-loading\b/)
})

test('shows WordPress s words under an empty Email and Name and sends nothing', async () => {
	const sent = recordInvites()
	renderAt('/users/new')

	await userEvent.click(await screen.findByRole('button', { name: 'Send invitation' }))

	await waitFor(() =>
		expect(screen.getByLabelText('Email (Required)')).toHaveAccessibleDescription('Please enter an email address.'),
	)
	expect(screen.getByLabelText('Name (Required)')).toHaveAccessibleDescription('Please provide a name.')
	expect(sent).toEqual([])
})

test('takes a name of spaces only for an empty one', async () => {
	const sent = recordInvites()
	await fillIn('maria@example.com', '   ')

	await send()

	await waitFor(() =>
		expect(screen.getByLabelText('Name (Required)')).toHaveAccessibleDescription('Please provide a name.'),
	)
	expect(sent).toEqual([])
})

test('shows the activation link in place of the form when no mail server delivered the invitation', async () => {
	server.use(inviteUndelivered(ACTIVATION_LINK))
	await fillIn()

	await send()

	const link = await screen.findByLabelText('Activation link')
	expect(link).toHaveValue(ACTIVATION_LINK)
	expect(link).toHaveAttribute('readonly')
	expect(link).toHaveAccessibleDescription(NO_MAIL)
	expect(screen.getByText(NO_MAIL)).toHaveAttribute('role', 'status')
	expect(screen.queryByLabelText('Email (Required)')).toBeNull()
	expect(screen.queryByRole('button', { name: 'Send invitation' })).toBeNull()
	expect(toastNow('Invitation sent.')).toBeNull()
})

test('keeps the title, the line under it and the breadcrumb beside the activation link', async () => {
	server.use(inviteUndelivered(ACTIVATION_LINK))
	await fillIn()

	await send()
	await screen.findByLabelText('Activation link')

	const main = within(screen.getByRole('main'))
	expect(main.getByRole('heading', { level: 2 })).toHaveTextContent('Add User')
	expect(main.getByText('Create a brand new user and add them to this site.')).toBeInTheDocument()
	const breadcrumbs = screen.getByRole('navigation', { name: 'Breadcrumbs' })
	expect(within(breadcrumbs).getByRole('link', { name: 'Users' })).toHaveAttribute('href', `${adminBasepath}/users`)
})

test('puts the focus on the activation link once it replaces the form', async () => {
	server.use(inviteUndelivered(ACTIVATION_LINK))
	await fillIn()

	await send()

	const link = await screen.findByLabelText('Activation link')
	await waitFor(() => expect(link).toHaveFocus())
})

test('goes back to the list with Done, the invited account listed and no toast', async () => {
	server.use(inviteUndelivered(ACTIVATION_LINK))
	renderAt('/users')
	await rowOf(SIGNED_IN.name)
	await userEvent.click(within(screen.getByRole('main')).getByRole('link', { name: 'Add User' }))
	await userEvent.type(await screen.findByLabelText('Email (Required)'), INVITED.email)
	await userEvent.type(screen.getByLabelText('Name (Required)'), INVITED.name)
	serveAccounts(SIGNED_IN, INVITED)
	await send()
	await screen.findByLabelText('Activation link')

	await userEvent.click(screen.getByRole('button', { name: 'Done' }))

	expect(await rowOf(INVITED.name)).toBeDefined()
	expect(screen.getByRole('heading', { level: 2, name: 'Users' })).toBeInTheDocument()
	expect(toastNow('Invitation sent.')).toBeNull()
})

test.each([
	['Email (Required)', EMAIL_INVALID, EMAIL_INVALID_WORDS],
	['Name (Required)', NAME_TOO_LONG, NAME_TOO_LONG_WORDS],
])('shows under %s why the server rejected the invitation, the focus on that field and no toast', async (
	label,
	body,
	words,
) => {
	rejectInvites(422, body)
	await fillIn()

	await send()

	await waitFor(() => expect(screen.getByLabelText(label)).toHaveAccessibleDescription(words))
	expect(screen.getByLabelText(label)).toHaveFocus()
	expect(toastNow(`${FAILED} ${words}`)).toBeNull()
})

test.each([
	{ field: 'Name (Required)', first: 'Email (Required)', body: NAME_TOO_LONG, words: NAME_TOO_LONG_WORDS },
	{ field: 'Email (Required)', first: 'Name (Required)', body: EMAIL_INVALID, words: EMAIL_INVALID_WORDS },
])('shows under $field why the server rejected the invitation sent with Enter from that field', async ({
	field,
	first,
	body,
	words,
}) => {
	rejectInvites(422, body)
	renderAt('/users/new')
	await userEvent.type(await screen.findByLabelText(first), TYPED_IN[first])
	await userEvent.type(screen.getByLabelText(field), TYPED_IN[field])

	await userEvent.keyboard('{Enter}')

	await waitFor(() => expect(screen.getByLabelText(field)).toHaveAccessibleDescription(words))
})

test('speaks at once the words the server put under a field', async () => {
	const assertive = listenAt('assertive')
	rejectInvites(422, NAME_TOO_LONG)
	await fillIn()

	await send()

	await waitFor(() => expect(assertive()).toContain(NAME_TOO_LONG_WORDS))
})

test('drops the words under Email at the first edit, never again shown for the same address typed back', async () => {
	rejectInvites(422, EMAIL_INVALID)
	await fillIn()
	await send()
	const email = screen.getByLabelText('Email (Required)')
	await waitFor(() => expect(email).toHaveAccessibleDescription(EMAIL_INVALID_WORDS))

	await userEvent.type(email, 'x')
	await waitFor(() => expect(email).toHaveAccessibleDescription(''))
	await userEvent.type(email, '{Backspace}')

	expect(email).toHaveValue('maria@example.com')
	expect(email).toHaveAccessibleDescription('')
	expect(email).toBeValid()
})

test('lets the form go again once the reader changes the email the server rejected', async () => {
	rejectInvites(422, EMAIL_INVALID)
	await fillIn()
	await send()
	const email = screen.getByLabelText('Email (Required)')
	await waitFor(() => expect(email).toHaveAccessibleDescription(EMAIL_INVALID_WORDS))
	const sent = recordInvites()

	await userEvent.clear(email)
	await userEvent.type(email, 'maria.perez@example.com')

	expect(email).toHaveAccessibleDescription('')
	await send()
	await waitFor(() => expect(sent).toEqual([expect.objectContaining({ email: 'maria.perez@example.com' })]))
})

test('says in a toast why the server rejected the email the reader changed while it was on its way', async () => {
	const release = holdRejection(422, EMAIL_INVALID)
	await fillIn()
	await send()
	const email = screen.getByLabelText('Email (Required)')

	await userEvent.type(email, 'x')
	release()

	expect(await toast(`${FAILED} ${EMAIL_INVALID_WORDS}`)).toBeInTheDocument()
	expect(email).toHaveAccessibleDescription('')
})

test('says in a toast why the server rejected the name once the reader left the form for the list', async () => {
	const release = holdRejection(422, NAME_TOO_LONG)
	await fillIn()
	await send()
	const breadcrumbs = screen.getByRole('navigation', { name: 'Breadcrumbs' })

	await userEvent.click(within(breadcrumbs).getByRole('link', { name: 'Users' }))
	await screen.findByRole('heading', { level: 2, name: 'Users' })
	release()

	expect(await toast(`${FAILED} ${NAME_TOO_LONG_WORDS}`)).toBeInTheDocument()
})

test('says in a toast that the invitation went out after the reader left, keeping the reader there', async () => {
	const release = holdInvites(inviteDelivered())
	await fillIn()
	await send()

	await leaveForSettings()
	release()

	expect(await toast('Invitation sent.')).toBeInTheDocument()
	expect(screen.getByRole('heading', { level: 2, name: 'Settings' })).toBeInTheDocument()
})

test('says in a toast where to get an activation link once the reader left before the answer came', async () => {
	const release = holdInvites(inviteUndelivered(ACTIVATION_LINK))
	await fillIn()
	await send()

	await leaveForSettings()
	release()

	expect(await toast(LINK_WAITS)).toBeInTheDocument()
	expect(screen.getByRole('heading', { level: 2, name: 'Settings' })).toBeInTheDocument()
})

test('shows in the reader s language why the server rejected the email', async () => {
	await readAccountWordsInSpanish()
	rejectInvites(422, EMAIL_INVALID)
	await fillIn()

	await send()

	await waitFor(() =>
		expect(screen.getByLabelText('Email (Required)')).toHaveAccessibleDescription(
			'La dirección de correo electrónico no es correcta.',
		),
	)
})

test.each([
	{ status: 403, body: ROLE_NOT_GRANTABLE, words: `${FAILED} Sorry, you are not allowed to give users that role.` },
	{ status: 422, body: ROLE_UNKNOWN, words: `${FAILED} Sorry, you are not allowed to give users that role.` },
	{ status: 403, body: REQUEST_CROSS_ORIGIN, words: `${FAILED} ${REQUEST_CROSS_ORIGIN_WORDS}` },
	{ status: 403, body: ROLE_INSUFFICIENT, words: `${FAILED} Sorry, you are not allowed to create users.` },
	{ status: 422, body: { error: 'that name is too long' }, words: `${FAILED} that name is too long` },
	{ status: 422, body: { error: 'role is required', code: 'body_field_required' }, words: FAILED },
	{ status: 502, body: { error: 'delivery failed', code: 'delivery_failed' }, words: FAILED },
])('says in a toast why the server answered $status $body.code, the form kept as typed', async ({
	status,
	body,
	words,
}) => {
	rejectInvites(status, body)
	await fillIn()

	await send()

	expect(await toast(words)).toBeInTheDocument()
	expect(screen.getByLabelText('Email (Required)')).toHaveValue('maria@example.com')
	expect(screen.getByLabelText('Name (Required)')).toHaveValue('Maria Perez')
	expect(screen.getByLabelText('Name (Required)')).toBeValid()
})

test('says in a toast only that the invitation was not sent when the server failed, the button on again', async () => {
	server.use(inviteFailure())
	await fillIn()

	await send()

	expect(await toast(FAILED)).toBeInTheDocument()
	expect(screen.queryByText(/internal error/)).toBeNull()
	await waitFor(() =>
		expect(screen.getByRole('button', { name: 'Send invitation' })).not.toHaveAttribute('aria-disabled', 'true'),
	)
	expect(screen.getByLabelText('Email (Required)')).toHaveValue('maria@example.com')
})

test('says in a toast in the reader s language that the reader may not give the role', async () => {
	await readIn('es-ES')
	rejectInvites(403, ROLE_NOT_GRANTABLE)
	await fillInAnyLanguage()

	await userEvent.click(screen.getByRole('button', { name: 'Enviar invitación' }))

	expect(
		await toast('No se ha podido enviar la invitación. Lo siento, no tienes permisos para dar ese rol a los usuarios.'),
	).toBeInTheDocument()
})

test.each(INVITATION_WORDS)('reads Send invitation and the toast of a delivered invitation in $locale', async ({
	locale,
	send: sendWords,
	sent,
}) => {
	await readIn(locale)
	server.use(inviteDelivered())
	await fillInAnyLanguage()

	await userEvent.click(screen.getByRole('button', { name: sendWords }))

	expect(await toast(sent)).toBeInTheDocument()
})

test.each(INVITATION_WORDS)('reads the activation link page in $locale', async ({
	locale,
	send: sendWords,
	...words
}) => {
	await readIn(locale)
	server.use(inviteUndelivered(ACTIVATION_LINK))
	await fillInAnyLanguage()

	await userEvent.click(screen.getByRole('button', { name: sendWords }))

	expect(await screen.findByLabelText(words.link)).toHaveValue(ACTIVATION_LINK)
	expect(screen.getByText(words.noMail)).toHaveAttribute('role', 'status')
	expect(screen.getByRole('button', { name: words.done })).toBeInTheDocument()
})

test.each(INVITATION_WORDS)('reads where to get an activation link once the reader left in $locale', async ({
	locale,
	send: sendWords,
	waits,
}) => {
	await readIn(locale)
	const release = holdInvites(inviteUndelivered(ACTIVATION_LINK))
	const router = await fillInAnyLanguage()
	await userEvent.click(screen.getByRole('button', { name: sendWords }))

	await router.navigate({ to: '/settings' })
	await waitFor(() => expect(screen.queryByRole('button', { name: sendWords })).toBeNull())
	release()

	expect(await toast(waits)).toBeInTheDocument()
})

test.each(INVITATION_WORDS)('reads the toasts of a failed invitation in $locale', async ({
	locale,
	send: sendWords,
	failed,
	mayNotCreate,
}) => {
	await readIn(locale)
	server.use(inviteFailure())
	await fillInAnyLanguage()
	const button = screen.getByRole('button', { name: sendWords })

	await userEvent.click(button)
	expect(await toast(failed)).toBeInTheDocument()
	rejectInvites(403, ROLE_INSUFFICIENT)
	await waitFor(() => expect(button).not.toHaveAttribute('aria-disabled', 'true'))
	await userEvent.click(button)

	expect(await toast(mayNotCreate)).toBeInTheDocument()
})
