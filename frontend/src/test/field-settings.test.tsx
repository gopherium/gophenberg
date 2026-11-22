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
	FEATURES,
	HERO,
	listing,
	openFields,
	READING_TIME,
	serveFieldGroups,
	SUBTITLE,
} from './groupFieldsFixtures'
import { renderAt } from './render'
import { expectReadOnly, waitBusy } from './typeFixtures'

serveFieldGroups()

test('renames a field without moving what is stored under it', async () => {
	let sent: unknown
	server.use(
		http.patch('/api/groups/3/fields/subtitle', async ({ request }) => {
			sent = await request.json()
			return HttpResponse.json({ ...SUBTITLE, label: 'Standfirst' })
		}),
	)
	renderAt('/field-groups')
	const dialog = await openFields()

	await userEvent.click(within(dialog).getByRole('button', { name: 'Rename Subtitle' }))
	const renaming = await screen.findByRole('dialog', { name: 'Rename Subtitle' })
	const box = within(renaming).getByLabelText('Name')
	await userEvent.clear(box)
	await userEvent.type(box, 'Standfirst')
	await userEvent.click(within(renaming).getByRole('button', { name: 'Rename' }))

	await waitFor(() =>
		expect(sent).toEqual({ label: 'Standfirst', updated_at: '2026-08-01T10:00:00Z' }),
	)
})

test('requires a field that was optional', async () => {
	let sent: unknown
	server.use(
		http.patch('/api/groups/3/fields/subtitle', async ({ request }) => {
			sent = await request.json()
			return HttpResponse.json({ ...SUBTITLE, required: true })
		}),
	)
	renderAt('/field-groups')
	const dialog = await openFields()

	await userEvent.click(within(dialog).getByRole('button', { name: 'Require Subtitle' }))

	await waitFor(() =>
		expect(sent).toEqual({ required: true, updated_at: '2026-08-01T10:00:00Z' }),
	)
})

test('makes a required field optional again', async () => {
	let sent: unknown
	server.use(
		http.patch('/api/groups/3/fields/reading-time', async ({ request }) => {
			sent = await request.json()
			return HttpResponse.json({ ...READING_TIME, required: false })
		}),
	)
	renderAt('/field-groups')
	const dialog = await openFields()

	await userEvent.click(within(dialog).getByRole('button', { name: 'Make Reading time optional' }))

	await waitFor(() =>
		expect(sent).toEqual({ required: false, updated_at: '2026-08-01T10:00:00Z' }),
	)
})

test('renames on the timestamp the dialog opened with, not one that landed while it was open', async () => {
	let sent: unknown
	server.use(
		http.patch('/api/groups/3/fields/subtitle', async ({ request }) => {
			sent = await request.json()
			return HttpResponse.json({ ...SUBTITLE, label: 'Standfirst' })
		}),
	)
	const client = renderAt('/field-groups')
	const dialog = await openFields()
	await userEvent.click(within(dialog).getByRole('button', { name: 'Rename Subtitle' }))
	const renaming = await screen.findByRole('dialog', { name: 'Rename Subtitle' })
	const box = within(renaming).getByLabelText('Name')
	await userEvent.clear(box)
	await userEvent.type(box, 'Standfirst')

	listing([
		{ ...DETAILS, fields: [{ ...SUBTITLE, updated_at: '2026-08-02T09:00:00Z' }, READING_TIME] },
		EXTRAS,
	])
	await act(async () => {
		await client.invalidateQueries({ queryKey: groupsQueryKey })
	})
	await userEvent.click(within(renaming).getByRole('button', { name: 'Rename' }))

	await waitFor(() =>
		expect(sent).toEqual({ label: 'Standfirst', updated_at: '2026-08-01T10:00:00Z' }),
	)
})

test('settles on the timestamp the dialog opened with, not one that landed while it was open', async () => {
	let sent: unknown
	server.use(
		http.patch('/api/groups/3/fields/subtitle', async ({ request }) => {
			sent = await request.json()
			return HttpResponse.json(SUBTITLE)
		}),
	)
	const client = renderAt('/field-groups')
	const dialog = await openFields()
	await userEvent.click(within(dialog).getByRole('button', { name: 'Settings of Subtitle' }))
	const settings = await screen.findByRole('dialog', { name: 'Settings of Subtitle' })
	await userEvent.type(within(settings).getByLabelText('Instructions'), 'Say who wrote it.')

	listing([
		{ ...DETAILS, fields: [{ ...SUBTITLE, updated_at: '2026-08-02T09:00:00Z' }, READING_TIME] },
		EXTRAS,
	])
	await act(async () => {
		await client.invalidateQueries({ queryKey: groupsQueryKey })
	})
	await userEvent.click(within(settings).getByRole('button', { name: 'Save settings' }))

	await waitFor(() =>
		expect(sent).toEqual({
			settings: { instructions: 'Say who wrote it.' },
			updated_at: '2026-08-01T10:00:00Z',
		}),
	)
})

test('renames the same field twice in one sitting', async () => {
	const sent: unknown[] = []
	server.use(
		http.patch('/api/groups/3/fields/subtitle', async ({ request }) => {
			sent.push(await request.json())
			listing([
				{ ...DETAILS, fields: [{ ...SUBTITLE, updated_at: '2026-08-02T09:00:00Z' }, READING_TIME] },
				EXTRAS,
			])
			return HttpResponse.json({ ...SUBTITLE, updated_at: '2026-08-02T09:00:00Z' })
		}),
	)
	renderAt('/field-groups')
	const dialog = await openFields()

	await userEvent.click(within(dialog).getByRole('button', { name: 'Rename Subtitle' }))
	await userEvent.click(
		within(await screen.findByRole('dialog', { name: 'Rename Subtitle' })).getByRole('button', {
			name: 'Rename',
		}),
	)
	await waitFor(() => expect(sent).toHaveLength(1))
	await userEvent.click(within(dialog).getByRole('button', { name: 'Rename Subtitle' }))
	await userEvent.click(
		within(await screen.findByRole('dialog', { name: 'Rename Subtitle' })).getByRole('button', {
			name: 'Rename',
		}),
	)

	await waitFor(() => expect(sent).toHaveLength(2))
	expect(sent[1]).toMatchObject({ updated_at: '2026-08-02T09:00:00Z' })
})

test('reads the field afresh after a settings save lost to a concurrent edit', async () => {
	const sent: unknown[] = []
	server.use(
		http.patch('/api/groups/3/fields/subtitle', async ({ request }) => {
			sent.push(await request.json())
			if (sent.length > 1) {
				return HttpResponse.json(SUBTITLE)
			}
			listing([
				{ ...DETAILS, fields: [{ ...SUBTITLE, updated_at: '2026-08-02T09:00:00Z' }, READING_TIME] },
				EXTRAS,
			])
			return HttpResponse.json(
				{ error: 'content: conflicting update', code: 'content_stale_update' },
				{ status: 409 },
			)
		}),
	)
	renderAt('/field-groups')
	const dialog = await openFields()
	await userEvent.click(within(dialog).getByRole('button', { name: 'Settings of Subtitle' }))
	await userEvent.type(
		within(await screen.findByRole('dialog', { name: 'Settings of Subtitle' })).getByLabelText(
			'Instructions',
		),
		'Say who wrote it.',
	)
	await userEvent.click(
		within(await screen.findByRole('dialog', { name: 'Settings of Subtitle' })).getByRole('button', {
			name: 'Save settings',
		}),
	)
	await within(dialog).findByRole('alert')

	await userEvent.click(within(dialog).getByRole('button', { name: 'Settings of Subtitle' }))
	await userEvent.click(
		within(await screen.findByRole('dialog', { name: 'Settings of Subtitle' })).getByRole('button', {
			name: 'Save settings',
		}),
	)

	await waitFor(() => expect(sent).toHaveLength(2))
	expect(sent[1]).toMatchObject({ updated_at: '2026-08-02T09:00:00Z' })
})

test('reads the field afresh even when the listing is slow to answer', async () => {
	const sent: unknown[] = []
	let stamp = '2026-08-01T10:00:00Z'
	server.use(
		http.get('/api/groups', async () => {
			await new Promise((settle) => setTimeout(settle, 50))
			return HttpResponse.json({
				items: [
					{ ...DETAILS, fields: [{ ...SUBTITLE, updated_at: stamp }, READING_TIME] },
					EXTRAS,
				],
			})
		}),
		http.patch('/api/groups/3/fields/subtitle', async ({ request }) => {
			sent.push(await request.json())
			if (sent.length > 1) {
				return HttpResponse.json(SUBTITLE)
			}
			stamp = '2026-08-02T09:00:00Z'
			return HttpResponse.json(
				{ error: 'content: conflicting update', code: 'content_stale_update' },
				{ status: 409 },
			)
		}),
	)
	renderAt('/field-groups')
	const dialog = await openFields()
	await userEvent.click(within(dialog).getByRole('button', { name: 'Settings of Subtitle' }))
	await userEvent.click(
		within(await screen.findByRole('dialog', { name: 'Settings of Subtitle' })).getByRole('button', {
			name: 'Save settings',
		}),
	)
	await within(dialog).findByRole('alert')
	await waitFor(() =>
		expect(screen.queryByRole('dialog', { name: 'Settings of Subtitle' })).toBeNull(),
	)

	await userEvent.click(within(dialog).getByRole('button', { name: 'Settings of Subtitle' }))
	await userEvent.click(
		within(await screen.findByRole('dialog', { name: 'Settings of Subtitle' })).getByRole('button', {
			name: 'Save settings',
		}),
	)

	await waitFor(() => expect(sent).toHaveLength(2))
	expect(sent[1]).toMatchObject({ updated_at: '2026-08-02T09:00:00Z' })
})

test('keeps the rename open on the name typed and says inside why it was turned away', async () => {
	server.use(
		http.patch('/api/groups/3/fields/subtitle', () =>
			HttpResponse.json(
				{ error: 'content: field label required', code: 'field_label_required' },
				{ status: 422 },
			),
		),
	)
	renderAt('/field-groups')
	const dialog = await openFields()
	await userEvent.click(within(dialog).getByRole('button', { name: 'Rename Subtitle' }))
	const renaming = await screen.findByRole('dialog', { name: 'Rename Subtitle' })
	const box = within(renaming).getByLabelText('Name')
	await userEvent.clear(box)
	await userEvent.type(box, 'Standfirst')
	await userEvent.click(within(renaming).getByRole('button', { name: 'Rename' }))

	const said = 'A field needs a name. Type one and try again.'
	expect(await within(renaming).findByText(said)).toBeInTheDocument()
	expect(screen.getAllByText(said)).toHaveLength(1)
	expect(screen.getByRole('dialog', { name: 'Rename Subtitle' })).toBe(renaming)
	expect(within(renaming).getByLabelText('Name')).toHaveValue('Standfirst')
})

test('reports a rename that lost to a concurrent edit', async () => {
	server.use(
		http.patch('/api/groups/3/fields/subtitle', () =>
			HttpResponse.json(
				{ error: 'content: conflicting update', code: 'content_stale_update' },
				{ status: 409 },
			),
		),
	)
	renderAt('/field-groups')
	const dialog = await openFields()

	await userEvent.click(within(dialog).getByRole('button', { name: 'Rename Subtitle' }))
	const renaming = await screen.findByRole('dialog', { name: 'Rename Subtitle' })
	await userEvent.click(within(renaming).getByRole('button', { name: 'Rename' }))

	expect(await within(dialog).findByRole('alert')).toHaveTextContent(/someone else saved/i)
})

test('keeps the name when a rename is dismissed', async () => {
	let hit = false
	server.use(
		http.patch('/api/groups/3/fields/subtitle', () => {
			hit = true
			return HttpResponse.json(SUBTITLE)
		}),
	)
	renderAt('/field-groups')
	const dialog = await openFields()

	await userEvent.click(within(dialog).getByRole('button', { name: 'Rename Subtitle' }))
	const renaming = await screen.findByRole('dialog', { name: 'Rename Subtitle' })
	await userEvent.click(within(renaming).getByRole('button', { name: 'Cancel' }))

	await waitFor(() => expect(screen.queryByRole('dialog', { name: 'Rename Subtitle' })).not.toBeInTheDocument())
	expect(hit).toBe(false)
})

test.each([
	{ form: 'Rename Subtitle', send: 'Rename' },
	{ form: 'Settings of Subtitle', send: 'Save settings' },
	{ form: 'Move Subtitle elsewhere', send: 'Move the field' },
])('keeps every control of $form read only while its write runs', async ({ form, send }) => {
	const answer = gate()
	server.use(
		http.patch('/api/groups/3/fields/subtitle', async () => {
			await answer.held
			return HttpResponse.json(SUBTITLE)
		}),
		http.post('/api/groups/3/fields/subtitle/move', async () => {
			await answer.held
			return HttpResponse.json(SUBTITLE)
		}),
	)
	renderAt('/field-groups')
	const dialog = await openFields()

	await userEvent.click(within(dialog).getByRole('button', { name: form }))
	const asked = await screen.findByRole('dialog', { name: form })
	await userEvent.click(within(asked).getByRole('button', { name: send }))
	await waitBusy(asked, send)

	expectReadOnly(asked)
	answer.release()
	await waitFor(() => expect(screen.queryByRole('dialog', { name: form })).not.toBeInTheDocument())
})

test('offers the settings a text field takes and stores them', async () => {
	let sent: unknown
	server.use(
		http.patch('/api/groups/3/fields/subtitle', async ({ request }) => {
			sent = await request.json()
			return HttpResponse.json({ ...SUBTITLE, settings: { maxlength: 80 } })
		}),
	)
	renderAt('/field-groups')
	const dialog = await openFields()

	await userEvent.click(within(dialog).getByRole('button', { name: 'Settings of Subtitle' }))
	const settings = await screen.findByRole('dialog', { name: 'Settings of Subtitle' })
	await userEvent.type(within(settings).getByLabelText('Instructions'), 'Say who wrote it.')
	await userEvent.type(within(settings).getByLabelText('Longest'), '80')
	await userEvent.click(within(settings).getByRole('button', { name: 'Save settings' }))

	await waitFor(() =>
		expect(sent).toEqual({
			settings: { instructions: 'Say who wrote it.', maxlength: 80 },
			updated_at: '2026-08-01T10:00:00Z',
		}),
	)
})

test('offers a number field its own bounds', async () => {
	listing([{ ...DETAILS, fields: [READING_TIME] }, EXTRAS])
	let sent: unknown
	server.use(
		http.patch('/api/groups/3/fields/reading-time', async ({ request }) => {
			sent = await request.json()
			return HttpResponse.json(READING_TIME)
		}),
	)
	renderAt('/field-groups')
	const dialog = await openFields()

	await userEvent.click(within(dialog).getByRole('button', { name: 'Settings of Reading time' }))
	const settings = await screen.findByRole('dialog', { name: 'Settings of Reading time' })
	await userEvent.type(within(settings).getByLabelText('Lowest'), '1')
	await userEvent.type(within(settings).getByLabelText('Highest'), '10')
	await userEvent.click(within(settings).getByRole('button', { name: 'Save settings' }))

	await waitFor(() =>
		expect(sent).toEqual({ settings: { min: 1, max: 10 }, updated_at: '2026-08-01T10:00:00Z' }),
	)
})

test('offers a media field nothing but instructions', async () => {
	listing([{ ...DETAILS, fields: [{ ...SUBTITLE, key: 'cover', label: 'Cover', kind: 'media' }] }])
	renderAt('/field-groups')
	const dialog = await openFields()

	await userEvent.click(within(dialog).getByRole('button', { name: 'Settings of Cover' }))
	const settings = await screen.findByRole('dialog', { name: 'Settings of Cover' })

	expect(within(settings).getByLabelText('Instructions')).toBeInTheDocument()
	expect(within(settings).queryByLabelText('Longest')).not.toBeInTheDocument()
	expect(within(settings).queryByLabelText('Default')).not.toBeInTheDocument()
})

test('shows the settings a field already carries', async () => {
	listing([
		{ ...DETAILS, fields: [{ ...SUBTITLE, settings: { instructions: 'Held.', maxlength: 40 } }] },
	])
	renderAt('/field-groups')
	const dialog = await openFields()

	await userEvent.click(within(dialog).getByRole('button', { name: 'Settings of Subtitle' }))
	const settings = await screen.findByRole('dialog', { name: 'Settings of Subtitle' })

	expect(within(settings).getByLabelText('Instructions')).toHaveValue('Held.')
	expect(within(settings).getByLabelText('Longest')).toHaveValue(40)
})

test('stores nothing for a setting left empty', async () => {
	let sent: unknown
	server.use(
		http.patch('/api/groups/3/fields/subtitle', async ({ request }) => {
			sent = await request.json()
			return HttpResponse.json(SUBTITLE)
		}),
	)
	renderAt('/field-groups')
	const dialog = await openFields()

	await userEvent.click(within(dialog).getByRole('button', { name: 'Settings of Subtitle' }))
	const settings = await screen.findByRole('dialog', { name: 'Settings of Subtitle' })
	await userEvent.click(within(settings).getByRole('button', { name: 'Save settings' }))

	await waitFor(() =>
		expect(sent).toEqual({ settings: {}, updated_at: '2026-08-01T10:00:00Z' }),
	)
})

test('keeps the settings open and says inside why they were turned away', async () => {
	server.use(
		http.patch('/api/groups/3/fields/subtitle', () =>
			HttpResponse.json(
				{ error: 'content: settings disagree', code: 'setting_bounds', meta: { setting: 'min' } },
				{ status: 422 },
			),
		),
	)
	renderAt('/field-groups')
	const dialog = await openFields()

	await userEvent.click(within(dialog).getByRole('button', { name: 'Settings of Subtitle' }))
	const settings = await screen.findByRole('dialog', { name: 'Settings of Subtitle' })
	await userEvent.click(within(settings).getByRole('button', { name: 'Save settings' }))

	const said = 'The setting min falls outside the others. Keep min below max and the default inside the bounds.'
	expect(await within(settings).findByText(said)).toBeInTheDocument()
	expect(screen.getAllByText(said)).toHaveLength(1)
})

test('keeps what was typed in the open settings when they are turned away', async () => {
	server.use(
		http.patch('/api/groups/3/fields/subtitle', () =>
			HttpResponse.json(
				{ error: 'content: setting shape', code: 'setting_shape', meta: { setting: 'maxlength' } },
				{ status: 422 },
			),
		),
	)
	renderAt('/field-groups')
	const dialog = await openFields()

	await userEvent.click(within(dialog).getByRole('button', { name: 'Settings of Subtitle' }))
	const settings = await screen.findByRole('dialog', { name: 'Settings of Subtitle' })
	await userEvent.type(within(settings).getByLabelText('Instructions'), 'Say who wrote it.')
	await userEvent.click(within(settings).getByRole('button', { name: 'Save settings' }))

	expect(await within(settings).findByText(/The setting maxlength cannot hold/)).toBeInTheDocument()
	expect(screen.getByRole('dialog', { name: 'Settings of Subtitle' })).toBe(settings)
	expect(within(settings).getByLabelText('Instructions')).toHaveValue('Say who wrote it.')
})

test('keeps a setting the dialog does not offer', async () => {
	listing([
		{ ...DETAILS, fields: [{ ...READING_TIME, settings: { placeholder: 'How many minutes' } }] },
	])
	let sent: unknown
	server.use(
		http.patch('/api/groups/3/fields/reading-time', async ({ request }) => {
			sent = await request.json()
			return HttpResponse.json(READING_TIME)
		}),
	)
	renderAt('/field-groups')
	const dialog = await openFields()

	await userEvent.click(within(dialog).getByRole('button', { name: 'Settings of Reading time' }))
	const settings = await screen.findByRole('dialog', { name: 'Settings of Reading time' })
	await userEvent.type(within(settings).getByLabelText('Lowest'), '1')
	await userEvent.click(within(settings).getByRole('button', { name: 'Save settings' }))

	await waitFor(() =>
		expect(sent).toEqual({
			settings: { placeholder: 'How many minutes', min: 1 },
			updated_at: '2026-08-01T10:00:00Z',
		}),
	)
})

test('offers a number field no placeholder, which its control never shows', async () => {
	listing([{ ...DETAILS, fields: [READING_TIME] }])
	renderAt('/field-groups')
	const dialog = await openFields()

	await userEvent.click(within(dialog).getByRole('button', { name: 'Settings of Reading time' }))
	const settings = await screen.findByRole('dialog', { name: 'Settings of Reading time' })

	expect(within(settings).queryByLabelText('Placeholder')).not.toBeInTheDocument()
})

test('offers a choice field the flags its kind takes', async () => {
	listing([
		{ ...DETAILS, fields: [{ ...SUBTITLE, key: 'style', label: 'Style', kind: 'choice' }] },
	])
	let sent: unknown
	server.use(
		http.patch('/api/groups/3/fields/style', async ({ request }) => {
			sent = await request.json()
			return HttpResponse.json({ ...SUBTITLE, key: 'style', kind: 'choice' })
		}),
	)
	renderAt('/field-groups')
	const dialog = await openFields()

	await userEvent.click(within(dialog).getByRole('button', { name: 'Settings of Style' }))
	const settings = await screen.findByRole('dialog', { name: 'Settings of Style' })
	await userEvent.click(within(settings).getByRole('combobox', { name: 'Allow custom' }))
	await userEvent.click(await screen.findByRole('option', { name: 'Yes' }))
	await userEvent.click(within(settings).getByRole('button', { name: 'Save settings' }))

	await waitFor(() =>
		expect(sent).toEqual({ settings: { allow_custom: true }, updated_at: '2026-08-01T10:00:00Z' }),
	)
})

test('offers a boolean field the value it starts on', async () => {
	listing([
		{ ...DETAILS, fields: [{ ...SUBTITLE, key: 'boxed', label: 'Boxed', kind: 'boolean' }] },
	])
	let sent: unknown
	server.use(
		http.patch('/api/groups/3/fields/boxed', async ({ request }) => {
			sent = await request.json()
			return HttpResponse.json(SUBTITLE)
		}),
	)
	renderAt('/field-groups')
	const dialog = await openFields()

	await userEvent.click(within(dialog).getByRole('button', { name: 'Settings of Boxed' }))
	const settings = await screen.findByRole('dialog', { name: 'Settings of Boxed' })
	await userEvent.click(within(settings).getByRole('combobox', { name: 'Default' }))
	await userEvent.click(await screen.findByRole('option', { name: 'Yes' }))
	await userEvent.click(within(settings).getByRole('button', { name: 'Save settings' }))

	await waitFor(() =>
		expect(sent).toEqual({ settings: { default: true }, updated_at: '2026-08-01T10:00:00Z' }),
	)
})

test('offers a repeater the count of rows it takes and stores it', async () => {
	listing([
		{ ...DETAILS, fields: [{ ...SUBTITLE, key: 'team', label: 'Team', kind: 'repeater', fields: [] }] },
	])
	let sent: unknown
	server.use(
		http.patch('/api/groups/3/fields/team', async ({ request }) => {
			sent = await request.json()
			return HttpResponse.json(SUBTITLE)
		}),
	)
	renderAt('/field-groups')
	const dialog = await openFields()

	await userEvent.click(within(dialog).getByRole('button', { name: 'Settings of Team' }))
	const settings = await screen.findByRole('dialog', { name: 'Settings of Team' })
	await userEvent.type(within(settings).getByLabelText('Fewest rows'), '1')
	await userEvent.type(within(settings).getByLabelText('Most rows'), '4')
	await userEvent.click(within(settings).getByRole('button', { name: 'Save settings' }))

	await waitFor(() =>
		expect(sent).toEqual({ settings: { min: 1, max: 4 }, updated_at: '2026-08-01T10:00:00Z' }),
	)
})

test('offers a layout the count of rows it takes and stores it', async () => {
	listing([{ ...DETAILS, fields: [{ ...FEATURES, fields: [HERO] }] }])
	let sent: unknown
	server.use(
		http.patch('/api/groups/3/fields/features.hero', async ({ request }) => {
			sent = await request.json()
			return HttpResponse.json(HERO)
		}),
	)
	renderAt('/field-groups')
	const dialog = await openFields()

	await userEvent.click(within(dialog).getByRole('button', { name: 'Settings of Hero' }))
	const settings = await screen.findByRole('dialog', { name: 'Settings of Hero' })
	await userEvent.type(within(settings).getByLabelText('Most rows'), '2')
	await userEvent.click(within(settings).getByRole('button', { name: 'Save settings' }))

	await waitFor(() =>
		expect(sent).toEqual({ settings: { max: 2 }, updated_at: '2026-08-01T10:00:00Z' }),
	)
})

test('offers a flexible content field the count of rows it takes', async () => {
	listing([{ ...DETAILS, fields: [FEATURES] }])
	renderAt('/field-groups')
	const dialog = await openFields()

	await userEvent.click(within(dialog).getByRole('button', { name: 'Settings of Features' }))
	const settings = await screen.findByRole('dialog', { name: 'Settings of Features' })

	expect(within(settings).getByLabelText('Fewest rows')).toBeInTheDocument()
	expect(within(settings).getByLabelText('Most rows')).toBeInTheDocument()
})

test('offers a text field no count of rows', async () => {
	renderAt('/field-groups')
	const dialog = await openFields()

	await userEvent.click(within(dialog).getByRole('button', { name: 'Settings of Subtitle' }))
	const settings = await screen.findByRole('dialog', { name: 'Settings of Subtitle' })

	expect(within(settings).queryByLabelText('Most rows')).not.toBeInTheDocument()
})

test('offers no require button on a layout or a linked from field', async () => {
	listing([
		{
			...DETAILS,
			fields: [
				{ ...FEATURES, fields: [HERO] },
				{ ...SUBTITLE, key: 'linked-from', label: 'Linked from', kind: 'backlinks' },
			],
		},
		EXTRAS,
	])
	renderAt('/field-groups')
	const dialog = await openFields()

	expect(within(dialog).getByRole('button', { name: 'Require Features' })).toBeInTheDocument()
	expect(within(dialog).queryByRole('button', { name: 'Require Hero' })).not.toBeInTheDocument()
	expect(within(dialog).queryByRole('button', { name: 'Require Linked from' })).not.toBeInTheDocument()
})

test('offers a gallery the count of files it takes and stores it', async () => {
	listing([{ ...DETAILS, fields: [{ ...SUBTITLE, key: 'gallery', label: 'Gallery', kind: 'media', many: true }] }])
	let sent: unknown
	server.use(
		http.patch('/api/groups/3/fields/gallery', async ({ request }) => {
			sent = await request.json()
			return HttpResponse.json(SUBTITLE)
		}),
	)
	renderAt('/field-groups')
	const dialog = await openFields()

	await userEvent.click(within(dialog).getByRole('button', { name: 'Settings of Gallery' }))
	const settings = await screen.findByRole('dialog', { name: 'Settings of Gallery' })
	await userEvent.type(within(settings).getByLabelText('Fewest files'), '2')
	await userEvent.type(within(settings).getByLabelText('Most files'), '6')
	await userEvent.click(within(settings).getByRole('button', { name: 'Save settings' }))

	await waitFor(() =>
		expect(sent).toEqual({ settings: { min: 2, max: 6 }, updated_at: '2026-08-01T10:00:00Z' }),
	)
})

test('offers a media field holding one file no count', async () => {
	listing([{ ...DETAILS, fields: [{ ...SUBTITLE, key: 'cover', label: 'Cover', kind: 'media' }] }])
	renderAt('/field-groups')
	const dialog = await openFields()

	await userEvent.click(within(dialog).getByRole('button', { name: 'Settings of Cover' }))
	const settings = await screen.findByRole('dialog', { name: 'Settings of Cover' })

	expect(within(settings).queryByLabelText('Fewest files')).not.toBeInTheDocument()
})

test('keeps the rename open and says inside why it was turned away', async () => {
	server.use(
		http.patch('/api/groups/3/fields/subtitle', () =>
			HttpResponse.json({ error: 'content: field not found', code: 'field_unknown' }, { status: 404 }),
		),
	)
	renderAt('/field-groups')
	const dialog = await openFields()

	await userEvent.click(within(dialog).getByRole('button', { name: 'Rename Subtitle' }))
	const renaming = await screen.findByRole('dialog', { name: 'Rename Subtitle' })
	await userEvent.click(within(renaming).getByRole('button', { name: 'Rename' }))

	expect(await within(renaming).findByText('content: field not found')).toBeInTheDocument()
	expect(screen.getAllByText('content: field not found')).toHaveLength(1)
})

test('stores no settings when the dialog is abandoned', async () => {
	let hit = false
	server.use(
		http.patch('/api/groups/3/fields/subtitle', () => {
			hit = true
			return HttpResponse.json(SUBTITLE)
		}),
	)
	renderAt('/field-groups')
	const dialog = await openFields()

	await userEvent.click(within(dialog).getByRole('button', { name: 'Settings of Subtitle' }))
	const settings = await screen.findByRole('dialog', { name: 'Settings of Subtitle' })
	await userEvent.type(within(settings).getByLabelText('Instructions'), 'Never stored.')
	await userEvent.click(within(settings).getByRole('button', { name: 'Cancel' }))

	await waitFor(() =>
		expect(screen.queryByRole('dialog', { name: 'Settings of Subtitle' })).not.toBeInTheDocument(),
	)
	expect(hit).toBe(false)
})
