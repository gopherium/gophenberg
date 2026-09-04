// SPDX-License-Identifier: Apache-2.0

import { http, HttpResponse, server } from '@gophenberg/frontend-sdk/testing'
import { screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { expect, test } from 'vitest'

import { renderAt } from './render'

/** Serves the given drift from the drift endpoint. */
function drifting(orphans: unknown[], collisions: unknown[]) {
	server.use(http.get('/api/definitions/drift', () => HttpResponse.json({ orphans, collisions })))
}

const ORPHAN = { subject: 'group', key: 'event-details', origin: 'events', label: 'Event details' }
const COLLISION = { subject: 'group', key: 'extras', origin: 'events', label: 'Extras' }

test('names a group whose plugin no longer declares it', async () => {
	drifting([ORPHAN], [])

	renderAt('/field-groups')

	expect(
		await screen.findByText('Event details came from the events plugin, which no longer declares it.'),
	).toBeInTheDocument()
})

test('names a key a plugin wanted and the site keeps', async () => {
	drifting([], [COLLISION])

	renderAt('/field-groups')

	expect(
		await screen.findByText('The events plugin also declares Extras, and the site keeps the key.'),
	).toBeInTheDocument()
})

test('takes an orphan over as the site own', async () => {
	let sent = ''
	drifting([ORPHAN], [])
	server.use(
		http.post('/api/definitions/adopt', async ({ request }) => {
			sent = await request.text()
			return new HttpResponse(null, { status: 204 })
		}),
	)

	renderAt('/field-groups')
	await userEvent.click(await screen.findByRole('button', { name: 'Adopt' }))

	await screen.findByText('Event details belongs to the site now.')
	expect(JSON.parse(sent)).toEqual({ subject: 'group', key: 'event-details' })
})

test('says why taking an orphan over was turned away', async () => {
	drifting([ORPHAN], [])
	server.use(
		http.post('/api/definitions/adopt', () =>
			HttpResponse.json({ error: 'gone', code: 'group_not_found' }, { status: 404 }),
		),
	)

	renderAt('/field-groups')
	await userEvent.click(await screen.findByRole('button', { name: 'Adopt' }))

	expect(await screen.findByRole('alert')).toBeInTheDocument()
})

test('says taking an orphan over was turned away even with no reason given', async () => {
	drifting([ORPHAN], [])
	server.use(
		http.post('/api/definitions/adopt', () => new HttpResponse('gateway said no', { status: 502 })),
	)

	renderAt('/field-groups')
	await userEvent.click(await screen.findByRole('button', { name: 'Adopt' }))

	expect(await screen.findByRole('alert')).toBeInTheDocument()
})

test('shows no drift when the site cannot be asked for it', async () => {
	server.use(http.get('/api/definitions/drift', () => new HttpResponse(null, { status: 500 })))

	renderAt('/field-groups')

	await screen.findByRole('region', { name: 'Field Groups' })
	expect(screen.queryByRole('button', { name: 'Adopt' })).not.toBeInTheDocument()
})

test('shows nothing when no definition stands apart', async () => {
	drifting([], [])

	renderAt('/field-groups')

	await screen.findByRole('region', { name: 'Field Groups' })
	expect(screen.queryByRole('button', { name: 'Adopt' })).not.toBeInTheDocument()
})
