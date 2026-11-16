// SPDX-License-Identifier: Apache-2.0

import { http, HttpResponse, server } from '@gophenberg/frontend-sdk/testing'
import { act, screen, waitFor, within } from '@testing-library/react'
import { expect, test, vi } from 'vitest'

import { groupsQueryKey } from '../content/groups'
import { typesQueryKey } from '../content/nav'
import { DETAILS, EXTRAS, listing, SUBTITLE } from './groupFieldsFixtures'
import { renderAt } from './render'
import type { QueryClient, QueryKey } from '@tanstack/react-query'

const TYPES_LOST =
	'The content types could not be loaded, so where this group appears, which fields it shadows ' +
	'and what a relation can point at are not shown.'

const TYPES_STALE =
	'The content types could not be refreshed, so where this group appears, which fields it shadows ' +
	'and what a relation can point at may be out of date.'

const OVERLAP = 'subtitle is served by Article details, so Extras does not show it on Posts.'

const PLUGIN_GROUP = { ...EXTRAS, id: 5, key: 'event-details', title: 'Event details', origin: 'events' }

/** Returns the main work of the page once it carries the title. */
async function pageTitled(title: string) {
	const main = await screen.findByRole('main')
	await within(main).findByRole('heading', { level: 2, name: title })
	return main
}

/** Answers every later read of each address with a server error, keeping the console quiet about it. */
function failing(...paths: string[]) {
	vi.spyOn(console, 'error').mockImplementation(() => {})
	server.use(...paths.map((path) => http.get(path, () => new HttpResponse(null, { status: 500 }))))
}

/** Reads the queries under each key again, as the admin does after a write. */
async function readAgain(client: QueryClient, ...keys: QueryKey[]) {
	await act(async () => {
		for (const queryKey of keys) {
			await client.invalidateQueries({ queryKey })
		}
	})
}

test('names a failed first read of the content types above the fields and outside the settings', async () => {
	listing([DETAILS, { ...EXTRAS, fields: [SUBTITLE] }])
	failing('/api/types')
	renderAt('/field-groups/3')
	const main = await pageTitled('Article details')

	const notice = await within(main).findByRole('alert')

	expect(notice).toHaveTextContent(TYPES_LOST)
	expect(within(main).getByRole('region', { name: 'Field group settings' })).not.toContainElement(notice)
	const field = within(main).getByRole('listitem', { name: 'Subtitle' })
	expect(notice.compareDocumentPosition(field) & Node.DOCUMENT_POSITION_FOLLOWING).toBeTruthy()
	expect(within(main).queryByText(OVERLAP)).not.toBeInTheDocument()
})

test('names a failed refresh of the content types and keeps the overlaps the page already warned of', async () => {
	listing([DETAILS, { ...EXTRAS, fields: [SUBTITLE] }])
	const client = renderAt('/field-groups/3')
	const main = await pageTitled('Article details')
	expect(await within(main).findByText(OVERLAP)).toBeInTheDocument()

	failing('/api/types')
	await readAgain(client, typesQueryKey)

	expect(await within(main).findByRole('alert')).toHaveTextContent(TYPES_STALE)
	expect(within(main).getByText(OVERLAP)).toBeInTheDocument()
})

test('names a failed refresh of the content types right after a failed refresh of the groups', async () => {
	listing([DETAILS, EXTRAS])
	const client = renderAt('/field-groups/3')
	const main = await pageTitled('Article details')

	failing('/api/groups', '/api/types')
	await readAgain(client, groupsQueryKey, typesQueryKey)

	await waitFor(() => expect(within(main).getAllByRole('alert')).toHaveLength(2))
	const [groups, types] = within(main).getAllByRole('alert')
	expect(groups).toHaveTextContent('The field groups could not be loaded.')
	expect(types).toHaveTextContent(TYPES_STALE)
})

test('says nothing of the content types on the page of a group a plugin declared', async () => {
	listing([DETAILS, PLUGIN_GROUP])
	failing('/api/types')
	const client = renderAt('/field-groups/5')
	const main = await pageTitled('Event details')

	await waitFor(() => expect(client.getQueryState(typesQueryKey)?.status).toBe('error'))

	expect(within(main).queryByRole('alert')).not.toBeInTheDocument()
})
