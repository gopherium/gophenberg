// SPDX-License-Identifier: Apache-2.0

import { http, HttpResponse, server } from '@gophenberg/frontend-sdk/testing'
import { formatDate } from '@gopherium/gottext'
import { screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { resetLocaleData, setLocaleData } from '@wordpress/i18n'
import { beforeEach, expect, onTestFinished, test, vi } from 'vitest'

import { adminBasepath } from '../basepath'
import { catalogFor } from '../i18n/catalog'
import { gate } from './gate'
import {
	DETAILS,
	EXTRAS,
	fieldRow,
	listing,
	NO_FIELDS,
	openField,
	POST_TYPE,
	showTab,
	SUBTITLE,
} from './groupFieldsFixtures'
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

const OTHER_SAVE_STAMP = '2026-08-02T09:30:00Z'

const ON_RECIPES = [[{ source: 'content_type', operator: '==', value: 'recipe' }]]

const SOURCES = {
	items: [
		{
			source: 'content_type',
			operators: ['==', '!='],
			values: [
				{ value: '*', label: 'Any content type' },
				{ value: 'post', label: 'Posts' },
				{ value: 'recipe', label: 'Recipes' },
			],
		},
	],
}

const MAKER = { ...SUBTITLE, key: 'maker', label: 'Maker', kind: 'relation', relates_to: 'post' }

const CARS = {
	...EXTRAS,
	id: 6,
	key: 'cars',
	title: 'Cars',
	position: 4,
	fields: [MAKER, { ...MAKER, key: 'rival', label: 'Rival' }],
}

const LINKED_FROM = {
	...SUBTITLE,
	key: 'linked-from',
	label: 'Linked from',
	kind: 'backlinks',
	settings: { source_group: 'cars', source_field: ['maker'] },
}

/**
 * Returns the details column of the page.
 * @param main - The main region of the page.
 * @returns The details region.
 */
function detailsOf(main: HTMLElement) {
	return within(main).getByRole('region', { name: 'Field group settings' })
}

/**
 * Returns the notice the header Save leaves, once it shows above the fields and outside the details.
 * @param main - The main region of the page.
 * @returns The notice.
 */
async function saveNotice(main: HTMLElement) {
	const notice = await within(main).findByRole('alert')
	expect(detailsOf(main)).not.toContainElement(notice)
	const field = await fieldRow(main, 'Subtitle')
	expect(notice.compareDocumentPosition(field) & Node.DOCUMENT_POSITION_FOLLOWING).toBeTruthy()
	return notice
}

/**
 * Returns the Save button in the header of the page.
 * @param main - The main region of the page.
 * @returns The Save button.
 */
function saveOf(main: HTMLElement) {
	return within(main).getByRole('button', { name: 'Save' })
}

/**
 * Answers the group edit as the server stores it, lists the group as stored, and records every body the page sent.
 * @param sent - The bodies sent so far.
 * @param stored - The group the edit answers with.
 */
function storing(sent: unknown[], stored: typeof DETAILS) {
	server.use(
		http.patch('/api/groups/3', async ({ request }) => {
			sent.push(await request.json())
			listing([stored, EXTRAS, PLUGIN_GROUP])
			return HttpResponse.json(stored)
		}),
	)
}

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
 * Returns the Label box of the add field form, the one box of that name shown while no field stands open.
 * @param main - The main region of the page.
 * @param name - The name the box reads.
 * @returns The box.
 */
function labelBox(main: HTMLElement, name = 'Label') {
	return within(main).getByRole('textbox', { name })
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

	expect(await fieldRow(main, 'Subtitle')).toBeInTheDocument()
	expect(await fieldRow(main, 'Reading time')).toBeInTheDocument()
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

	await userEvent.type(labelBox(main), 'Summary')
	await userEvent.click(within(main).getByRole('button', { name: 'Add field' }))

	await waitFor(() => expect(sent).toMatchObject({ key: 'summary', label: 'Summary' }))
	expect(await findToast('Summary declared.')).toBeInTheDocument()
	expect(await fieldRow(main, 'Summary')).toBeInTheDocument()
})

test('lays the add field form out in one row with its button at its own width', async () => {
	renderAt('/field-groups/3')

	const main = await pageTitled('Article details')

	const form = within(main).getByRole('button', { name: 'Add field' }).parentElement
	expect(form).toHaveClass('godmin-form', 'godmin-form--inline')
	expect(labelBox(main).closest('.godmin-form__row')?.parentElement).toBe(form)
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
	const name = labelBox(main)
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

	await userEvent.type(labelBox(main), 'Subtitle')
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

test('names the field and why in the toast when the header Save of a field it made required is rejected', async () => {
	server.use(http.patch('/api/groups/3/fields/subtitle', rejectedWrite))
	renderAt('/field-groups/3')
	const main = await pageTitled('Article details')
	const part = await openField(main, 'Subtitle')

	await showTab(part, 'Validation')
	await userEvent.click(within(part).getByRole('checkbox', { name: 'Required' }))
	await userEvent.click(saveOf(main))

	expect(await findToast(`Could not save Subtitle. ${LOCKED}`)).toBeInTheDocument()
	expect(within(part).getByRole('checkbox', { name: 'Required' })).toBeChecked()
	expect(within(part).getByRole('alert')).toHaveTextContent(LOCKED)
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

	await userEvent.type(labelBox(main), 'Summary')
	await userEvent.click(within(main).getByRole('button', { name: 'Add field' }))

	expect(await within(main).findByRole('alert')).toHaveTextContent('The field groups could not be loaded.')
	expect(await fieldRow(main, 'Subtitle')).toBeInTheDocument()
	expect(within(main).getByRole('button', { name: 'Add field' })).toBeInTheDocument()
})

test('holds the title, the active flag, the rules and the day the group was made beside the fields', async () => {
	renderAt('/field-groups/3')

	const main = await pageTitled('Article details')
	const details = detailsOf(main)

	expect(within(details).getByRole('textbox', { name: 'Title' })).toHaveValue('Article details')
	expect(within(details).getByRole('checkbox', { name: 'Active' })).toBeChecked()
	expect(within(details).getByRole('heading', { level: 3, name: 'Location rules' })).toBeInTheDocument()
	expect(within(details).getByRole('group', { name: 'Rule 1 of set 1' })).toBeInTheDocument()
	expect(within(details).getByText(`Created ${formatDate(DETAILS.created_at)}`)).toBeInTheDocument()
	expect(details).not.toContainElement(await fieldRow(main, 'Subtitle'))
})

test('keeps Save off until a part of the group changes, and off again once the change is undone', async () => {
	renderAt('/field-groups/3')
	const main = await pageTitled('Article details')
	const title = within(detailsOf(main)).getByRole('textbox', { name: 'Title' })
	expect(saveOf(main)).toHaveAttribute('aria-disabled', 'true')

	await userEvent.type(title, ' here')
	expect(saveOf(main)).not.toHaveAttribute('aria-disabled', 'true')

	await userEvent.clear(title)
	await userEvent.type(title, 'Article details')
	expect(saveOf(main)).toHaveAttribute('aria-disabled', 'true')
})

test('keeps Save off while the title is blank, and still asks before that draft is left', async () => {
	const sent: unknown[] = []
	storing(sent, DETAILS)
	const { router } = renderRoutedAt('/field-groups/3')
	const main = await pageTitled('Article details')
	const title = within(detailsOf(main)).getByRole('textbox', { name: 'Title' })

	await userEvent.clear(title)
	expect(saveOf(main)).toHaveAttribute('aria-disabled', 'true')
	await userEvent.type(title, '   ')
	expect(saveOf(main)).toHaveAttribute('aria-disabled', 'true')
	await userEvent.click(saveOf(main))
	await userEvent.click(backToList(main))

	expect(await screen.findByRole('alertdialog', { name: 'Discard changes?' })).toBeInTheDocument()
	expect(router.state.location.pathname).toBe('/field-groups/3')
	expect(sent).toEqual([])
})

test('titles the page with the title as it is typed, and Field Group while it is empty or only spaces', async () => {
	renderAt('/field-groups/3')
	const main = await pageTitled('Article details')
	const title = within(detailsOf(main)).getByRole('textbox', { name: 'Title' })

	await userEvent.clear(title)
	expect(within(main).getByRole('heading', { level: 2, name: 'Field Group' })).toBeInTheDocument()

	await userEvent.type(title, '   ')
	expect(within(main).getByRole('heading', { level: 2, name: 'Field Group' })).toBeInTheDocument()

	await userEvent.clear(title)
	await userEvent.type(title, 'Car details')
	expect(within(main).getByRole('heading', { level: 2, name: 'Car details' })).toBeInTheDocument()
})

test('sends the title alone with the stamp the page read, then says so and turns Save off', async () => {
	const sent: unknown[] = []
	storing(sent, { ...DETAILS, title: 'Car details', updated_at: OTHER_SAVE_STAMP })
	renderAt('/field-groups/3')
	const main = await pageTitled('Article details')
	const title = within(detailsOf(main)).getByRole('textbox', { name: 'Title' })

	await userEvent.clear(title)
	await userEvent.type(title, 'Car details')
	await userEvent.click(saveOf(main))

	await waitFor(() => expect(sent).toEqual([{ title: 'Car details', updated_at: DETAILS.updated_at }]))
	expect(await findToast('Car details updated.')).toBeInTheDocument()
	await waitFor(() => expect(saveOf(main)).toHaveAttribute('aria-disabled', 'true'))
	expect(title).toHaveValue('Car details')
})

test('sends the active flag alone once the box is cleared', async () => {
	const sent: unknown[] = []
	storing(sent, { ...DETAILS, active: false })
	renderAt('/field-groups/3')
	const main = await pageTitled('Article details')

	await userEvent.click(within(detailsOf(main)).getByRole('checkbox', { name: 'Active' }))
	await userEvent.click(saveOf(main))

	await waitFor(() => expect(sent).toEqual([{ active: false, updated_at: DETAILS.updated_at }]))
})

test('sends the rules alone once a rule changes', async () => {
	server.use(http.get('/api/groups/params', () => HttpResponse.json(SOURCES)))
	const sent: unknown[] = []
	storing(sent, { ...DETAILS, location: ON_RECIPES })
	renderAt('/field-groups/3')
	const main = await pageTitled('Article details')

	const rule = within(detailsOf(main)).getByRole('group', { name: 'Rule 1 of set 1' })
	await userEvent.click(within(rule).getByLabelText('Value'))
	await userEvent.click(await screen.findByRole('option', { name: 'Recipes' }))
	await userEvent.click(saveOf(main))

	await waitFor(() => expect(sent).toEqual([{ location: ON_RECIPES, updated_at: DETAILS.updated_at }]))
})

test('asks before the breadcrumb leaves a draft, stays on Keep editing and leaves on Discard', async () => {
	const { router } = renderRoutedAt('/field-groups/3')
	const main = await pageTitled('Article details')
	const title = within(detailsOf(main)).getByRole('textbox', { name: 'Title' })
	await userEvent.type(title, ' here')

	await userEvent.click(backToList(main))
	const asking = await screen.findByRole('alertdialog', { name: 'Discard changes?' })
	expect(within(asking).getByText('The changes to this field group are not saved.')).toBeInTheDocument()
	await userEvent.click(within(asking).getByRole('button', { name: 'Keep editing' }))

	await waitFor(() => expect(screen.queryByRole('alertdialog')).not.toBeInTheDocument())
	expect(router.state.location.pathname).toBe('/field-groups/3')
	expect(title).toHaveValue('Article details here')

	await userEvent.click(backToList(main))
	await userEvent.click(within(await screen.findByRole('alertdialog')).getByRole('button', { name: 'Discard' }))

	expect(await screen.findByRole('region', { name: 'Field Groups' })).toBeInTheDocument()
	expect(router.state.location.pathname).toBe('/field-groups')
})

test('leaves by the breadcrumb without asking while no draft is held', async () => {
	const { router } = renderRoutedAt('/field-groups/3')
	const main = await pageTitled('Article details')

	await userEvent.click(backToList(main))

	expect(await screen.findByRole('region', { name: 'Field Groups' })).toBeInTheDocument()
	expect(router.state.location.pathname).toBe('/field-groups')
	expect(screen.queryByRole('alertdialog')).not.toBeInTheDocument()
})

test('keeps the draft and names why above the fields and in a toast when the save is rejected', async () => {
	const said = 'content: another group already claims those items'
	server.use(
		http.patch('/api/groups/3', () => HttpResponse.json({ error: said, code: 'group_collides' }, { status: 422 })),
	)
	renderAt('/field-groups/3')
	const main = await pageTitled('Article details')
	const title = within(detailsOf(main)).getByRole('textbox', { name: 'Title' })

	await userEvent.type(title, ' here')
	await userEvent.click(saveOf(main))

	expect(await saveNotice(main)).toHaveTextContent(said)
	expect(title).toHaveValue('Article details here')
	expect(saveOf(main)).not.toHaveAttribute('aria-disabled', 'true')
	expect(await findToast(`Could not save the field group. ${said}`)).toBeInTheDocument()
})

test('keeps the draft on a save someone else beat, then moves it onto theirs with its changes on top', async () => {
	server.use(http.get('/api/groups/params', () => HttpResponse.json(SOURCES)))
	let listed: unknown[] = [DETAILS, EXTRAS]
	const sent: unknown[] = []
	server.use(http.get('/api/groups', () => HttpResponse.json({ items: listed })))
	server.use(
		http.patch('/api/groups/3', async ({ request }) => {
			const body = (await request.json()) as { updated_at?: string }
			if (body.updated_at === DETAILS.updated_at) {
				const theirs = { ...DETAILS, active: false, location: ON_RECIPES, updated_at: OTHER_SAVE_STAMP }
				listed = [theirs, EXTRAS]
				return HttpResponse.json({ error: 'content: conflicting update', code: 'content_stale_update' }, { status: 409 })
			}
			sent.push(body)
			return HttpResponse.json(DETAILS)
		}),
	)
	renderAt('/field-groups/3')
	const main = await pageTitled('Article details')
	const details = detailsOf(main)
	const title = within(details).getByRole('textbox', { name: 'Title' })
	await userEvent.clear(title)
	await userEvent.type(title, 'Car details')
	const rule = within(details).getByRole('group', { name: 'Rule 1 of set 1' })
	await userEvent.click(within(rule).getByLabelText('Value'))
	await userEvent.click(await screen.findByRole('option', { name: 'Any content type' }))

	await userEvent.click(saveOf(main))

	const notice = await saveNotice(main)
	expect(notice).toHaveTextContent('Someone saved Article details after you opened it.')
	const toast = 'Could not save the field group. Someone saved Article details after you opened it.'
	expect(await findToast(toast)).toBeInTheDocument()
	expect(title).toHaveValue('Car details')
	expect(within(details).getByRole('checkbox', { name: 'Active' })).toBeChecked()

	await userEvent.click(within(notice).getByRole('button', { name: 'Load saved version' }))

	await waitFor(() => expect(within(details).getByRole('checkbox', { name: 'Active' })).not.toBeChecked())
	expect(title).toHaveValue('Car details')
	expect(within(rule).getByLabelText('Value')).toHaveTextContent('Recipes')
	expect(within(details).getByText('Changed elsewhere too. Your edit was not kept.')).toBeInTheDocument()
	expect(within(main).queryByText('Someone saved Article details after you opened it.')).not.toBeInTheDocument()

	await userEvent.click(saveOf(main))

	await waitFor(() => expect(sent).toEqual([{ title: 'Car details', updated_at: OTHER_SAVE_STAMP }]))
})

test('stores a Linked from pick in the same Save that renames its field, on the stamp that rename left', async () => {
	let listed: unknown[] = [{ ...DETAILS, fields: [LINKED_FROM] }, CARS]
	const renamed = { ...LINKED_FROM, label: 'Linked cars', updated_at: OTHER_SAVE_STAMP }
	const sent: unknown[] = []
	server.use(http.get('/api/groups', () => HttpResponse.json({ items: listed })))
	server.use(
		http.patch('/api/groups/3/fields/linked-from', async ({ request }) => {
			sent.push(await request.json())
			listed = [{ ...DETAILS, fields: [renamed] }, CARS]
			return HttpResponse.json(renamed)
		}),
		http.patch('/api/groups/3', async ({ request }) => {
			const body = (await request.json()) as { backlinks?: Record<string, { updated_at?: string }> }
			sent.push(body)
			if (body.backlinks?.['linked-from'].updated_at !== OTHER_SAVE_STAMP) {
				return HttpResponse.json({ error: 'content: conflicting update', code: 'content_stale_update' }, { status: 409 })
			}
			return HttpResponse.json({ ...DETAILS, fields: [renamed] })
		}),
	)
	renderAt('/field-groups/3')
	const main = await pageTitled('Article details')
	const reading = within(detailsOf(main)).getByRole('group', { name: 'Linked from' })
	await userEvent.click(within(reading).getByRole('combobox', { name: 'Through' }))
	await userEvent.click(await screen.findByRole('option', { name: 'Rival' }))

	const part = await openField(main, 'Linked from')
	const box = within(part).getByRole('textbox', { name: 'Label' })
	await userEvent.clear(box)
	await userEvent.type(box, 'Linked cars')
	await userEvent.click(saveOf(main))

	expect(await findToast('Article details updated.')).toBeInTheDocument()
	expect(sent).toEqual([
		{ label: 'Linked cars', required: false, settings: LINKED_FROM.settings, updated_at: LINKED_FROM.updated_at },
		{
			backlinks: { 'linked-from': { source_group: 'cars', source_field: ['rival'], updated_at: OTHER_SAVE_STAMP } },
			updated_at: DETAILS.updated_at,
		},
	])
	expect(await fieldRow(main, 'Linked cars')).toBeInTheDocument()
})

test('draws no Save and no details on the page of a group a plugin declared', async () => {
	renderAt('/field-groups/5')

	const main = await pageTitled('Event details')

	expect(within(main).queryByRole('button', { name: 'Save' })).not.toBeInTheDocument()
	expect(within(main).queryByRole('region', { name: 'Field group settings' })).not.toBeInTheDocument()
})

test('keeps the fields in view after a save that answers the group without them', async () => {
	const reading = gate()
	let saved = false
	server.use(
		http.get('/api/groups', async () => {
			if (saved) {
				await reading.held
			}
			return HttpResponse.json({ items: [DETAILS, EXTRAS, PLUGIN_GROUP] })
		}),
		http.patch('/api/groups/3', () => {
			saved = true
			return HttpResponse.json({ ...DETAILS, title: 'Car details', updated_at: OTHER_SAVE_STAMP, fields: [] })
		}),
	)
	renderAt('/field-groups/3')
	const main = await pageTitled('Article details')
	const title = within(detailsOf(main)).getByRole('textbox', { name: 'Title' })

	await userEvent.clear(title)
	await userEvent.type(title, 'Car details')
	await userEvent.click(saveOf(main))

	expect(await findToast('Car details updated.')).toBeInTheDocument()
	expect(await fieldRow(main, 'Subtitle')).toBeInTheDocument()
	expect(within(main).queryByRole('heading', { name: NO_FIELDS })).not.toBeInTheDocument()
	reading.release()
})

test('forgets a Linked from pick once its field leaves the group, and saves the other parts alone', async () => {
	let listed: unknown[] = [{ ...DETAILS, fields: [LINKED_FROM] }, CARS]
	const sent: unknown[] = []
	server.use(
		http.get('/api/groups', () => HttpResponse.json({ items: listed })),
		http.delete('/api/groups/3/fields/linked-from', () => {
			listed = [{ ...DETAILS, fields: [] }, CARS]
			return new HttpResponse(null, { status: 204 })
		}),
		http.patch('/api/groups/3', async ({ request }) => {
			sent.push(await request.json())
			return HttpResponse.json({ ...DETAILS, title: 'Article details here', fields: [] })
		}),
	)
	renderAt('/field-groups/3')
	const main = await pageTitled('Article details')
	const reading = within(detailsOf(main)).getByRole('group', { name: 'Linked from' })
	await userEvent.click(within(reading).getByRole('combobox', { name: 'Through' }))
	await userEvent.click(await screen.findByRole('option', { name: 'Rival' }))
	expect(saveOf(main)).not.toHaveAttribute('aria-disabled', 'true')

	await userEvent.click(within(main).getByRole('button', { name: 'Delete Linked from' }))
	await userEvent.click(within(await screen.findByRole('alertdialog')).getByRole('button', { name: 'Delete the field' }))
	expect(await within(main).findByRole('heading', { name: NO_FIELDS })).toBeInTheDocument()

	expect(saveOf(main)).toHaveAttribute('aria-disabled', 'true')
	await userEvent.type(within(detailsOf(main)).getByRole('textbox', { name: 'Title' }), ' here')
	await userEvent.click(saveOf(main))

	await waitFor(() => expect(sent).toEqual([{ title: 'Article details here', updated_at: DETAILS.updated_at }]))
})

test('keeps the stale notice and names the failed read when the saved version cannot be loaded', async () => {
	vi.spyOn(console, 'error').mockImplementation(() => {})
	server.use(
		http.patch('/api/groups/3', () => {
			server.use(http.get('/api/groups', () => new HttpResponse(null, { status: 500 })))
			return HttpResponse.json({ error: 'content: conflicting update', code: 'content_stale_update' }, { status: 409 })
		}),
	)
	renderAt('/field-groups/3')
	const main = await pageTitled('Article details')
	const details = detailsOf(main)
	await userEvent.type(within(details).getByRole('textbox', { name: 'Title' }), ' here')
	await userEvent.click(saveOf(main))

	await userEvent.click(within(await saveNotice(main)).getByRole('button', { name: 'Load saved version' }))

	await within(main).findAllByText('The field groups could not be loaded.')
	expect(within(main).getAllByText('The field groups could not be loaded.')).toHaveLength(1)
	expect(within(main).getByText('Someone saved Article details after you opened it.')).toBeInTheDocument()
	expect(within(details).queryByRole('alert')).not.toBeInTheDocument()
	expect(within(details).getByRole('textbox', { name: 'Title' })).toHaveValue('Article details here')
})

test('says only that the group does not exist once the saved version shows someone deleted it', async () => {
	let listed: unknown[] = [DETAILS, EXTRAS]
	server.use(
		http.get('/api/groups', () => HttpResponse.json({ items: listed })),
		http.patch('/api/groups/3', () => {
			listed = [EXTRAS]
			return HttpResponse.json({ error: 'content: conflicting update', code: 'content_stale_update' }, { status: 409 })
		}),
	)
	renderAt('/field-groups/3')
	const main = await pageTitled('Article details')
	const details = detailsOf(main)
	await userEvent.type(within(details).getByRole('textbox', { name: 'Title' }), ' here')
	await userEvent.click(saveOf(main))
	let failed = false
	const watching = new MutationObserver(() => {
		failed ||= within(main).queryByText('The field groups could not be loaded.') !== null
	})
	watching.observe(main, { childList: true, subtree: true })
	onTestFinished(() => watching.disconnect())

	await userEvent.click(await within(main).findByRole('button', { name: 'Load saved version' }))

	expect(await within(main).findByText(/^This field group does not exist/)).toBeInTheDocument()
	expect(within(main).getByRole('heading', { level: 2, name: 'Field Group' })).toBeInTheDocument()
	expect(within(main).queryByRole('button', { name: 'Save' })).not.toBeInTheDocument()
	expect(failed).toBe(false)
})

test('drops the line saying a part changed elsewhere once that part is edited again', async () => {
	let listed: unknown[] = [DETAILS, EXTRAS]
	server.use(
		http.get('/api/groups', () => HttpResponse.json({ items: listed })),
		http.patch('/api/groups/3', () => {
			listed = [{ ...DETAILS, title: 'Car details', updated_at: OTHER_SAVE_STAMP }, EXTRAS]
			return HttpResponse.json({ error: 'content: conflicting update', code: 'content_stale_update' }, { status: 409 })
		}),
	)
	renderAt('/field-groups/3')
	const main = await pageTitled('Article details')
	const details = detailsOf(main)
	const title = within(details).getByRole('textbox', { name: 'Title' })
	await userEvent.type(title, ' here')
	await userEvent.click(saveOf(main))
	await userEvent.click(await within(main).findByRole('button', { name: 'Load saved version' }))
	expect(await within(details).findByText('Changed elsewhere too. Your edit was not kept.')).toBeInTheDocument()

	await userEvent.type(title, ' again')

	expect(within(details).queryByText('Changed elsewhere too. Your edit was not kept.')).not.toBeInTheDocument()
})

test('takes the notice of a rejected save away once the draft is undone', async () => {
	const said = 'content: another group already claims those items'
	server.use(
		http.patch('/api/groups/3', () => HttpResponse.json({ error: said, code: 'group_collides' }, { status: 422 })),
	)
	renderAt('/field-groups/3')
	const main = await pageTitled('Article details')
	const title = within(detailsOf(main)).getByRole('textbox', { name: 'Title' })
	await userEvent.type(title, ' here')
	await userEvent.click(saveOf(main))
	expect(await within(main).findByRole('alert')).toHaveTextContent(said)

	await userEvent.clear(title)
	await userEvent.type(title, 'Article details')

	expect(within(main).queryByRole('alert')).not.toBeInTheDocument()
})

test('takes the stale notice away once the draft is undone', async () => {
	server.use(
		http.patch('/api/groups/3', () =>
			HttpResponse.json({ error: 'content: conflicting update', code: 'content_stale_update' }, { status: 409 }),
		),
	)
	renderAt('/field-groups/3')
	const main = await pageTitled('Article details')
	const title = within(detailsOf(main)).getByRole('textbox', { name: 'Title' })
	await userEvent.type(title, ' here')
	await userEvent.click(saveOf(main))
	expect(await within(main).findByRole('alert')).toHaveTextContent('Someone saved Article details')

	await userEvent.clear(title)
	await userEvent.type(title, 'Article details')

	expect(within(main).queryByRole('alert')).not.toBeInTheDocument()
})

test('heads the add field form with its own section title', async () => {
	renderAt('/field-groups/3')

	const main = await pageTitled('Article details')

	const heading = within(main).getByRole('heading', { level: 3, name: 'Add a field' })
	expect(heading.parentElement).toContainElement(within(main).getByRole('button', { name: 'Add field' }))
})

test('leaves a large gap between the fields table and the add field section under it', async () => {
	renderAt('/field-groups/3')

	const main = await pageTitled('Article details')

	const body = within(main).getByRole('region', { name: 'Fields' }).parentElement as HTMLElement
	expect(body).toContainElement(within(main).getByRole('heading', { level: 3, name: 'Add a field' }))
	expect(body.style.gap).toMatch(/^var\(--wpds-dimension-gap-lg[,)]/)
})

const READERS = [
	{
		locale: 'es-ES',
		title: 'Grupo de campos',
		list: 'Grupos de campos',
		missing: 'Este grupo de campos no existe, y puede que se haya borrado. Vuelve a la lista para ver qué hay.',
		declared:
			'Event details viene de events, así que es de solo lectura. ' +
			'Sus campos se muestran desde la lista Grupos de campos.',
	},
	{
		locale: 'fr-FR',
		title: 'Groupe de champs',
		list: 'Groupes de champs',
		missing:
			"Ce groupe de champs n'existe pas, et il a peut-être été supprimé. " +
			"Revenez à la liste pour voir ce qui s'y trouve.",
		declared:
			'Event details vient de events, donc il est en lecture seule. ' +
			"Ses champs s'affichent depuis la liste Groupes de champs.",
	},
]

const ADD_READERS = [
	{ locale: 'es-ES', label: 'Etiqueta', add: 'Añadir campo', failed: 'No se ha podido añadir el campo.' },
	{ locale: 'fr-FR', label: 'Libellé', add: 'Ajouter un champ', failed: 'Impossible d’ajouter le champ.' },
]

test.each(ADD_READERS)('names a field that could not be added in its toast in $locale', async (said) => {
	setLocaleData(await catalogFor(said.locale), 'gophenberg')
	onTestFinished(() => resetLocaleData({}, 'gophenberg'))
	server.use(http.post('/api/groups/3/fields', rejectedWrite))
	renderAt('/field-groups/3')
	const main = await pageTitled('Article details')

	await userEvent.type(labelBox(main, said.label), 'Subtitle')
	await userEvent.click(within(main).getByRole('button', { name: said.add }))

	expect(await findToast(`${said.failed} ${LOCKED}`)).toBeInTheDocument()
})

test.each(READERS)('titles an address no group holds and says the group does not exist in $locale', async (said) => {
	setLocaleData(await catalogFor(said.locale), 'gophenberg')
	onTestFinished(() => resetLocaleData({}, 'gophenberg'))
	renderAt('/field-groups/99')

	const main = await pageTitled(said.title)

	expect(await within(main).findByRole('alert')).toHaveTextContent(said.missing)
	const trail = within(main).getByRole('navigation')
	expect(within(trail).getByRole('link', { name: said.list })).toBeInTheDocument()
})

const SETTINGS_READERS = [
	{
		locale: 'es-ES',
		list: 'Grupos de campos',
		settings: 'Ajustes del grupo de campos',
		rules: 'Reglas de ubicación',
		adding: 'Añadir un campo',
		active: 'Activo',
		created: 'Creado el',
		ask: '¿Descartar los cambios?',
		lose: 'Los cambios de este grupo de campos no se han guardado.',
		keep: 'Seguir editando',
		discard: 'Descartar',
	},
	{
		locale: 'fr-FR',
		list: 'Groupes de champs',
		settings: 'Réglages du groupe de champs',
		rules: "Règles d'emplacement",
		adding: 'Ajouter un champ',
		active: 'Actif',
		created: 'Créé le',
		ask: 'Abandonner les modifications ?',
		lose: 'Les modifications de ce groupe de champs ne sont pas enregistrées.',
		keep: 'Continuer à modifier',
		discard: 'Abandonner',
	},
]

test.each(SETTINGS_READERS)('names the details of a group and asks before a draft is left in $locale', async (said) => {
	setLocaleData(await catalogFor(said.locale), 'gophenberg')
	onTestFinished(() => resetLocaleData({}, 'gophenberg'))
	renderAt('/field-groups/3')
	const main = await pageTitled('Article details')

	const details = within(main).getByRole('region', { name: said.settings })
	expect(within(details).getByRole('heading', { level: 3, name: said.rules })).toBeInTheDocument()
	expect(within(main).getByRole('heading', { level: 3, name: said.adding })).toBeInTheDocument()
	expect(within(details).getByText(`${said.created} ${formatDate(DETAILS.created_at)}`)).toBeInTheDocument()
	await userEvent.click(within(details).getByRole('checkbox', { name: said.active }))
	await userEvent.click(within(within(main).getByRole('navigation')).getByRole('link', { name: said.list }))

	const asking = await screen.findByRole('alertdialog', { name: said.ask })
	expect(within(asking).getByText(said.lose)).toBeInTheDocument()
	expect(within(asking).getByRole('button', { name: said.keep })).toBeInTheDocument()
	expect(within(asking).getByRole('button', { name: said.discard })).toBeInTheDocument()
})

const STALE_READERS = [
	{
		locale: 'es-ES',
		settings: 'Ajustes del grupo de campos',
		save: 'Guardar',
		load: 'Cargar la versión guardada',
		lost: 'También se cambió en otro sitio. Tu cambio no se ha conservado.',
	},
	{
		locale: 'fr-FR',
		settings: 'Réglages du groupe de champs',
		save: 'Enregistrer',
		load: 'Charger la version enregistrée',
		lost: "Modifié ailleurs aussi. Votre modification n'a pas été conservée.",
	},
]

test.each(STALE_READERS)('offers the version someone else saved and names the part it won in $locale', async (said) => {
	setLocaleData(await catalogFor(said.locale), 'gophenberg')
	onTestFinished(() => resetLocaleData({}, 'gophenberg'))
	let listed: unknown[] = [DETAILS, EXTRAS]
	server.use(http.get('/api/groups', () => HttpResponse.json({ items: listed })))
	server.use(
		http.patch('/api/groups/3', () => {
			listed = [{ ...DETAILS, title: 'Car details', updated_at: OTHER_SAVE_STAMP }, EXTRAS]
			return HttpResponse.json({ error: 'content: conflicting update', code: 'content_stale_update' }, { status: 409 })
		}),
	)
	renderAt('/field-groups/3')
	const main = await pageTitled('Article details')
	const details = within(main).getByRole('region', { name: said.settings })
	await userEvent.type(within(details).getByRole('textbox'), ' here')
	await userEvent.click(within(main).getByRole('button', { name: said.save }))

	await userEvent.click(await within(main).findByRole('button', { name: said.load }))

	expect(await within(details).findByText(said.lost)).toBeInTheDocument()
	expect(within(details).getByRole('textbox')).toHaveValue('Car details')
})

test.each(READERS)('says a group a plugin declared is read only in $locale', async (said) => {
	setLocaleData(await catalogFor(said.locale), 'gophenberg')
	onTestFinished(() => resetLocaleData({}, 'gophenberg'))
	renderAt('/field-groups/5')

	const main = await pageTitled('Event details')

	expect(within(main).getByText(said.declared)).toBeInTheDocument()
})
