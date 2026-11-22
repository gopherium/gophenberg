// SPDX-License-Identifier: Apache-2.0

import { ACCESS_ADMIN } from '@gophenberg/frontend-sdk'
import { http, HttpResponse, server } from '@gophenberg/frontend-sdk/testing'
import { DOMAIN as BRICK_DOMAIN, catalogFor as brickCatalogFor, createAuthQueryClient } from '@gopherium/react-auth'
import type { User } from '@gopherium/react-auth'
import { activateInvalidToken, activateOk, defaultUser, sessionAnonymous } from '@gopherium/react-auth/testing'
import { QueryClientProvider } from '@tanstack/react-query'
import { render, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { resetLocaleData, setLocaleData } from '@wordpress/i18n'
import { StrictMode } from 'react'
import type { ReactNode } from 'react'
import { expect, onTestFinished, test, vi } from 'vitest'

import { AdminHost } from '../AdminHost'
import { publicAuthScreen } from '../PublicAuth'

/** An activation link as the server mails it. */
const ACTIVATION_LINK = new URL('https://example.com/admin/activate?token=t-1')

/** An activation link that lost its token on the way. */
const LINK_WITHOUT_TOKEN = new URL('https://example.com/admin/activate')

/** A password the server takes. */
const PASSWORD = 'correct horse battery staple'

/** An account carrying the capability that opens the admin. */
const MAY_OPEN_ADMIN: User = { ...defaultUser, role: 'editor', capabilities: [ACCESS_ADMIN] }

/** An account carrying no capability that opens the admin. */
const MAY_NOT_OPEN_ADMIN: User = { ...defaultUser, role: 'member', capabilities: [] }

/** How long a test waits for a request the page must never send. */
const QUIET = 60

/**
 * Renders the page as main renders it, under the strict checks, the auth query client and the admin host.
 * @param page - The page to render.
 */
function renderAsMain(page: ReactNode) {
	render(
		<StrictMode>
			<QueryClientProvider client={createAuthQueryClient()}>
				<AdminHost>{page}</AdminHost>
			</QueryClientProvider>
		</StrictMode>,
	)
}

/**
 * Records the method and path of every request the page sends until the test ends.
 * @returns The requests sent so far, in order.
 */
function recordRequests(): string[] {
	const asked: string[] = []
	server.use(
		http.all('*', ({ request }) => {
			asked.push(`${request.method} ${new URL(request.url).pathname}`)
		}),
	)
	return asked
}

/**
 * Records the body of every activation the page sends.
 * @returns The bodies sent so far, in order.
 */
function recordActivations(): unknown[] {
	const sent: unknown[] = []
	server.use(
		http.post('/api/auth/activate', async ({ request }) => {
			sent.push(await request.clone().json())
		}),
	)
	return sent
}

/** Types the password on the page and sends it. */
async function setPassword() {
	await userEvent.type(await screen.findByLabelText('Password'), PASSWORD)
	await userEvent.click(screen.getByRole('button', { name: 'Set password' }))
}

test.each(['/', '/admin', '/admin/', '/admin/users', '/activate', '/admin/activateX', '/admin/activate/again'])(
	'opens no activation page at %s',
	(path) => {
		expect(publicAuthScreen(new URL(path, ACTIVATION_LINK), vi.fn())).toBeNull()
	},
)

test('opens the activation page under the Gophenberg name at the path the server mails', async () => {
	renderAsMain(publicAuthScreen(ACTIVATION_LINK, vi.fn()))

	expect(await screen.findByRole('heading', { level: 1, name: 'Gophenberg' })).toBeInTheDocument()
	expect(screen.getByLabelText('Password')).toBeInTheDocument()
	expect(screen.getByRole('button', { name: 'Set password' })).toBeInTheDocument()
})

test('sends the token the link carries with the password chosen', async () => {
	server.use(activateOk(MAY_OPEN_ADMIN))
	const sent = recordActivations()
	renderAsMain(publicAuthScreen(ACTIVATION_LINK, vi.fn()))

	await setPassword()

	await waitFor(() => expect(sent).toEqual([{ token: 't-1', password: PASSWORD }]))
})

test('sends an empty token from a link that carries none and says the link is no longer valid', async () => {
	server.use(activateInvalidToken())
	const sent = recordActivations()
	const go = vi.fn()
	renderAsMain(publicAuthScreen(LINK_WITHOUT_TOKEN, go))

	await setPassword()

	expect(await screen.findByRole('alert')).toHaveTextContent(
		'This link is no longer valid. Ask the person who invited you for a new one.',
	)
	expect(sent).toEqual([{ token: '', password: PASSWORD }])
	expect(go).not.toHaveBeenCalled()
})

test.each([
	{ account: 'an account that may open the admin', user: MAY_OPEN_ADMIN, landing: '/admin/' },
	{ account: 'an account that may not open the admin', user: MAY_NOT_OPEN_ADMIN, landing: '/' },
	{ account: 'an account the server names no capability for', user: defaultUser, landing: '/' },
])('takes $account to $landing once the password is set', async ({ user, landing }) => {
	server.use(activateOk(user))
	const go = vi.fn()
	renderAsMain(publicAuthScreen(ACTIVATION_LINK, go))

	await setPassword()

	await waitFor(() => expect(go).toHaveBeenCalledWith(landing))
	expect(go).toHaveBeenCalledTimes(1)
})

test('says to wait a minute once the reader spent too many dead links', async () => {
	server.use(
		http.post('/api/auth/activate', () =>
			HttpResponse.json({ error: 'too many attempts, try again later', code: 'link_rate_limited' }, { status: 429 }),
		),
	)
	renderAsMain(publicAuthScreen(ACTIVATION_LINK, vi.fn()))

	await setPassword()

	expect(await screen.findByRole('alert')).toHaveTextContent('Too many attempts. Please wait a minute and try again.')
})

test('shows a visitor with no session the page with no error, asking the server only to set the password', async () => {
	server.use(
		sessionAnonymous(),
		http.get('/api/settings', () => HttpResponse.json({ error: 'no session', code: 'session_absent' }, { status: 401 })),
		activateOk(MAY_OPEN_ADMIN),
	)
	const asked = recordRequests()
	const errors = vi.spyOn(console, 'error')
	onTestFinished(() => errors.mockRestore())
	const go = vi.fn()
	renderAsMain(publicAuthScreen(ACTIVATION_LINK, go))

	await setPassword()
	await waitFor(() => expect(go).toHaveBeenCalledWith('/admin/'))
	await new Promise((resolve) => setTimeout(resolve, QUIET))

	expect(asked).toEqual(['POST /api/auth/activate'])
	expect(screen.queryByRole('alert')).toBeNull()
	expect(errors).not.toHaveBeenCalled()
})

test('reads the activation page in Spanish from the account screens catalogue', async () => {
	setLocaleData(await brickCatalogFor('es-ES'), BRICK_DOMAIN)
	onTestFinished(() => resetLocaleData({}, BRICK_DOMAIN))
	renderAsMain(publicAuthScreen(ACTIVATION_LINK, vi.fn()))

	expect(await screen.findByRole('button', { name: 'Guardar contraseña' })).toBeInTheDocument()
})
