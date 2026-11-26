// SPDX-License-Identifier: Apache-2.0

import { http, HttpResponse, server } from '@gophenberg/frontend-sdk/testing'
import { screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { resetLocaleData, setLocaleData } from '@wordpress/i18n'
import { expect, onTestFinished, test, vi } from 'vitest'

import { catalogFor } from '../i18n/catalog'
import { gate } from './gate'
import {
	addFormOf,
	DETAILS,
	EXTRAS,
	FEATURES,
	listing,
	openField,
	openFields,
	serveFieldGroups,
	SUBTITLE,
} from './groupFieldsFixtures'
import { renderAt } from './render'
import { siteSettings } from './siteSettings'
import { findToast, waitBusy } from './typeFixtures'

/** A time limit far shorter than any test. */
const SHORT_LIMIT = 20

serveFieldGroups()

test('declares a field into the group it was opened from, its key made from the Label typed', async () => {
	let sent: unknown
	server.use(
		http.post('/api/groups/3/fields', async ({ request }) => {
			sent = await request.json()
			return HttpResponse.json(SUBTITLE, { status: 201 })
		}),
	)
	renderAt('/field-groups')
	const dialog = await openFields()

	await userEvent.type(within(dialog).getByRole('textbox', { name: 'Label' }), 'Summary')
	await userEvent.click(within(dialog).getByRole('button', { name: 'Add field' }))

	await waitFor(() =>
		expect(sent).toMatchObject({ key: 'summary', label: 'Summary', kind: 'text' }),
	)
})

test('reports a refused field where the operator is looking', async () => {
	server.use(
		http.post('/api/groups/3/fields', () =>
			HttpResponse.json({ error: 'content: field key taken', code: 'field_taken' }, { status: 422 }),
		),
	)
	renderAt('/field-groups')
	const dialog = await openFields()

	await userEvent.type(within(dialog).getByRole('textbox', { name: 'Label' }), 'Subtitle')
	await userEvent.click(within(dialog).getByRole('button', { name: 'Add field' }))

	const notice = await within(dialog).findByRole('alert')
	expect(notice).toHaveTextContent(/field with that name/i)
})

test('declares a relation field pointing at the type and holding what was picked', async () => {
	let sent: unknown
	server.use(
		http.post('/api/groups/3/fields', async ({ request }) => {
			sent = await request.json()
			return HttpResponse.json(SUBTITLE, { status: 201 })
		}),
	)
	renderAt('/field-groups')
	const dialog = await openFields()

	await userEvent.type(within(dialog).getByRole('textbox', { name: 'Label' }), 'Author')
	await userEvent.click(within(dialog).getByLabelText('Kind'))
	await userEvent.click(await screen.findByRole('option', { name: 'Relation' }))
	await userEvent.click(within(dialog).getByLabelText('Points at'))
	await userEvent.click(await screen.findByRole('option', { name: 'Posts' }))
	await userEvent.click(within(dialog).getByLabelText('Holds'))
	await userEvent.click(await screen.findByRole('option', { name: 'One target' }))
	await userEvent.click(within(dialog).getByRole('button', { name: 'Add field' }))

	await waitFor(() =>
		expect(sent).toMatchObject({
			key: 'author',
			kind: 'relation',
			relates_to: 'post',
			many: false,
			required: false,
		}),
	)
})

test('asks nothing about Required on the add form, so a new field lands optional', async () => {
	let sent: unknown
	server.use(
		http.post('/api/groups/3/fields', async ({ request }) => {
			sent = await request.json()
			return HttpResponse.json(SUBTITLE, { status: 201 })
		}),
	)
	renderAt('/field-groups')
	const dialog = await openFields()
	const form = within(dialog).getByRole('button', { name: 'Add field' }).closest('form') as HTMLFormElement

	expect(within(form).queryByLabelText(/Required/)).toBeNull()
	expect(within(form).getAllByRole('combobox')).toEqual([within(form).getByRole('combobox', { name: 'Kind' })])
	await userEvent.type(within(form).getByRole('textbox', { name: 'Label' }), 'Summary')
	await userEvent.click(within(form).getByRole('button', { name: 'Add field' }))

	await waitFor(() => expect(sent).toMatchObject({ key: 'summary', kind: 'text', required: false }))
})

test('asks nothing about Required on the form adding a field inside a container', async () => {
	listing([{ ...DETAILS, fields: [{ ...SUBTITLE, key: 'author', label: 'Author', kind: 'section', fields: [] }] }])
	renderAt('/field-groups')
	const dialog = await openFields()
	const author = await openField(dialog, 'Author')

	const adding = addFormOf(author, 'Author')

	expect(within(adding).getAllByRole('combobox')).toEqual([within(adding).getByRole('combobox', { name: 'Kind' })])
	expect(within(adding).queryByLabelText(/Required/)).toBeNull()
})

test('offers a built-in type as a relation target in the words of WordPress es_ES for a Spanish reader', async () => {
	setLocaleData(await catalogFor('es-ES'), 'gophenberg')
	onTestFinished(() => resetLocaleData({}, 'gophenberg'))
	renderAt('/field-groups')
	const dialog = await openFields('Article details', 'Grupos de campos')

	await userEvent.click(within(dialog).getByLabelText('Tipo'))
	await userEvent.click(await screen.findByRole('option', { name: 'Relación' }))

	expect(within(dialog).getByLabelText('Apunta a')).toHaveTextContent('Entradas')
})

test('declares a plain field while the content types are still out of reach', async () => {
	let sent: unknown
	vi.spyOn(console, 'error').mockImplementation(() => {})
	server.use(http.get('/api/types', () => new HttpResponse(null, { status: 500 })))
	server.use(
		http.post('/api/groups/3/fields', async ({ request }) => {
			sent = await request.json()
			return HttpResponse.json(SUBTITLE, { status: 201 })
		}),
	)
	renderAt('/field-groups')
	const dialog = await openFields()

	await userEvent.type(within(dialog).getByRole('textbox', { name: 'Label' }), 'Summary')
	await userEvent.click(within(dialog).getByLabelText('Kind'))
	await userEvent.click(await screen.findByRole('option', { name: 'Relation' }))
	await userEvent.click(within(dialog).getByRole('button', { name: 'Add field' }))

	expect(within(dialog).queryByLabelText('Points at')).not.toBeInTheDocument()
	expect(within(dialog).getByRole('button', { name: 'Add field' })).toHaveAttribute(
		'aria-disabled',
		'true',
	)
	expect(sent).toBeUndefined()
})

test('declares a radio group as the choice kind presented as radio', async () => {
	let sent: unknown
	server.use(
		http.post('/api/groups/3/fields', async ({ request }) => {
			sent = await request.json()
			return HttpResponse.json({ ...SUBTITLE, key: 'style', label: 'Style', kind: 'choice' })
		}),
	)
	renderAt('/field-groups')
	const dialog = await openFields()

	await userEvent.type(within(dialog).getByRole('textbox', { name: 'Label' }), 'Style')
	await userEvent.click(within(dialog).getByRole('combobox', { name: 'Kind' }))
	await userEvent.click(await screen.findByRole('option', { name: 'Radio group' }))
	await userEvent.click(within(dialog).getByRole('button', { name: 'Add field' }))

	await waitFor(() =>
		expect(sent).toMatchObject({
			key: 'style',
			kind: 'choice',
			settings: { presentation: 'radio' },
		}),
	)
})

test('declares an email field as text checked as an email', async () => {
	let sent: unknown
	server.use(
		http.post('/api/groups/3/fields', async ({ request }) => {
			sent = await request.json()
			return HttpResponse.json({ ...SUBTITLE, key: 'contact', label: 'Contact' })
		}),
	)
	renderAt('/field-groups')
	const dialog = await openFields()

	await userEvent.type(within(dialog).getByRole('textbox', { name: 'Label' }), 'Contact')
	await userEvent.click(within(dialog).getByRole('combobox', { name: 'Kind' }))
	await userEvent.click(await screen.findByRole('option', { name: 'Email' }))
	await userEvent.click(within(dialog).getByRole('button', { name: 'Add field' }))

	await waitFor(() =>
		expect(sent).toMatchObject({ kind: 'text', settings: { variant: 'email' } }),
	)
})

test('declares a gallery as a media field holding many', async () => {
	let sent: unknown
	server.use(
		http.post('/api/groups/3/fields', async ({ request }) => {
			sent = await request.json()
			return HttpResponse.json({ ...SUBTITLE, key: 'gallery', label: 'Gallery', kind: 'media' })
		}),
	)
	renderAt('/field-groups')
	const dialog = await openFields()

	await userEvent.type(within(dialog).getByRole('textbox', { name: 'Label' }), 'Gallery')
	await userEvent.click(within(dialog).getByRole('combobox', { name: 'Kind' }))
	await userEvent.click(await screen.findByRole('option', { name: 'Gallery' }))
	await userEvent.click(within(dialog).getByRole('button', { name: 'Add field' }))

	await waitFor(() => expect(sent).toMatchObject({ kind: 'media', many: true }))
	expect((sent as { settings?: unknown }).settings).toBeUndefined()
})

test('offers flexible content at the top of a group and no bare layout', async () => {
	renderAt('/field-groups')
	const dialog = await openFields()

	await userEvent.click(within(dialog).getByRole('combobox', { name: 'Kind' }))

	expect(await screen.findByRole('option', { name: 'Flexible content' })).toBeInTheDocument()
	expect(screen.queryByRole('option', { name: 'Layout' })).not.toBeInTheDocument()
})

test('asks only the label of a layout inside a flexible content field, its button reading Add layout', async () => {
	listing([{ ...DETAILS, fields: [FEATURES] }, EXTRAS])
	renderAt('/field-groups')
	const main = await openFields()
	const features = await openField(main, 'Features')

	const adding = addFormOf(features, 'Features', 'Add layout')

	expect(within(adding).queryByRole('combobox')).toBeNull()
	expect(within(adding).getAllByRole('textbox')).toEqual([within(adding).getByRole('textbox', { name: 'Label' })])
	expect(within(adding).getByRole('button', { name: 'Add layout to Features' }).textContent).toBe('Add layout')
})

const VANS = {
	...DETAILS,
	id: 5,
	key: 'vans',
	title: 'Vans',
	position: 3,
	fields: [{ ...SUBTITLE, key: 'driver', label: 'Driver', kind: 'relation', relates_to: 'post' }],
}

const CARS = {
	...DETAILS,
	id: 6,
	key: 'cars',
	title: 'Cars',
	position: 4,
	fields: [
		{ ...SUBTITLE, key: 'maker', label: 'Maker', kind: 'relation', relates_to: 'post' },
		{ ...SUBTITLE, key: 'owner', label: 'Owner', kind: 'relation', relates_to: 'post' },
		{ ...SUBTITLE, key: 'note', label: 'Note', kind: 'text' },
	],
}

/**
 * Opens the kind picker on the linked from entry.
 * @param dialog - The dialog the fields stand in.
 */
async function pickLinkedFrom(dialog: HTMLElement) {
	await userEvent.click(within(dialog).getByRole('combobox', { name: 'Kind' }))
	await userEvent.click(await screen.findByRole('option', { name: 'Linked from' }))
}

test('declares a linked from field reading a relation another group holds', async () => {
	let sent: unknown
	listing([DETAILS, EXTRAS, VANS, CARS])
	server.use(
		http.post('/api/groups/3/fields', async ({ request }) => {
			sent = await request.json()
			return HttpResponse.json({ ...SUBTITLE, key: 'linked-from', kind: 'backlinks' }, { status: 201 })
		}),
	)
	renderAt('/field-groups')
	const dialog = await openFields()

	await userEvent.type(within(dialog).getByRole('textbox', { name: 'Label' }), 'Linked from')
	await pickLinkedFrom(dialog)
	await userEvent.click(within(dialog).getByRole('combobox', { name: 'Reads from' }))
	await userEvent.click(await screen.findByRole('option', { name: 'Cars' }))
	await userEvent.click(within(dialog).getByRole('combobox', { name: 'Through' }))
	await userEvent.click(await screen.findByRole('option', { name: 'Owner' }))
	await userEvent.click(within(dialog).getByRole('button', { name: 'Add field' }))

	await waitFor(() =>
		expect(sent).toMatchObject({
			key: 'linked-from',
			kind: 'backlinks',
			settings: { source_group: 'cars', source_field: ['owner'] },
		}),
	)
})

test('declares a linked from field optional', async () => {
	let sent: unknown
	listing([DETAILS, EXTRAS, VANS, CARS])
	server.use(
		http.post('/api/groups/3/fields', async ({ request }) => {
			sent = await request.json()
			return HttpResponse.json({ ...SUBTITLE, key: 'linked-from', kind: 'backlinks' }, { status: 201 })
		}),
	)
	renderAt('/field-groups')
	const dialog = await openFields()

	await userEvent.type(within(dialog).getByRole('textbox', { name: 'Label' }), 'Linked from')
	await pickLinkedFrom(dialog)
	await userEvent.click(within(dialog).getByRole('combobox', { name: 'Reads from' }))
	await userEvent.click(await screen.findByRole('option', { name: 'Cars' }))
	await userEvent.click(within(dialog).getByRole('combobox', { name: 'Through' }))
	await userEvent.click(await screen.findByRole('option', { name: 'Owner' }))
	await userEvent.click(within(dialog).getByRole('button', { name: 'Add field' }))

	await waitFor(() => expect(sent).toMatchObject({ kind: 'backlinks', required: false }))
})

test('offers a linked from field only the groups holding a relation', async () => {
	listing([DETAILS, EXTRAS, VANS, CARS])
	renderAt('/field-groups')
	const dialog = await openFields()

	await pickLinkedFrom(dialog)
	await userEvent.click(within(dialog).getByRole('combobox', { name: 'Reads from' }))

	expect(await screen.findByRole('option', { name: 'Vans' })).toBeInTheDocument()
	expect(screen.queryByRole('option', { name: 'Extras' })).not.toBeInTheDocument()
	expect(screen.queryByRole('option', { name: 'Article details' })).not.toBeInTheDocument()
})

test('offers a linked from field only the relations of the group it reads', async () => {
	listing([DETAILS, EXTRAS, VANS, CARS])
	renderAt('/field-groups')
	const dialog = await openFields()

	await pickLinkedFrom(dialog)
	await userEvent.click(within(dialog).getByRole('combobox', { name: 'Reads from' }))
	await userEvent.click(await screen.findByRole('option', { name: 'Cars' }))
	await userEvent.click(within(dialog).getByRole('combobox', { name: 'Through' }))

	expect(await screen.findByRole('option', { name: 'Maker' })).toBeInTheDocument()
	expect(screen.queryByRole('option', { name: 'Note' })).not.toBeInTheDocument()
	expect(screen.queryByRole('option', { name: 'Driver' })).not.toBeInTheDocument()
})

test('holds back a linked from field when no group holds a relation', async () => {
	renderAt('/field-groups')
	const dialog = await openFields()

	await pickLinkedFrom(dialog)

	expect(within(dialog).getByRole('button', { name: 'Add field' })).toHaveAttribute(
		'aria-disabled',
		'true',
	)
})

test('offers no linked from field inside a container', async () => {
	listing([
		{
			...DETAILS,
			fields: [{ ...SUBTITLE, key: 'author', label: 'Author', kind: 'section', fields: [] }],
		},
		CARS,
	])
	renderAt('/field-groups')
	const dialog = await openFields()
	const author = await openField(dialog, 'Author')

	await userEvent.click(within(addFormOf(author, 'Author')).getByRole('combobox', { name: 'Kind' }))

	expect(await screen.findByRole('option', { name: 'Text' })).toBeInTheDocument()
	expect(screen.queryByRole('option', { name: 'Linked from' })).not.toBeInTheDocument()
})

test("stops a field declared into the group past the site's time limit, keeping what its form holds", async () => {
	server.use(
		http.get('/api/settings', () =>
			HttpResponse.json({ ...siteSettings, dialog_write_timeout_milliseconds: SHORT_LIMIT }),
		),
		http.post('/api/groups/3/fields', () => new Promise<never>(() => {})),
	)
	renderAt('/field-groups')
	const main = await openFields()

	await userEvent.type(within(main).getByRole('textbox', { name: 'Label' }), 'Summary')
	await userEvent.click(within(main).getByRole('button', { name: 'Add field' }))

	expect(await findToast('Could not add the field. The field groups could not be reached.')).toBeInTheDocument()
	expect(within(main).getByRole('textbox', { name: 'Label' })).toHaveValue('Summary')
	expect(within(main).getByRole('textbox', { name: 'Label' })).not.toHaveAttribute('readonly')
})

test('keeps the label and the kind of a new field read only while it is declared', async () => {
	const answer = gate()
	server.use(
		http.post('/api/groups/3/fields', async () => {
			await answer.held
			return HttpResponse.json({ ...SUBTITLE, key: 'summary', label: 'Summary' }, { status: 201 })
		}),
	)
	renderAt('/field-groups')
	const dialog = await openFields()

	await userEvent.type(within(dialog).getByRole('textbox', { name: 'Label' }), 'Summary')
	await userEvent.click(within(dialog).getByRole('button', { name: 'Add field' }))
	await waitBusy(dialog, 'Add field')

	expect(within(dialog).getByRole('textbox', { name: 'Label' })).toHaveAttribute('readonly')
	expect(within(dialog).getByRole('combobox', { name: 'Kind' })).toHaveAttribute('aria-readonly', 'true')
	answer.release()
	await waitFor(() => expect(within(dialog).getByRole('textbox', { name: 'Label' })).not.toHaveAttribute('readonly'))
})
