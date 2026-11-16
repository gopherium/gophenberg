// SPDX-License-Identifier: Apache-2.0

import { http, HttpResponse, server } from '@gophenberg/frontend-sdk/testing'
import { screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { beforeEach, expect, test } from 'vitest'

import { DETAILS, EXTRAS, listing, POST_TYPE } from './groupFieldsFixtures'
import { renderAt } from './render'

const STALE = 'Someone saved Article details after you opened it.'

const READ_FAILURE = 'The field groups could not be loaded.'

const OTHER_SAVE_STAMP = '2026-08-02T09:30:00Z'

/** Saves a title and active edit on the group's page after another save, which leaves the groups unreadable. */
async function draftBeatenByAnotherSave() {
	server.use(
		http.patch('/api/groups/3', () => {
			server.use(http.get('/api/groups', () => new HttpResponse(null, { status: 500 })))
			return HttpResponse.json({ error: 'content: conflicting update', code: 'content_stale_update' }, { status: 409 })
		}),
	)
	renderAt('/field-groups/3')
	const main = await screen.findByRole('main')
	const details = await within(main).findByRole('region', { name: 'Field group settings' })
	await userEvent.type(within(details).getByRole('textbox', { name: 'Title' }), ' here')
	await userEvent.click(within(details).getByRole('checkbox', { name: 'Active' }))
	await userEvent.click(within(main).getByRole('button', { name: 'Save' }))
	await within(main).findByText(STALE)
	return { main, details }
}

/** Expects the details column to hold the title and the active flag as the draft edited them. */
function expectDraftKept(details: HTMLElement) {
	expect(within(details).getByRole('textbox', { name: 'Title' })).toHaveValue('Article details here')
	expect(within(details).getByRole('checkbox', { name: 'Active' })).not.toBeChecked()
}

beforeEach(() => {
	server.use(http.get('/api/types', () => HttpResponse.json({ items: [POST_TYPE] })))
	listing([DETAILS, EXTRAS])
})

test('loads the saved version on a second try once the groups can be read again, the draft on top', async () => {
	const { main, details } = await draftBeatenByAnotherSave()
	await userEvent.click(within(main).getByRole('button', { name: 'Load saved version' }))
	await within(main).findByText(READ_FAILURE)
	listing([{ ...DETAILS, updated_at: OTHER_SAVE_STAMP }, EXTRAS])

	await userEvent.click(within(main).getByRole('button', { name: 'Load saved version' }))

	await waitFor(() => expect(within(main).queryByText(STALE)).not.toBeInTheDocument())
	expect(within(main).queryByText(READ_FAILURE)).not.toBeInTheDocument()
	expectDraftKept(details)
})
