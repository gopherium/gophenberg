// SPDX-License-Identifier: Apache-2.0

import { http, HttpResponse, server } from '@gophenberg/frontend-sdk/testing'
import { act, screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { expect, test } from 'vitest'

import { groupsQueryKey } from '../content/groups'
import { gate } from './gate'
import {
	DETAILS,
	EXTRAS,
	fieldsOf,
	listing,
	openField,
	openFields,
	READING_TIME,
	serveFieldGroups,
	SUBTITLE,
} from './groupFieldsFixtures'
import { renderAt } from './render'
import { expectReadOnly, findToast, waitBusy } from './typeFixtures'

const NAME = { ...SUBTITLE, key: 'name', label: 'Name' }

const AUTHOR = { ...SUBTITLE, key: 'author', label: 'Author', kind: 'section', fields: [NAME] }

const NESTED = { ...DETAILS, fields: [SUBTITLE, READING_TIME, AUTHOR] }

const BOX = { ...SUBTITLE, key: 'box', label: 'Box', kind: 'section', fields: [] }

const CRATED = { ...EXTRAS, fields: [BOX] }

const QUOTE = { ...SUBTITLE, key: 'quote', label: 'Quote', kind: 'layout', fields: [] }

const BLOCKS = { ...SUBTITLE, key: 'blocks', label: 'Blocks', kind: 'flexible', fields: [QUOTE] }

const LINKED = { ...SUBTITLE, key: 'linked-from', label: 'Linked from', kind: 'backlinks' }

const VALUES_FOLLOW_MESSAGE = 'The field keeps its values on the content both groups reach.'

const VALUES_STAY_MESSAGE = 'The values stored under it do not follow.'

const STALE_WRITE_MESSAGE =
	'Someone else saved changes to this item while you were editing, so yours were not saved. ' +
	'Copy your text somewhere safe, then reload the item and put your changes back in.'

serveFieldGroups()

/**
 * Opens the move of a field.
 * @param scope - The element the field's Move button stands in.
 * @param label - The label of the field being carried.
 * @returns The move dialog.
 */
async function openMoveDialog(scope: HTMLElement, label: string) {
	const moveButtonName = `Move ${label} elsewhere`
	await userEvent.click(within(scope).getByRole('button', { name: moveButtonName }))
	return screen.findByRole('dialog', { name: moveButtonName })
}

/**
 * Opens the move of a field and the list of places it may land.
 * @param scope - The element the field's Move button stands in.
 * @param label - The label of the field being carried.
 * @returns The move dialog.
 */
async function openDestinationList(scope: HTMLElement, label: string) {
	const moveDialog = await openMoveDialog(scope, label)
	await userEvent.click(within(moveDialog).getByLabelText('Destination'))
	return moveDialog
}

test.each([
	{
		carries: 'over to another group',
		field: SUBTITLE,
		path: 'subtitle',
		groups: [DETAILS, EXTRAS],
		openedContainer: '',
		destination: 'Extras',
		message: VALUES_FOLLOW_MESSAGE,
		body: { to_group: 4 },
	},
	{
		carries: 'inside a container and says its values stay behind',
		field: SUBTITLE,
		path: 'subtitle',
		groups: [NESTED, EXTRAS],
		openedContainer: '',
		destination: 'Article details: Author',
		message: VALUES_STAY_MESSAGE,
		body: { to_group: 3, to_parent: 'author' },
	},
	{
		carries: 'out of a container to the top of its group',
		field: NAME,
		path: 'author.name',
		groups: [NESTED, EXTRAS],
		openedContainer: 'Author',
		destination: 'Article details',
		message: VALUES_STAY_MESSAGE,
		body: { to_group: 3 },
	},
])('carries a field $carries', async (moveCase) => {
	let sentBody: unknown
	server.use(
		http.post(`/api/groups/3/fields/${moveCase.path}/move`, async ({ request }) => {
			sentBody = await request.json()
			return HttpResponse.json(moveCase.field)
		}),
	)
	listing(moveCase.groups)
	renderAt('/field-groups')
	const main = await openFields()
	const scope = moveCase.openedContainer === '' ? main : await openField(main, moveCase.openedContainer)

	const moveDialog = await openDestinationList(scope, moveCase.field.label)
	await userEvent.click(await screen.findByRole('option', { name: moveCase.destination }))
	expect(within(moveDialog).getByText(moveCase.message)).toBeInTheDocument()
	await userEvent.click(within(moveDialog).getByRole('button', { name: 'Move the field' }))

	await waitFor(() => expect(sentBody).toEqual(moveCase.body))
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
	const main = await openFields()
	await userEvent.click(within(main).getByRole('button', { name: 'Delete Author' }))
	const warning = await screen.findByRole('alertdialog', { name: 'Delete "Author"?' })
	await userEvent.click(within(warning).getByRole('button', { name: 'Delete the field' }))
	await waitFor(() => expect(within(fieldsOf(main)).queryByText('Author')).not.toBeInTheDocument())

	const moveDialog = await openMoveDialog(main, 'Subtitle')
	expect(within(moveDialog).getByText(VALUES_FOLLOW_MESSAGE)).toBeInTheDocument()
	await userEvent.click(within(moveDialog).getByRole('button', { name: 'Move the field' }))

	await waitFor(() => expect(sent).toEqual({ to_group: 4 }))
})

test('offers nowhere to carry a field when the group is the only one', async () => {
	listing([DETAILS])
	renderAt('/field-groups')

	const main = await openFields()

	expect(within(main).queryByRole('button', { name: 'Move Subtitle elsewhere' })).not.toBeInTheDocument()
})

test('offers every container beside the other groups as a place to carry a field', async () => {
	listing([NESTED, CRATED])
	renderAt('/field-groups')
	const main = await openFields()

	await openDestinationList(main, 'Subtitle')

	expect(await screen.findByRole('option', { name: 'Extras' })).toBeInTheDocument()
	expect(screen.getByRole('option', { name: 'Extras: Box' })).toBeInTheDocument()
	expect(screen.getByRole('option', { name: 'Article details: Author' })).toBeInTheDocument()
	expect(screen.queryByRole('option', { name: 'Article details' })).not.toBeInTheDocument()
})

test('never offers a container to a Linked from field', async () => {
	listing([{ ...DETAILS, fields: [LINKED, AUTHOR] }, CRATED])
	renderAt('/field-groups')
	const main = await openFields()

	await openDestinationList(main, 'Linked from')

	expect(await screen.findByRole('option', { name: 'Extras' })).toBeInTheDocument()
	expect(screen.queryByRole('option', { name: 'Article details: Author' })).not.toBeInTheDocument()
	expect(screen.queryByRole('option', { name: 'Extras: Box' })).not.toBeInTheDocument()
})

test('offers a layout nothing but another flexible content field', async () => {
	const gallery = { ...SUBTITLE, key: 'gallery', label: 'Gallery', kind: 'flexible', fields: [] }
	listing([{ ...DETAILS, fields: [BLOCKS, AUTHOR] }, { ...EXTRAS, fields: [gallery, BOX] }])
	renderAt('/field-groups')
	const main = await openFields()
	const blocksPart = await openField(main, 'Blocks')

	await openDestinationList(blocksPart, 'Quote')

	expect(await screen.findByRole('option', { name: 'Extras: Gallery' })).toBeInTheDocument()
	expect(screen.queryByRole('option', { name: 'Article details: Author' })).not.toBeInTheDocument()
	expect(screen.queryByRole('option', { name: 'Extras: Box' })).not.toBeInTheDocument()
	expect(screen.queryByRole('option', { name: 'Extras' })).not.toBeInTheDocument()
})

test('never offers a flexible content field to anything but a layout', async () => {
	listing([{ ...DETAILS, fields: [SUBTITLE, BLOCKS] }, EXTRAS])
	renderAt('/field-groups')
	const main = await openFields()

	await openDestinationList(main, 'Subtitle')

	expect(await screen.findByRole('option', { name: 'Extras' })).toBeInTheDocument()
	expect(screen.queryByRole('option', { name: 'Article details: Blocks' })).not.toBeInTheDocument()
})

test('offers a choice field every place a field of its own may stand', async () => {
	listing([{ ...DETAILS, fields: [{ ...SUBTITLE, key: 'style', label: 'Style', kind: 'choice' }] }, CRATED])
	renderAt('/field-groups')
	const main = await openFields()

	await openDestinationList(main, 'Style')

	expect(await screen.findByRole('option', { name: 'Extras' })).toBeInTheDocument()
	expect(screen.getByRole('option', { name: 'Extras: Box' })).toBeInTheDocument()
})

test('never offers a group or a container a plugin keeps', async () => {
	const kept = { ...BOX, key: 'kept', label: 'Kept', origin: 'forms' }
	listing([{ ...NESTED, fields: [SUBTITLE, AUTHOR, kept] }, { ...CRATED, origin: 'forms' }])
	renderAt('/field-groups')
	const main = await openFields()

	await openDestinationList(main, 'Subtitle')

	expect(await screen.findByRole('option', { name: 'Article details: Author' })).toBeInTheDocument()
	expect(screen.queryByRole('option', { name: 'Article details: Kept' })).not.toBeInTheDocument()
	expect(screen.queryByRole('option', { name: 'Extras' })).not.toBeInTheDocument()
	expect(screen.queryByRole('option', { name: 'Extras: Box' })).not.toBeInTheDocument()
})

test('never offers a container inside the field being carried', async () => {
	const nestedSection = { ...BOX, key: 'inner', label: 'Inner' }
	listing([{ ...DETAILS, fields: [SUBTITLE, READING_TIME, { ...AUTHOR, fields: [NAME, nestedSection] }, BOX] }, EXTRAS])
	renderAt('/field-groups')
	const main = await openFields()

	await openDestinationList(main, 'Author')

	expect(await screen.findByRole('option', { name: 'Extras' })).toBeInTheDocument()
	expect(screen.getByRole('option', { name: 'Article details: Box' })).toBeInTheDocument()
	expect(screen.queryByRole('option', { name: 'Article details: Author' })).not.toBeInTheDocument()
	expect(screen.queryByRole('option', { name: 'Article details: Inner' })).not.toBeInTheDocument()
})

test('keeps the move open and says inside why it was turned away', async () => {
	server.use(
		http.post('/api/groups/3/fields/subtitle/move', () =>
			HttpResponse.json({ error: 'content: field key taken', code: 'field_taken' }, { status: 422 }),
		),
	)
	renderAt('/field-groups')
	const main = await openFields()

	const moveDialog = await openMoveDialog(main, 'Subtitle')
	await userEvent.click(within(moveDialog).getByRole('button', { name: 'Move the field' }))

	const takenNameMessage = 'A field with that name already exists on the items this group reaches. Pick another name.'
	expect(await within(moveDialog).findByText(takenNameMessage)).toBeInTheDocument()
	expect(screen.getAllByText(takenNameMessage)).toHaveLength(1)
})

test('closes the move, says why in a toast and reads the groups again once someone changed them first', async () => {
	let groupReadCount = 0
	server.use(
		http.get('/api/groups', () => {
			groupReadCount += 1
			return HttpResponse.json({ items: [DETAILS, EXTRAS] })
		}),
		http.post('/api/groups/3/fields/subtitle/move', () =>
			HttpResponse.json({ error: 'content: conflicting update', code: 'content_stale_update' }, { status: 409 }),
		),
	)
	renderAt('/field-groups')
	const main = await openFields()
	const readCountBefore = groupReadCount

	const moveDialog = await openMoveDialog(main, 'Subtitle')
	await userEvent.click(within(moveDialog).getByRole('button', { name: 'Move the field' }))

	await waitFor(() => expect(screen.queryByRole('dialog', { name: 'Move Subtitle elsewhere' })).not.toBeInTheDocument())
	expect(await findToast(`Could not save the field. ${STALE_WRITE_MESSAGE}`)).toBeInTheDocument()
	await waitFor(() => expect(groupReadCount).toBe(readCountBefore + 1))
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
	const main = await openFields()

	const moveDialog = await openMoveDialog(main, 'Subtitle')
	await userEvent.click(within(moveDialog).getByRole('button', { name: 'Cancel' }))

	await waitFor(() =>
		expect(screen.queryByRole('dialog', { name: 'Move Subtitle elsewhere' })).not.toBeInTheDocument(),
	)
	expect(hit).toBe(false)
})

test('keeps the destination of a field read only while its move runs', async () => {
	const answer = gate()
	server.use(
		http.post('/api/groups/3/fields/subtitle/move', async () => {
			await answer.held
			return HttpResponse.json(SUBTITLE)
		}),
	)
	renderAt('/field-groups')
	const main = await openFields()

	const moveDialog = await openMoveDialog(main, 'Subtitle')
	await userEvent.click(within(moveDialog).getByRole('button', { name: 'Move the field' }))
	await waitBusy(moveDialog, 'Move the field')

	expectReadOnly(moveDialog)
	answer.release()
	await waitFor(() => expect(screen.queryByRole('dialog', { name: 'Move Subtitle elsewhere' })).not.toBeInTheDocument())
})

test('holds Move back while a draft is held in or under the field, and lets it go once discarded', async () => {
	listing([NESTED, EXTRAS])
	renderAt('/field-groups')
	const main = await openFields()
	const subtitle = await openField(main, 'Subtitle')

	await userEvent.type(within(subtitle).getByRole('textbox', { name: 'Label' }), ' line')

	const move = within(main).getByRole('button', { name: 'Move Subtitle elsewhere' })
	expect(move).toHaveAttribute('aria-disabled', 'true')
	const other = within(main).getByRole('button', { name: 'Move Reading time elsewhere' })
	expect(other).not.toHaveAttribute('aria-disabled', 'true')
	await userEvent.click(move)
	expect(screen.queryByRole('dialog', { name: 'Move Subtitle elsewhere' })).toBeNull()
	await userEvent.click(within(subtitle).getByRole('button', { name: 'Discard changes' }))
	expect(move).not.toHaveAttribute('aria-disabled', 'true')

	await openField(main, 'Author')
	const name = await openField(main, 'Name')
	await userEvent.type(within(name).getByRole('textbox', { name: 'Label' }), ' line')
	expect(within(main).getByRole('button', { name: 'Move Author elsewhere' })).toHaveAttribute('aria-disabled', 'true')
	expect(within(main).getByRole('button', { name: 'Move Name elsewhere' })).toHaveAttribute('aria-disabled', 'true')
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
	const main = await openFields()
	const moveDialog = await openDestinationList(main, 'Subtitle')
	await userEvent.click(await screen.findByRole('option', { name: 'Later' }))

	listing([DETAILS, EXTRAS])
	await act(async () => {
		await client.invalidateQueries({ queryKey: groupsQueryKey })
	})
	await waitFor(() => expect(within(moveDialog).getByLabelText('Destination')).toHaveTextContent('Extras'))
	await userEvent.click(within(moveDialog).getByRole('button', { name: 'Move the field' }))

	await waitFor(() => expect(sent).toEqual({ to_group: 4 }))
})
