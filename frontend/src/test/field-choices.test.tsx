// SPDX-License-Identifier: Apache-2.0

import { http, HttpResponse, server } from '@gophenberg/frontend-sdk/testing'
import { waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { expect, test } from 'vitest'

import { DETAILS, listing, openField, openFields, saveOf, serveFieldGroups, SUBTITLE } from './groupFieldsFixtures'
import { renderAt } from './render'

const STYLE = { ...SUBTITLE, key: 'style', label: 'Style', kind: 'choice' }

serveFieldGroups()

/**
 * Records the body of the next edit of the style field.
 * @returns The recorder holding what was sent.
 */
function recordingStyle(): { sent: unknown } {
	const held: { sent: unknown } = { sent: undefined }
	server.use(
		http.patch('/api/groups/3/fields/style', async ({ request }) => {
			held.sent = await request.json()
			return HttpResponse.json(STYLE)
		}),
	)
	return held
}

test('offers a choice field its choices in place and stores the pairs from the header Save', async () => {
	listing([{ ...DETAILS, fields: [STYLE] }])
	const held = recordingStyle()
	renderAt('/field-groups')
	const main = await openFields()
	const part = await openField(main, 'Style')

	await userEvent.click(within(part).getByRole('button', { name: 'Add choice' }))
	await userEvent.type(within(part).getByRole('textbox', { name: 'Value' }), 'ipa')
	await userEvent.type(within(part).getAllByRole('textbox', { name: 'Label' })[1], 'IPA')
	await userEvent.click(saveOf(main))

	await waitFor(() =>
		expect(held.sent).toEqual({
			label: 'Style',
			required: false,
			settings: { choices: [{ value: 'ipa', label: 'IPA' }] },
			updated_at: '2026-08-01T10:00:00Z',
		}),
	)
})

test('shows the pairs a choice field already carries and drops a removed one', async () => {
	const pairs = [
		{ value: 'ipa', label: 'IPA' },
		{ value: 'stout', label: 'Stout' },
	]
	listing([{ ...DETAILS, fields: [{ ...STYLE, settings: { choices: pairs } }] }])
	const held = recordingStyle()
	renderAt('/field-groups')
	const main = await openFields()
	const part = await openField(main, 'Style')
	expect(within(part).getAllByRole('textbox', { name: 'Value' })).toHaveLength(2)

	await userEvent.type(within(part).getAllByRole('textbox', { name: 'Label' })[2], ' Beer')
	await userEvent.type(within(part).getAllByRole('textbox', { name: 'Value' })[1], 's')
	await userEvent.click(within(part).getAllByRole('button', { name: 'Remove' })[0])
	await userEvent.click(saveOf(main))

	await waitFor(() =>
		expect(held.sent).toEqual({
			label: 'Style',
			required: false,
			settings: { choices: [{ value: 'stouts', label: 'Stout Beer' }] },
			updated_at: '2026-08-01T10:00:00Z',
		}),
	)
})

test('shows only the pairs a stray choices value actually holds', async () => {
	const stray = [{ value: 'ipa', label: 'IPA' }, 'stray', { value: 1, label: 'One' }]
	listing([{ ...DETAILS, fields: [{ ...STYLE, settings: { choices: stray } }] }])
	renderAt('/field-groups')
	const main = await openFields()

	const part = await openField(main, 'Style')

	expect(within(part).getAllByRole('textbox', { name: 'Value' })).toHaveLength(1)
	expect(within(part).getByRole('textbox', { name: 'Value' })).toHaveValue('ipa')
})
