// SPDX-License-Identifier: Apache-2.0

import { createAuthQueryClient } from '@gopherium/react-auth'
import { defaultUser, seedSession } from '@gopherium/react-auth/testing'
import { QueryClientProvider } from '@tanstack/react-query'
import { render, screen } from '@testing-library/react'
import { expect, test } from 'vitest'

import { CHANGE_OTHERS_WORK, MANAGE_USERS, can, useSession } from '../index'
import type { Session } from '../index'

/**
 * Renders what a plugin screen reads about the signed in account.
 * @returns The reported reading.
 */
function PluginScreen() {
	const session = useSession().data
	return (
		<ul>
			<li>{`rank:${session?.rank ?? 'none'}`}</li>
			<li>{`changes:${String(can(session?.rank, CHANGE_OTHERS_WORK))}`}</li>
			<li>{`administers:${String(can(session?.rank, MANAGE_USERS))}`}</li>
		</ul>
	)
}

/**
 * Renders the plugin screen with the given account signed in.
 * @param session - The account the plugin sees.
 */
function renderAs(session: Session | null) {
	const client = createAuthQueryClient({ queries: { retry: false, staleTime: Infinity } })
	seedSession(client, session)
	render(
		<QueryClientProvider client={client}>
			<PluginScreen />
		</QueryClientProvider>,
	)
}

test('a plugin screen reads the rank the session carries', async () => {
	renderAs({ ...defaultUser, rank: 'editor' })

	expect(await screen.findByText('rank:editor')).toBeInTheDocument()
})

test('a plugin screen asks capabilities rather than comparing ranks', async () => {
	renderAs({ ...defaultUser, rank: 'editor' })

	expect(await screen.findByText('changes:true')).toBeInTheDocument()
	expect(screen.getByText('administers:false')).toBeInTheDocument()
})

test('a plugin screen holds nothing without a session', async () => {
	renderAs(null)

	expect(await screen.findByText('rank:none')).toBeInTheDocument()
	expect(screen.getByText('changes:false')).toBeInTheDocument()
})
