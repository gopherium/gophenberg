// SPDX-License-Identifier: Apache-2.0

import { http, HttpResponse, server } from '@gophenberg/frontend-sdk/testing'
import { act, screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { expect, test } from 'vitest'

import { groupsQueryKey } from '../content/groups'
import { DETAILS, EXTRAS, listing, openFields, READING_TIME, serveFieldGroups, SUBTITLE } from './groupFieldsFixtures'
import { renderAt } from './render'

const NAME = { ...SUBTITLE, key: 'name', label: 'Name' }

const AUTHOR = { ...SUBTITLE, key: 'author', label: 'Author', kind: 'section', fields: [NAME] }

const NESTED = { ...DETAILS, fields: [SUBTITLE, READING_TIME, AUTHOR] }

const BOX = { ...SUBTITLE, key: 'box', label: 'Box', kind: 'section', fields: [] }

const CRATED = { ...EXTRAS, fields: [BOX] }

const QUOTE = { ...SUBTITLE, key: 'quote', label: 'Quote', kind: 'layout', fields: [] }

const BLOCKS = { ...SUBTITLE, key: 'blocks', label: 'Blocks', kind: 'flexible', fields: [QUOTE] }

const LINKED = { ...SUBTITLE, key: 'linked-from', label: 'Linked from', kind: 'backlinks' }

serveFieldGroups()

test('carries a field over to another group', async () => {
	let sent: unknown
	server.use(
		http.post('/api/groups/3/fields/subtitle/move', async ({ request }) => {
			sent = await request.json()
			return HttpResponse.json(SUBTITLE)
		}),
	)
	renderAt('/field-groups')
	const dialog = await openFields()

	await userEvent.click(within(dialog).getByRole('button', { name: 'Move Subtitle elsewhere' }))
	const moving = await screen.findByRole('dialog', { name: 'Move Subtitle elsewhere' })
	await userEvent.click(within(moving).getByLabelText('Destination'))
	await userEvent.click(await screen.findByRole('option', { name: 'Extras' }))
	expect(
		within(moving).getByText('The field keeps its values on the content both groups reach.'),
	).toBeInTheDocument()
	await userEvent.click(within(moving).getByRole('button', { name: 'Move the field' }))

	await waitFor(() => expect(sent).toEqual({ to_group: 4 }))
})

test('carries a field to the first place still on offer once the container it would land in is gone', async () => {
	let sent: unknown
	server.use(
		http.delete('/api/groups/3/fields/author', () => {
			listing([DETAILS, EXTRAS])
			return new HttpResponse(null, { status: 204 })
		}),
		http.post('/api/groups/3/fields/subtitle/move', async ({ request }) => {
			sent = await request.json()
			return HttpResponse.json(SUBTITLE)
		}),
	)
	listing([NESTED, EXTRAS])
	renderAt('/field-groups')
	const dialog = await openFields()
	await userEvent.click(within(dialog).getByRole('button', { name: 'Delete Author' }))
	const warning = await screen.findByRole('alertdialog', { name: 'Delete "Author"?' })
	await userEvent.click(within(warning).getByRole('button', { name: 'Delete the field' }))
	await waitFor(() => expect(within(dialog).queryByRole('listitem', { name: 'Author' })).not.toBeInTheDocument())

	await userEvent.click(within(dialog).getByRole('button', { name: 'Move Subtitle elsewhere' }))
	const moving = await screen.findByRole('dialog', { name: 'Move Subtitle elsewhere' })
	expect(
		within(moving).getByText('The field keeps its values on the content both groups reach.'),
	).toBeInTheDocument()
	await userEvent.click(within(moving).getByRole('button', { name: 'Move the field' }))

	await waitFor(() => expect(sent).toEqual({ to_group: 4 }))
})

test('offers nowhere to carry a field when the group is the only one', async () => {
	listing([DETAILS])
	renderAt('/field-groups')

	const dialog = await openFields()

	expect(within(dialog).queryByRole('button', { name: 'Move Subtitle elsewhere' })).not.toBeInTheDocument()
})

test('offers every container beside the other groups as a place to carry a field', async () => {
	listing([NESTED, CRATED])
	renderAt('/field-groups')
	const dialog = await openFields()

	await userEvent.click(within(dialog).getByRole('button', { name: 'Move Subtitle elsewhere' }))
	const moving = await screen.findByRole('dialog', { name: 'Move Subtitle elsewhere' })
	await userEvent.click(within(moving).getByLabelText('Destination'))

	expect(await screen.findByRole('option', { name: 'Extras' })).toBeInTheDocument()
	expect(screen.getByRole('option', { name: 'Extras: Box' })).toBeInTheDocument()
	expect(screen.getByRole('option', { name: 'Article details: Author' })).toBeInTheDocument()
	expect(screen.queryByRole('option', { name: 'Article details' })).not.toBeInTheDocument()
})

test('carries a field inside a container and says its values stay behind', async () => {
	let sent: unknown
	server.use(
		http.post('/api/groups/3/fields/subtitle/move', async ({ request }) => {
			sent = await request.json()
			return HttpResponse.json(SUBTITLE)
		}),
	)
	listing([NESTED, EXTRAS])
	renderAt('/field-groups')
	const dialog = await openFields()

	await userEvent.click(within(dialog).getByRole('button', { name: 'Move Subtitle elsewhere' }))
	const moving = await screen.findByRole('dialog', { name: 'Move Subtitle elsewhere' })
	await userEvent.click(within(moving).getByLabelText('Destination'))
	await userEvent.click(await screen.findByRole('option', { name: 'Article details: Author' }))
	expect(within(moving).getByText('The values stored under it do not follow.')).toBeInTheDocument()
	await userEvent.click(within(moving).getByRole('button', { name: 'Move the field' }))

	await waitFor(() => expect(sent).toEqual({ to_group: 3, to_parent: 'author' }))
})

test('carries a field out of a container to the top of its group', async () => {
	let sent: unknown
	server.use(
		http.post('/api/groups/3/fields/author.name/move', async ({ request }) => {
			sent = await request.json()
			return HttpResponse.json(NAME)
		}),
	)
	listing([NESTED, EXTRAS])
	renderAt('/field-groups')
	const dialog = await openFields()

	await userEvent.click(within(dialog).getByRole('button', { name: 'Move Name elsewhere' }))
	const moving = await screen.findByRole('dialog', { name: 'Move Name elsewhere' })
	await userEvent.click(within(moving).getByLabelText('Destination'))
	await userEvent.click(await screen.findByRole('option', { name: 'Article details' }))
	expect(within(moving).getByText('The values stored under it do not follow.')).toBeInTheDocument()
	await userEvent.click(within(moving).getByRole('button', { name: 'Move the field' }))

	await waitFor(() => expect(sent).toEqual({ to_group: 3 }))
})

test('never offers a container to a Linked from field', async () => {
	listing([{ ...DETAILS, fields: [LINKED, AUTHOR] }, CRATED])
	renderAt('/field-groups')
	const dialog = await openFields()

	await userEvent.click(within(dialog).getByRole('button', { name: 'Move Linked from elsewhere' }))
	const moving = await screen.findByRole('dialog', { name: 'Move Linked from elsewhere' })
	await userEvent.click(within(moving).getByLabelText('Destination'))

	expect(await screen.findByRole('option', { name: 'Extras' })).toBeInTheDocument()
	expect(screen.queryByRole('option', { name: 'Article details: Author' })).not.toBeInTheDocument()
	expect(screen.queryByRole('option', { name: 'Extras: Box' })).not.toBeInTheDocument()
})

test('offers a layout nothing but another flexible content field', async () => {
	const gallery = { ...SUBTITLE, key: 'gallery', label: 'Gallery', kind: 'flexible', fields: [] }
	listing([{ ...DETAILS, fields: [BLOCKS, AUTHOR] }, { ...EXTRAS, fields: [gallery, BOX] }])
	renderAt('/field-groups')
	const dialog = await openFields()

	await userEvent.click(within(dialog).getByRole('button', { name: 'Move Quote elsewhere' }))
	const moving = await screen.findByRole('dialog', { name: 'Move Quote elsewhere' })
	await userEvent.click(within(moving).getByLabelText('Destination'))

	expect(await screen.findByRole('option', { name: 'Extras: Gallery' })).toBeInTheDocument()
	expect(screen.queryByRole('option', { name: 'Article details: Author' })).not.toBeInTheDocument()
	expect(screen.queryByRole('option', { name: 'Extras: Box' })).not.toBeInTheDocument()
	expect(screen.queryByRole('option', { name: 'Extras' })).not.toBeInTheDocument()
})

test('never offers a flexible content field to anything but a layout', async () => {
	listing([{ ...DETAILS, fields: [SUBTITLE, BLOCKS] }, EXTRAS])
	renderAt('/field-groups')
	const dialog = await openFields()

	await userEvent.click(within(dialog).getByRole('button', { name: 'Move Subtitle elsewhere' }))
	const moving = await screen.findByRole('dialog', { name: 'Move Subtitle elsewhere' })
	await userEvent.click(within(moving).getByLabelText('Destination'))

	expect(await screen.findByRole('option', { name: 'Extras' })).toBeInTheDocument()
	expect(screen.queryByRole('option', { name: 'Article details: Blocks' })).not.toBeInTheDocument()
})

test('offers a choice field every place a field of its own may stand', async () => {
	listing([{ ...DETAILS, fields: [{ ...SUBTITLE, key: 'style', label: 'Style', kind: 'choice' }] }, CRATED])
	renderAt('/field-groups')
	const dialog = await openFields()

	await userEvent.click(within(dialog).getByRole('button', { name: 'Move Style elsewhere' }))
	const moving = await screen.findByRole('dialog', { name: 'Move Style elsewhere' })
	await userEvent.click(within(moving).getByLabelText('Destination'))

	expect(await screen.findByRole('option', { name: 'Extras' })).toBeInTheDocument()
	expect(screen.getByRole('option', { name: 'Extras: Box' })).toBeInTheDocument()
})

test('never offers a group or a container a plugin keeps', async () => {
	const kept = { ...BOX, key: 'kept', label: 'Kept', origin: 'forms' }
	listing([{ ...NESTED, fields: [SUBTITLE, AUTHOR, kept] }, { ...CRATED, origin: 'forms' }])
	renderAt('/field-groups')
	const dialog = await openFields()

	await userEvent.click(within(dialog).getByRole('button', { name: 'Move Subtitle elsewhere' }))
	const moving = await screen.findByRole('dialog', { name: 'Move Subtitle elsewhere' })
	await userEvent.click(within(moving).getByLabelText('Destination'))

	expect(await screen.findByRole('option', { name: 'Article details: Author' })).toBeInTheDocument()
	expect(screen.queryByRole('option', { name: 'Article details: Kept' })).not.toBeInTheDocument()
	expect(screen.queryByRole('option', { name: 'Extras' })).not.toBeInTheDocument()
	expect(screen.queryByRole('option', { name: 'Extras: Box' })).not.toBeInTheDocument()
})

test('never offers a container inside the field being carried', async () => {
	listing([NESTED, EXTRAS])
	renderAt('/field-groups')
	const dialog = await openFields()

	await userEvent.click(within(dialog).getByRole('button', { name: 'Move Author elsewhere' }))
	const moving = await screen.findByRole('dialog', { name: 'Move Author elsewhere' })
	await userEvent.click(within(moving).getByLabelText('Destination'))

	expect(await screen.findByRole('option', { name: 'Extras' })).toBeInTheDocument()
	expect(screen.queryByRole('option', { name: 'Article details: Author' })).not.toBeInTheDocument()
})

test('keeps the move open and says inside why it was turned away', async () => {
	server.use(
		http.post('/api/groups/3/fields/subtitle/move', () =>
			HttpResponse.json({ error: 'content: field key taken', code: 'field_taken' }, { status: 422 }),
		),
	)
	renderAt('/field-groups')
	const dialog = await openFields()

	await userEvent.click(within(dialog).getByRole('button', { name: 'Move Subtitle elsewhere' }))
	const moving = await screen.findByRole('dialog', { name: 'Move Subtitle elsewhere' })
	await userEvent.click(within(moving).getByRole('button', { name: 'Move the field' }))

	const said = 'A field with that name already exists on the items this group reaches. Pick another name.'
	expect(await within(moving).findByText(said)).toBeInTheDocument()
	expect(screen.getAllByText(said)).toHaveLength(1)
})

test('keeps a field where it is when the move is dismissed', async () => {
	let hit = false
	server.use(
		http.post('/api/groups/3/fields/subtitle/move', () => {
			hit = true
			return HttpResponse.json(SUBTITLE)
		}),
	)
	renderAt('/field-groups')
	const dialog = await openFields()

	await userEvent.click(within(dialog).getByRole('button', { name: 'Move Subtitle elsewhere' }))
	const moving = await screen.findByRole('dialog', { name: 'Move Subtitle elsewhere' })
	await userEvent.click(within(moving).getByRole('button', { name: 'Cancel' }))

	await waitFor(() =>
		expect(screen.queryByRole('dialog', { name: 'Move Subtitle elsewhere' })).not.toBeInTheDocument(),
	)
	expect(hit).toBe(false)
})

test('carries a field to the first place still on offer once the group picked left while it asked', async () => {
	let sent: unknown
	const later = { ...EXTRAS, id: 5, key: 'later', title: 'Later', position: 3 }
	server.use(
		http.post('/api/groups/3/fields/subtitle/move', async ({ request }) => {
			sent = await request.json()
			return HttpResponse.json(SUBTITLE)
		}),
	)
	listing([DETAILS, EXTRAS, later])
	const client = renderAt('/field-groups')
	const dialog = await openFields()
	await userEvent.click(within(dialog).getByRole('button', { name: 'Move Subtitle elsewhere' }))
	const moving = await screen.findByRole('dialog', { name: 'Move Subtitle elsewhere' })
	await userEvent.click(within(moving).getByLabelText('Destination'))
	await userEvent.click(await screen.findByRole('option', { name: 'Later' }))

	listing([DETAILS, EXTRAS])
	await act(async () => {
		await client.invalidateQueries({ queryKey: groupsQueryKey })
	})
	await waitFor(() => expect(within(moving).getByLabelText('Destination')).toHaveTextContent('Extras'))
	await userEvent.click(within(moving).getByRole('button', { name: 'Move the field' }))

	await waitFor(() => expect(sent).toEqual({ to_group: 4 }))
})
