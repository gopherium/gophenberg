// SPDX-License-Identifier: Apache-2.0

import { http, HttpResponse, server } from '@gophenberg/frontend-sdk/testing'
import { act, screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { beforeEach, expect, onTestFinished, test, vi } from 'vitest'

import { typesQueryKey } from '../content/nav'
import { gate } from './gate'
import { DETAILS, EXTRAS, listing } from './groupFieldsFixtures'
import { renderAt } from './render'
import { GUIDE_TYPE, POST_TYPE } from './typeFixtures'

const TYPES_LOST = 'The content types could not be loaded, so the group has no content type to start on.'

const LIST_TYPES_LOST =
	'The content types could not be loaded, so where these groups appear and which fields they shadow are not shown.'

const LIST_TYPES_STALE =
	'The content types could not be refreshed, so where these groups appear and what they shadow may be out of date.'

const STARTED_ON_POSTS = { title: 'Extras', location: [[{ source: 'content_type', operator: '==', value: 'post' }]] }

/** Records the body of every group the server is asked to create, answering each with the group stored. */
function recordCreates(): unknown[] {
	const sent: unknown[] = []
	server.use(
		http.post('/api/groups', async ({ request }) => {
			sent.push(await request.json())
			return HttpResponse.json(EXTRAS, { status: 201 })
		}),
	)
	return sent
}

/** Answers every later read of the content types with a server error, keeping the console quiet about it. */
function typesFailing() {
	vi.spyOn(console, 'error').mockImplementation(() => {})
	server.use(http.get('/api/types', () => new HttpResponse(null, { status: 500 })))
}

/** Opens the create form from the list of groups and types the title of the new group into it. */
async function openCreateForm(): Promise<HTMLElement> {
	await userEvent.click(await screen.findByRole('button', { name: 'Add Field Group' }))
	const dialog = await screen.findByRole('dialog', { name: 'Create a field group' })
	await userEvent.type(within(dialog).getByLabelText('Title'), 'Extras')
	return dialog
}

beforeEach(() => {
	listing([DETAILS])
})

test('keeps Create off while the content types are on their way, then starts the group on the first one', async () => {
	const typesRead = gate()
	onTestFinished(typesRead.release)
	server.use(
		http.get('/api/types', async () => {
			await typesRead.held
			return HttpResponse.json({ items: [POST_TYPE, GUIDE_TYPE] })
		}),
	)
	const sent = recordCreates()
	renderAt('/field-groups')
	const dialog = await openCreateForm()
	const create = within(dialog).getByRole('button', { name: 'Create' })

	expect(create).toHaveAttribute('aria-disabled', 'true')
	await userEvent.type(within(dialog).getByLabelText('Title'), '{Enter}')
	expect(within(dialog).queryByRole('alert')).not.toBeInTheDocument()
	typesRead.release()

	await waitFor(() => expect(create).not.toHaveAttribute('aria-disabled', 'true'))
	await userEvent.click(create)
	await waitFor(() => expect(sent).toEqual([STARTED_ON_POSTS]))
})

test('says in the form, below the title, that the content types could not be loaded, and sends nothing', async () => {
	typesFailing()
	const sent = recordCreates()
	renderAt('/field-groups')
	await screen.findByText(LIST_TYPES_LOST)
	const dialog = await openCreateForm()
	const create = within(dialog).getByRole('button', { name: 'Create' })

	const notice = within(dialog).getByRole('alert')
	expect(notice).toHaveTextContent(TYPES_LOST)
	const title = within(dialog).getByLabelText('Title')
	expect(title.compareDocumentPosition(notice) & Node.DOCUMENT_POSITION_FOLLOWING).toBeTruthy()
	expect(create).toHaveAttribute('aria-disabled', 'true')
	await userEvent.type(title, '{Enter}')
	await userEvent.click(create)
	await userEvent.click(within(dialog).getByRole('button', { name: 'Cancel' }))

	await waitFor(() => expect(screen.queryByRole('dialog')).not.toBeInTheDocument())
	expect(sent).toEqual([])
})

test('keeps Create on with the content types already read when a later read of them fails', async () => {
	server.use(http.get('/api/types', () => HttpResponse.json({ items: [POST_TYPE, GUIDE_TYPE] })))
	const sent = recordCreates()
	const client = renderAt('/field-groups')
	const table = await screen.findByRole('region', { name: 'Field Groups' })
	await within(table).findByText('Posts')
	typesFailing()
	await act(() => client.invalidateQueries({ queryKey: typesQueryKey }))
	await screen.findByText(LIST_TYPES_STALE)
	const dialog = await openCreateForm()

	expect(within(dialog).queryByRole('alert')).not.toBeInTheDocument()
	await userEvent.click(within(dialog).getByRole('button', { name: 'Create' }))

	await waitFor(() => expect(sent).toEqual([STARTED_ON_POSTS]))
})
