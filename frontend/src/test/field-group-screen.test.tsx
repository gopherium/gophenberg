// SPDX-License-Identifier: Apache-2.0

import { http, HttpResponse, server } from '@gophenberg/frontend-sdk/testing'
import { screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { beforeEach, expect, test, vi } from 'vitest'

import { adminBasepath } from '../basepath'
import { gate } from './gate'
import { DETAILS, EXTRAS, listing, POST_TYPE, SUBTITLE } from './groupFieldsFixtures'
import { renderAt, renderRoutedAt } from './render'
import { findToast } from './typeFixtures'

const PLUGIN_GROUP = {
	...EXTRAS,
	id: 5,
	key: 'event-details',
	title: 'Event details',
	position: 3,
	origin: 'events',
	fields: [SUBTITLE],
}

const SUMMARY = { ...SUBTITLE, key: 'summary', label: 'Summary' }

/**
 * Returns the main work of the page once it carries the title.
 * @param title - The title the page heading reads.
 * @returns The main region.
 */
async function pageTitled(title: string) {
	const main = await screen.findByRole('main')
	await within(main).findByRole('heading', { level: 2, name: title })
	return main
}

/**
 * Returns the link back to the Field Groups list in the breadcrumbs of the page.
 * @param main - The main region of the page.
 * @returns The breadcrumb link.
 */
function backToList(main: HTMLElement) {
	const trail = within(main).getByRole('navigation', { name: 'Breadcrumbs' })
	return within(trail).getByRole('link', { name: 'Field Groups' })
}

beforeEach(() => {
	server.use(http.get('/api/types', () => HttpResponse.json({ items: [POST_TYPE] })))
	listing([DETAILS, EXTRAS, PLUGIN_GROUP])
})

test('titles the page with the group title at the second level under a Field Groups breadcrumb', async () => {
	renderAt('/field-groups/3')

	const main = await pageTitled('Article details')

	const back = backToList(main)
	expect(back).toHaveAttribute('href', `${adminBasepath}/field-groups`)
	expect(back).not.toHaveAttribute('aria-current')
	expect(within(main).getAllByRole('heading', { level: 2 })).toHaveLength(1)
})

test('leads back to the Field Groups list from the breadcrumb', async () => {
	const { router } = renderRoutedAt('/field-groups/3')
	const main = await pageTitled('Article details')

	await userEvent.click(backToList(main))

	expect(await screen.findByRole('region', { name: 'Field Groups' })).toBeInTheDocument()
	expect(router.state.location.pathname).toBe('/field-groups')
})

test('keeps the Field Groups row of the sidebar current on the page of a group', async () => {
	renderAt('/field-groups/3')
	await pageTitled('Article details')

	const sidebar = screen.getByRole('region', { name: 'Navigation' })

	expect(within(sidebar).getByRole('link', { name: 'Field Groups' })).toHaveAttribute('aria-current', 'page')
})

test('keeps the header and shows a loading screen under it while the groups are on their way', async () => {
	const answer = gate()
	server.use(
		http.get('/api/groups', async () => {
			await answer.held
			return HttpResponse.json({ items: [DETAILS] })
		}),
	)
	renderAt('/field-groups/3')

	const main = await pageTitled('Field Group')

	const status = await within(main).findByRole('status')
	expect(status).toHaveTextContent('Loading field groups.')
	expect(status.closest('.godmin-loading-screen')).not.toBeNull()
	expect(backToList(main)).toBeInTheDocument()
	answer.release()
	expect(await within(main).findByRole('heading', { level: 2, name: 'Article details' })).toBeInTheDocument()
})

test('titles an address no group holds Field Group and says the group does not exist', async () => {
	renderAt('/field-groups/99')

	const main = await pageTitled('Field Group')

	expect(await within(main).findByRole('alert')).toHaveTextContent(
		'This field group does not exist, and it may have been deleted. Go back to the list to see what is there.',
	)
	expect(within(main).queryByRole('button', { name: 'Add field' })).not.toBeInTheDocument()
})

test('names a failed read of the groups under the title', async () => {
	vi.spyOn(console, 'error').mockImplementation(() => {})
	server.use(http.get('/api/groups', () => new HttpResponse(null, { status: 500 })))
	renderAt('/field-groups/3')

	const main = await pageTitled('Field Group')

	expect(await within(main).findByRole('alert')).toHaveTextContent('The field groups could not be loaded.')
})

test('says a group a plugin declared is read only and offers nothing to change on its page', async () => {
	renderAt('/field-groups/5')

	const main = await pageTitled('Event details')

	expect(
		within(main).getByText(
			'Event details comes from events, so it is read only. Its fields show from the Field Groups list.',
		),
	).toBeInTheDocument()
	expect(within(main).queryByRole('textbox')).not.toBeInTheDocument()
	expect(within(main).queryByRole('button', { name: 'Add field' })).not.toBeInTheDocument()
	expect(within(main).queryByRole('button', { name: 'Move Subtitle elsewhere' })).not.toBeInTheDocument()
	expect(within(main).queryByRole('button', { name: 'Delete Subtitle' })).not.toBeInTheDocument()
	expect(within(main).queryAllByRole('button')).toEqual([])
})

test('shows the fields of a site group in the main work with no dialog over them', async () => {
	renderAt('/field-groups/3')

	const main = await pageTitled('Article details')

	expect(within(main).getByRole('listitem', { name: 'Subtitle' })).toBeInTheDocument()
	expect(within(main).getByRole('listitem', { name: 'Reading time' })).toBeInTheDocument()
	expect(screen.queryByRole('dialog')).not.toBeInTheDocument()
})

test('declares a field from the page, reports it in a toast and shows it once the groups are read again', async () => {
	let sent: unknown
	server.use(
		http.post('/api/groups/3/fields', async ({ request }) => {
			sent = await request.json()
			listing([{ ...DETAILS, fields: [...DETAILS.fields, SUMMARY] }, EXTRAS, PLUGIN_GROUP])
			return HttpResponse.json(SUMMARY, { status: 201 })
		}),
	)
	renderAt('/field-groups/3')
	const main = await pageTitled('Article details')

	await userEvent.type(within(main).getByLabelText('Name'), 'Summary')
	await userEvent.click(within(main).getByRole('button', { name: 'Add field' }))

	await waitFor(() => expect(sent).toMatchObject({ key: 'summary', label: 'Summary' }))
	expect(await findToast('Summary declared.')).toBeInTheDocument()
	expect(await within(main).findByRole('listitem', { name: 'Summary' })).toBeInTheDocument()
})

test('wraps the actions of a field and of a sub field so their buttons keep their words whole', async () => {
	const name = { ...SUBTITLE, key: 'name', label: 'Name' }
	const author = { ...SUBTITLE, key: 'author', label: 'Author', kind: 'section', fields: [name] }
	listing([{ ...DETAILS, fields: [author] }, EXTRAS])
	renderAt('/field-groups/3')

	const main = await pageTitled('Article details')

	for (const field of ['Author', 'Name']) {
		const rename = within(main).getByRole('button', { name: `Rename ${field}` })
		expect(rename.parentElement).toHaveStyle({ flexWrap: 'wrap' })
	}
})

test('lays the add field form out in one row with its button at its own width', async () => {
	renderAt('/field-groups/3')

	const main = await pageTitled('Article details')

	const form = within(main).getByRole('button', { name: 'Add field' }).parentElement
	expect(form).toHaveClass('godmin-form', 'godmin-form--inline')
	expect(within(main).getByLabelText('Name').closest('.godmin-form__row')?.parentElement).toBe(form)
})

test('keeps Add field off while a field is being declared, so Enter sends it once', async () => {
	const answer = gate()
	const sent: unknown[] = []
	server.use(
		http.post('/api/groups/3/fields', async ({ request }) => {
			sent.push(await request.json())
			await answer.held
			return HttpResponse.json(SUMMARY, { status: 201 })
		}),
	)
	renderAt('/field-groups/3')
	const main = await pageTitled('Article details')
	const name = within(main).getByLabelText('Name')
	const add = within(main).getByRole('button', { name: 'Add field' })

	await userEvent.type(name, 'Summary{Enter}')
	await waitFor(() => expect(sent).toHaveLength(1))
	expect(add).toHaveAttribute('aria-disabled', 'true')
	await userEvent.type(name, '{Enter}')
	answer.release()

	expect(await findToast('Summary declared.')).toBeInTheDocument()
	expect(sent).toHaveLength(1)
})

test('names a field that could not be added in a toast as well as above the fields', async () => {
	const said = 'content: the field key is taken'
	server.use(http.post('/api/groups/3/fields', () => HttpResponse.json({ error: said }, { status: 422 })))
	renderAt('/field-groups/3')
	const main = await pageTitled('Article details')

	await userEvent.type(within(main).getByLabelText('Name'), 'Subtitle')
	await userEvent.click(within(main).getByRole('button', { name: 'Add field' }))

	expect(await findToast(`Could not add the field. ${said}`)).toBeInTheDocument()
	expect(within(main).getByRole('alert')).toHaveTextContent(said)
})

const LOCKED = 'content: the field group is locked'

/**
 * Answers a field write with the reason it was rejected.
 * @returns The rejected answer.
 */
function rejectedWrite() {
	return HttpResponse.json({ error: LOCKED }, { status: 422 })
}

const FIELD_WRITES = [
	{
		write: 'a field made required',
		failed: 'Could not save the field.',
		answer: http.patch('/api/groups/3/fields/subtitle', rejectedWrite),
		press: async (main: HTMLElement) => {
			await userEvent.click(within(main).getByRole('button', { name: 'Require Subtitle' }))
		},
	},
	{
		write: 'a field shown in the list',
		failed: 'Could not save the field.',
		answer: http.patch('/api/groups/3/fields/subtitle', rejectedWrite),
		press: async (main: HTMLElement) => {
			await userEvent.click(within(main).getByRole('button', { name: 'Show Subtitle in the list' }))
		},
	},
	{
		write: 'a new order of the fields',
		failed: 'Could not order the fields.',
		answer: http.put('/api/groups/3/fields/order', rejectedWrite),
		press: async (main: HTMLElement) => {
			await userEvent.click(within(main).getByRole('button', { name: 'Move Subtitle down' }))
		},
	},
	{
		write: 'a deleted field',
		failed: 'Could not delete the field.',
		answer: http.delete('/api/groups/3/fields/subtitle', rejectedWrite),
		press: async (main: HTMLElement) => {
			await userEvent.click(within(main).getByRole('button', { name: 'Delete Subtitle' }))
			const asking = await screen.findByRole('alertdialog')
			await userEvent.click(within(asking).getByRole('button', { name: 'Delete the field' }))
		},
	},
]

test.each(FIELD_WRITES)('names what failed in the toast when $write is rejected', async (asked) => {
	server.use(asked.answer)
	renderAt('/field-groups/3')
	const main = await pageTitled('Article details')

	await asked.press(main)

	expect(await findToast(`${asked.failed} ${LOCKED}`)).toBeInTheDocument()
	expect(within(main).getByRole('alert')).toHaveTextContent(LOCKED)
})

test('keeps the fields in view and names the failed read when the groups cannot be read again', async () => {
	vi.spyOn(console, 'error').mockImplementation(() => {})
	server.use(
		http.post('/api/groups/3/fields', () => {
			server.use(http.get('/api/groups', () => new HttpResponse(null, { status: 500 })))
			return HttpResponse.json(SUMMARY, { status: 201 })
		}),
	)
	renderAt('/field-groups/3')
	const main = await pageTitled('Article details')

	await userEvent.type(within(main).getByLabelText('Name'), 'Summary')
	await userEvent.click(within(main).getByRole('button', { name: 'Add field' }))

	expect(await within(main).findByRole('alert')).toHaveTextContent('The field groups could not be loaded.')
	expect(within(main).getByRole('listitem', { name: 'Subtitle' })).toBeInTheDocument()
	expect(within(main).getByRole('button', { name: 'Add field' })).toBeInTheDocument()
})
