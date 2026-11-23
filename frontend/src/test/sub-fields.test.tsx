// SPDX-License-Identifier: Apache-2.0

import { http, HttpResponse, server } from '@gophenberg/frontend-sdk/testing'
import { screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { expect, test } from 'vitest'

import { gate } from './gate'
import {
	DETAILS,
	EXTRAS,
	FEATURES,
	fieldRow,
	HERO,
	listing,
	openFields,
	serveFieldGroups,
	SUBTITLE,
} from './groupFieldsFixtures'
import { renderAt } from './render'
import { expectReadOnly, waitBusy } from './typeFixtures'

serveFieldGroups()

test('lists the sub fields a container declares under it', async () => {
	listing([
		{
			...DETAILS,
			fields: [
				{
					...SUBTITLE,
					key: 'author',
					label: 'Author',
					kind: 'section',
					fields: [{ ...SUBTITLE, key: 'name', label: 'Name' }],
				},
			],
		},
		EXTRAS,
	])
	renderAt('/field-groups')

	const dialog = await openFields()

	expect(within(dialog).getByRole('listitem', { name: 'Name' })).toBeInTheDocument()
})

test('declares a sub field inside the container it was opened from', async () => {
	let sent: unknown
	let where = ''
	listing([
		{
			...DETAILS,
			fields: [{ ...SUBTITLE, key: 'author', label: 'Author', kind: 'section', fields: [] }],
		},
		EXTRAS,
	])
	server.use(
		http.post('/api/groups/3/fields/author', async ({ request }) => {
			sent = await request.json()
			where = new URL(request.url).pathname
			return HttpResponse.json({ ...SUBTITLE, key: 'name', label: 'Name' }, { status: 201 })
		}),
	)
	renderAt('/field-groups')
	const dialog = await openFields()

	await userEvent.click(within(dialog).getByRole('button', { name: 'Add field to Author' }))
	const adding = await screen.findByRole('dialog', { name: 'Add field to Author' })
	await userEvent.type(within(adding).getByLabelText('Label'), 'Full name')
	await userEvent.click(within(adding).getByRole('button', { name: 'Add field' }))

	await waitFor(() => expect(sent).toMatchObject({ key: 'full-name', label: 'Full name' }))
	expect(where).toBe('/api/groups/3/fields/author')
})

test('declares a field inside a container standing inside another', async () => {
	let where = ''
	listing([
		{
			...DETAILS,
			fields: [
				{
					...SUBTITLE,
					key: 'author',
					label: 'Author',
					kind: 'section',
					fields: [{ ...SUBTITLE, key: 'address', label: 'Address', kind: 'section', fields: [] }],
				},
			],
		},
		EXTRAS,
	])
	server.use(
		http.post('/api/groups/3/fields/author.address', ({ request }) => {
			where = new URL(request.url).pathname
			return HttpResponse.json({ ...SUBTITLE, key: 'street', label: 'Street' }, { status: 201 })
		}),
	)
	renderAt('/field-groups')
	const dialog = await openFields()

	await userEvent.click(within(dialog).getByRole('button', { name: 'Add field to Address' }))
	const adding = await screen.findByRole('dialog', { name: 'Add field to Address' })
	await userEvent.type(within(adding).getByLabelText('Label'), 'Street')
	await userEvent.click(within(adding).getByRole('button', { name: 'Add field' }))

	await waitFor(() => expect(where).toBe('/api/groups/3/fields/author.address'))
})

test('requires a field standing inside a container', async () => {
	let where = ''
	let sent: unknown
	listing([
		{
			...DETAILS,
			fields: [
				{
					...SUBTITLE,
					key: 'author',
					label: 'Author',
					kind: 'section',
					fields: [{ ...SUBTITLE, key: 'name', label: 'Name' }],
				},
			],
		},
		EXTRAS,
	])
	server.use(
		http.patch('/api/groups/3/fields/author.name', async ({ request }) => {
			where = new URL(request.url).pathname
			sent = await request.json()
			return HttpResponse.json({ ...SUBTITLE, key: 'name', label: 'Name', required: true })
		}),
	)
	renderAt('/field-groups')
	const dialog = await openFields()

	await userEvent.click(within(dialog).getByRole('button', { name: 'Require Name' }))

	await waitFor(() => expect(where).toBe('/api/groups/3/fields/author.name'))
	expect(sent).toMatchObject({ required: true })
})

test('renames a field standing inside a container', async () => {
	let where = ''
	listing([
		{
			...DETAILS,
			fields: [
				{
					...SUBTITLE,
					key: 'author',
					label: 'Author',
					kind: 'section',
					fields: [{ ...SUBTITLE, key: 'name', label: 'Name' }],
				},
			],
		},
		EXTRAS,
	])
	server.use(
		http.patch('/api/groups/3/fields/author.name', ({ request }) => {
			where = new URL(request.url).pathname
			return HttpResponse.json({ ...SUBTITLE, key: 'name', label: 'Full name' })
		}),
	)
	renderAt('/field-groups')
	const dialog = await openFields()

	await userEvent.click(within(dialog).getByRole('button', { name: 'Rename Name' }))
	const renaming = await screen.findByRole('dialog', { name: 'Rename Name' })
	await userEvent.clear(within(renaming).getByLabelText('Name'))
	await userEvent.type(within(renaming).getByLabelText('Name'), 'Full name')
	await userEvent.click(within(renaming).getByRole('button', { name: 'Rename' }))

	await waitFor(() => expect(where).toBe('/api/groups/3/fields/author.name'))
})

test('stands the fields inside a container in a new order', async () => {
	let where = ''
	let sent: unknown
	listing([
		{
			...DETAILS,
			fields: [
				{
					...SUBTITLE,
					key: 'author',
					label: 'Author',
					kind: 'section',
					fields: [
						{ ...SUBTITLE, key: 'name', label: 'Name' },
						{ ...SUBTITLE, key: 'bio', label: 'Bio' },
					],
				},
			],
		},
		EXTRAS,
	])
	server.use(
		http.put('/api/groups/3/inside/author/order', async ({ request }) => {
			where = new URL(request.url).pathname
			sent = await request.json()
			return HttpResponse.json({ items: [] })
		}),
	)
	renderAt('/field-groups')
	const dialog = await openFields()

	await userEvent.click(within(dialog).getByRole('button', { name: 'Move Bio up' }))

	await waitFor(() => expect(where).toBe('/api/groups/3/inside/author/order'))
	expect(sent).toMatchObject({ order: ['bio', 'name'] })
})

test('marks a required field inside a container as required', async () => {
	listing([
		{
			...DETAILS,
			fields: [
				{
					...SUBTITLE,
					key: 'author',
					label: 'Author',
					kind: 'section',
					fields: [{ ...SUBTITLE, key: 'name', label: 'Name', required: true }],
				},
			],
		},
		EXTRAS,
	])
	renderAt('/field-groups')

	const dialog = await openFields()

	const held = within(dialog).getByRole('listitem', { name: 'Name' })
	expect(within(held).getByText('Required')).toBeInTheDocument()
})

test('stands a field inside a container further down', async () => {
	let sent: unknown
	listing([
		{
			...DETAILS,
			fields: [
				{
					...SUBTITLE,
					key: 'author',
					label: 'Author',
					kind: 'section',
					fields: [
						{ ...SUBTITLE, key: 'name', label: 'Name' },
						{ ...SUBTITLE, key: 'bio', label: 'Bio' },
					],
				},
			],
		},
		EXTRAS,
	])
	server.use(
		http.put('/api/groups/3/inside/author/order', async ({ request }) => {
			sent = await request.json()
			return HttpResponse.json({ items: [] })
		}),
	)
	renderAt('/field-groups')
	const dialog = await openFields()

	await userEvent.click(within(dialog).getByRole('button', { name: 'Move Name down' }))

	await waitFor(() => expect(sent).toMatchObject({ order: ['bio', 'name'] }))
})

test('reports a refused order inside a container', async () => {
	listing([
		{
			...DETAILS,
			fields: [
				{
					...SUBTITLE,
					key: 'author',
					label: 'Author',
					kind: 'section',
					fields: [
						{ ...SUBTITLE, key: 'name', label: 'Name' },
						{ ...SUBTITLE, key: 'bio', label: 'Bio' },
					],
				},
			],
		},
		EXTRAS,
	])
	server.use(
		http.put('/api/groups/3/inside/author/order', () =>
			HttpResponse.json(
				{ error: 'content: the order leaves declared fields out', code: 'field_order_incomplete' },
				{ status: 422 },
			),
		),
	)
	renderAt('/field-groups')
	const dialog = await openFields()

	await userEvent.click(within(dialog).getByRole('button', { name: 'Move Bio up' }))

	expect(await within(dialog).findByRole('alert')).toHaveTextContent(/field/i)
})

test('removes a sub field from the container that holds it', async () => {
	let hit = ''
	listing([
		{
			...DETAILS,
			fields: [
				{
					...SUBTITLE,
					key: 'author',
					label: 'Author',
					kind: 'section',
					fields: [{ ...SUBTITLE, key: 'name', label: 'Name' }],
				},
			],
		},
		EXTRAS,
	])
	server.use(
		http.delete('/api/groups/3/inside/author.name', ({ request }) => {
			hit = new URL(request.url).pathname
			return new HttpResponse(null, { status: 204 })
		}),
	)
	renderAt('/field-groups')
	const dialog = await openFields()

	await userEvent.click(within(dialog).getByRole('button', { name: 'Delete Name' }))
	const warning = await screen.findByRole('alertdialog', { name: 'Delete "Name"?' })
	await userEvent.click(within(warning).getByRole('button', { name: 'Delete the field' }))

	await waitFor(() => expect(hit).toBe('/api/groups/3/inside/author.name'))
})

test('declares a layout inside the flexible content field it was opened from', async () => {
	let sent: unknown
	listing([{ ...DETAILS, fields: [FEATURES] }, EXTRAS])
	server.use(
		http.post('/api/groups/3/fields/features', async ({ request }) => {
			sent = await request.json()
			return HttpResponse.json(HERO, { status: 201 })
		}),
	)
	renderAt('/field-groups')
	const dialog = await openFields()

	await userEvent.click(within(dialog).getByRole('button', { name: 'Add layout to Features' }))
	const adding = await screen.findByRole('dialog', { name: 'Add layout to Features' })
	await userEvent.type(within(adding).getByLabelText('Label'), 'Hero')
	await userEvent.click(within(adding).getByRole('button', { name: 'Add layout' }))

	await waitFor(() => expect(sent).toMatchObject({ key: 'hero', label: 'Hero', kind: 'layout' }))
})

test('lists a flexible content field with the layouts and fields beneath it', async () => {
	listing([
		{
			...DETAILS,
			fields: [{ ...FEATURES, fields: [{ ...HERO, fields: [{ ...SUBTITLE, key: 'title', label: 'Title' }] }] }],
		},
		EXTRAS,
	])
	renderAt('/field-groups')

	const dialog = await openFields()

	const flexible = await fieldRow(dialog, 'Features')
	const layout = within(dialog).getByRole('listitem', { name: 'Hero' })
	expect(within(flexible).getByText('Flexible content')).toBeInTheDocument()
	expect(within(layout).getByText('Layout')).toBeInTheDocument()
	expect(within(dialog).getByRole('listitem', { name: 'Title' })).toBeInTheDocument()
})

test('declares a field inside a layout of a flexible content field', async () => {
	let where = ''
	listing([{ ...DETAILS, fields: [{ ...FEATURES, fields: [HERO] }] }, EXTRAS])
	server.use(
		http.post('/api/groups/3/fields/features.hero', ({ request }) => {
			where = new URL(request.url).pathname
			return HttpResponse.json({ ...SUBTITLE, key: 'title', label: 'Title' }, { status: 201 })
		}),
	)
	renderAt('/field-groups')
	const dialog = await openFields()

	await userEvent.click(within(dialog).getByRole('button', { name: 'Add field to Hero' }))
	const adding = await screen.findByRole('dialog', { name: 'Add field to Hero' })
	await userEvent.type(within(adding).getByLabelText('Label'), 'Title')
	await userEvent.click(within(adding).getByRole('button', { name: 'Add field' }))

	await waitFor(() => expect(where).toBe('/api/groups/3/fields/features.hero'))
})

test('says inside its own dialog why a field declared into a container was turned away', async () => {
	listing([
		{
			...DETAILS,
			fields: [{ ...SUBTITLE, key: 'author', label: 'Author', kind: 'section', fields: [] }],
		},
		EXTRAS,
	])
	server.use(
		http.post('/api/groups/3/fields/author', () =>
			HttpResponse.json({ error: 'content: field key taken', code: 'field_taken' }, { status: 422 }),
		),
	)
	renderAt('/field-groups')
	const dialog = await openFields()

	await userEvent.click(within(dialog).getByRole('button', { name: 'Add field to Author' }))
	const adding = await screen.findByRole('dialog', { name: 'Add field to Author' })
	await userEvent.type(within(adding).getByLabelText('Label'), 'Full name')
	await userEvent.click(within(adding).getByRole('button', { name: 'Add field' }))

	const said = 'A field with that name already exists on the items this group reaches. Pick another name.'
	expect(await within(adding).findByText(said)).toBeInTheDocument()
	expect(screen.getAllByText(said)).toHaveLength(1)
	expect(within(adding).getByLabelText('Label')).toHaveValue('Full name')
})

test('keeps the controls of a field declared into a container read only while it is declared', async () => {
	const answer = gate()
	listing([
		{
			...DETAILS,
			fields: [{ ...SUBTITLE, key: 'author', label: 'Author', kind: 'section', fields: [] }],
		},
		EXTRAS,
	])
	server.use(
		http.post('/api/groups/3/fields/author', async () => {
			await answer.held
			return HttpResponse.json({ ...SUBTITLE, key: 'name', label: 'Name' }, { status: 201 })
		}),
	)
	renderAt('/field-groups')
	const dialog = await openFields()

	await userEvent.click(within(dialog).getByRole('button', { name: 'Add field to Author' }))
	const adding = await screen.findByRole('dialog', { name: 'Add field to Author' })
	await userEvent.type(within(adding).getByLabelText('Label'), 'Full name')
	await userEvent.click(within(adding).getByRole('button', { name: 'Add field' }))
	await waitBusy(adding, 'Add field')

	expectReadOnly(adding)
	answer.release()
	await waitFor(() => expect(screen.queryByRole('dialog', { name: 'Add field to Author' })).not.toBeInTheDocument())
})
