// SPDX-License-Identifier: Apache-2.0

import { http, HttpResponse, server } from '@gophenberg/frontend-sdk/testing'
import { screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { expect, test } from 'vitest'

import { DETAILS, listing, openFields, serveFieldGroups, SUBTITLE } from './groupFieldsFixtures'
import { renderAt } from './render'

serveFieldGroups()

test('offers a choice field its choices and stores the pairs', async () => {
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
	await userEvent.click(within(settings).getByRole('button', { name: 'Add choice' }))
	await userEvent.type(within(settings).getByLabelText('Value'), 'ipa')
	await userEvent.type(within(settings).getByLabelText('Label'), 'IPA')
	await userEvent.click(within(settings).getByRole('button', { name: 'Save settings' }))

	await waitFor(() =>
		expect(sent).toEqual({
			settings: { choices: [{ value: 'ipa', label: 'IPA' }] },
			updated_at: '2026-08-01T10:00:00Z',
		}),
	)
})

test('shows the pairs a choice field already carries and drops a removed one', async () => {
	listing([
		{
			...DETAILS,
			fields: [
				{
					...SUBTITLE,
					key: 'style',
					label: 'Style',
					kind: 'choice',
					settings: {
						choices: [
							{ value: 'ipa', label: 'IPA' },
							{ value: 'stout', label: 'Stout' },
						],
					},
				},
			],
		},
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
	expect(within(settings).getAllByLabelText('Value')).toHaveLength(2)

	await userEvent.type(within(settings).getAllByLabelText('Label')[1], ' Beer')
	await userEvent.type(within(settings).getAllByLabelText('Value')[1], 's')
	await userEvent.click(within(settings).getAllByRole('button', { name: 'Remove' })[0])
	await userEvent.click(within(settings).getByRole('button', { name: 'Save settings' }))

	await waitFor(() =>
		expect(sent).toEqual({
			settings: { choices: [{ value: 'stouts', label: 'Stout Beer' }] },
			updated_at: '2026-08-01T10:00:00Z',
		}),
	)
})

test('shows only the pairs a stray choices value actually holds', async () => {
	listing([
		{
			...DETAILS,
			fields: [
				{
					...SUBTITLE,
					key: 'style',
					label: 'Style',
					kind: 'choice',
					settings: {
						choices: [{ value: 'ipa', label: 'IPA' }, 'stray', { value: 1, label: 'One' }],
					},
				},
			],
		},
	])
	renderAt('/field-groups')
	const dialog = await openFields()

	await userEvent.click(within(dialog).getByRole('button', { name: 'Settings of Style' }))
	const settings = await screen.findByRole('dialog', { name: 'Settings of Style' })

	expect(within(settings).getAllByLabelText('Value')).toHaveLength(1)
	expect(within(settings).getByLabelText('Value')).toHaveValue('ipa')
})
