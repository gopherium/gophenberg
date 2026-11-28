// SPDX-License-Identifier: Apache-2.0

import { http, HttpResponse, server } from '@gophenberg/frontend-sdk/testing'
import { waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { expect, test } from 'vitest'

import { gate } from './gate'
import { DETAILS, listing, openField, openFields, saveOf, serveFieldGroups, SUBTITLE } from './groupFieldsFixtures'
import { renderAt } from './render'
import { waitBusy } from './typeFixtures'

const STYLE = { ...SUBTITLE, key: 'style', label: 'Style', kind: 'choice' }

/** The pairs a choice field carries before any edit. */
const STORED_PAIRS = [
	{ value: 'ipa', label: 'IPA' },
	{ value: 'stout', label: 'Stout' },
	{ value: 'porter', label: 'Porter' },
]

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

/**
 * Opens the style field of the article details group, carrying the given pairs.
 * @param pairs - The pairs the field carries, none when absent.
 * @returns The main work of the page and the row holding the open part.
 */
async function openStyle(pairs?: unknown[]) {
	const style = pairs === undefined ? STYLE : { ...STYLE, settings: { choices: pairs } }
	listing([{ ...DETAILS, fields: [style] }])
	renderAt('/field-groups')
	const main = await openFields()
	const part = await openField(main, 'Style')
	return { main, part }
}

/**
 * Returns the group of one choice row.
 * @param part - The row holding the open part.
 * @param place - The place of the choice, from 1.
 * @returns The group.
 */
function choiceAt(part: HTMLElement, place: number) {
	return within(part).getByRole('group', { name: `Choice ${place}` })
}

/**
 * Returns one box of a choice row.
 * @param part - The row holding the open part.
 * @param place - The place of the choice, from 1.
 * @param name - The name of the box.
 * @returns The box.
 */
function boxOf(part: HTMLElement, place: number, name: 'Value' | 'Label') {
	return within(choiceAt(part, place)).getByRole('textbox', { name })
}

/**
 * Returns one button of a choice row.
 * @param part - The row holding the open part.
 * @param place - The place of the choice, from 1.
 * @param name - The name of the button.
 * @returns The button.
 */
function buttonOf(part: HTMLElement, place: number, name: string) {
	return within(choiceAt(part, place)).getByRole('button', { name })
}

/**
 * Returns what the Value boxes of the choice rows hold, in order.
 * @param part - The row holding the open part.
 * @returns The values.
 */
function valuesOf(part: HTMLElement) {
	return within(part)
		.getAllByRole('textbox', { name: 'Value' })
		.map((box) => (box as HTMLInputElement).value)
}

/**
 * Returns the body a save of the style field sends with the given pairs.
 * @param choices - The pairs the save stores.
 * @returns The body.
 */
function savedWith(choices: unknown[]) {
	return { label: 'Style', required: false, settings: { choices }, updated_at: '2026-08-01T10:00:00Z' }
}

test('adds an empty pair from Add choice and stores it from the header Save', async () => {
	const held = recordingStyle()
	const { main, part } = await openStyle()

	await userEvent.click(within(part).getByRole('button', { name: 'Add choice' }))
	expect(boxOf(part, 1, 'Value')).toHaveValue('')
	expect(boxOf(part, 1, 'Label')).toHaveValue('')
	await userEvent.type(boxOf(part, 1, 'Value'), 'ipa')
	await userEvent.type(boxOf(part, 1, 'Label'), 'IPA')
	await userEvent.click(saveOf(main))

	await waitFor(() => expect(held.sent).toEqual(savedWith([{ value: 'ipa', label: 'IPA' }])))
})

test('names each choice row as a group by its place, holding its value and label', async () => {
	const { part } = await openStyle(STORED_PAIRS)

	STORED_PAIRS.forEach((pair, at) => {
		expect(boxOf(part, at + 1, 'Value')).toHaveValue(pair.value)
		expect(boxOf(part, at + 1, 'Label')).toHaveValue(pair.label)
	})
	expect(within(part).queryByRole('group', { name: 'Choice 4' })).not.toBeInTheDocument()
})

test('holds every choice row and Add choice in one group named Choices', async () => {
	const { part } = await openStyle(STORED_PAIRS)

	const choices = within(part).getByRole('group', { name: 'Choices' })

	expect(within(choices).getAllByRole('group', { name: /^Choice \d$/ })).toHaveLength(STORED_PAIRS.length)
	expect(within(choices).getByRole('button', { name: 'Add choice' })).toBeInTheDocument()
	expect(within(part).getAllByText('Choices')).toHaveLength(1)
})

test('moves a pair down and up with its arrows and stores the new order', async () => {
	const held = recordingStyle()
	const { main, part } = await openStyle(STORED_PAIRS)
	expect(buttonOf(part, 1, 'Move choice up')).toHaveAttribute('aria-disabled', 'true')
	expect(buttonOf(part, 3, 'Move choice down')).toHaveAttribute('aria-disabled', 'true')

	await userEvent.click(buttonOf(part, 1, 'Move choice down'))
	expect(valuesOf(part)).toEqual(['stout', 'ipa', 'porter'])
	await userEvent.click(buttonOf(part, 3, 'Move choice up'))
	expect(valuesOf(part)).toEqual(['stout', 'porter', 'ipa'])
	await userEvent.click(saveOf(main))

	await waitFor(() =>
		expect(held.sent).toEqual(savedWith([STORED_PAIRS[1], STORED_PAIRS[2], STORED_PAIRS[0]])),
	)
})

test('removes a pair with its trash and stores the edited pairs left', async () => {
	const held = recordingStyle()
	const { main, part } = await openStyle(STORED_PAIRS.slice(0, 2))

	await userEvent.type(boxOf(part, 2, 'Label'), ' Beer')
	await userEvent.type(boxOf(part, 2, 'Value'), 's')
	await userEvent.click(buttonOf(part, 1, 'Remove choice'))
	expect(valuesOf(part)).toEqual(['stouts'])
	await userEvent.click(saveOf(main))

	await waitFor(() => expect(held.sent).toEqual(savedWith([{ value: 'stouts', label: 'Stout Beer' }])))
})

test('keeps the focus in a choice box once an edit brings it back to the stored text', async () => {
	const { part } = await openStyle(STORED_PAIRS)

	await userEvent.type(boxOf(part, 1, 'Label'), 'x')
	await userEvent.keyboard('{Backspace}')
	expect(boxOf(part, 1, 'Label')).toHaveFocus()
	await userEvent.keyboard(' Ale')

	expect(boxOf(part, 1, 'Label')).toHaveValue('IPA Ale')
})

test('moves the focus to the group of the row Add choice adds', async () => {
	const { part } = await openStyle(STORED_PAIRS)

	await userEvent.click(within(part).getByRole('button', { name: 'Add choice' }))

	expect(choiceAt(part, 4)).toHaveFocus()
})

test('moves the focus to the group of the row taking the place of the one its trash removed', async () => {
	const { part } = await openStyle(STORED_PAIRS)

	await userEvent.click(buttonOf(part, 1, 'Remove choice'))

	expect(choiceAt(part, 1)).toHaveFocus()
	expect(boxOf(part, 1, 'Value')).toHaveValue('stout')
})

test('says no choices are held while the field holds no pair, and again once the last one goes', async () => {
	const { part } = await openStyle()
	expect(within(part).getByText('No choices yet.')).toBeInTheDocument()

	await userEvent.click(within(part).getByRole('button', { name: 'Add choice' }))
	expect(within(part).queryByText('No choices yet.')).not.toBeInTheDocument()
	await userEvent.click(buttonOf(part, 1, 'Remove choice'))

	expect(within(part).getByText('No choices yet.')).toBeInTheDocument()
})

test('drops a pair holding only a value or only a label on save and keeps the whole ones', async () => {
	const held = recordingStyle()
	const { main, part } = await openStyle(STORED_PAIRS.slice(0, 1))

	await userEvent.click(within(part).getByRole('button', { name: 'Add choice' }))
	await userEvent.type(boxOf(part, 2, 'Value'), 'stout')
	await userEvent.click(within(part).getByRole('button', { name: 'Add choice' }))
	await userEvent.type(boxOf(part, 3, 'Label'), 'Porter')
	await userEvent.click(saveOf(main))

	await waitFor(() => expect(held.sent).toEqual(savedWith([STORED_PAIRS[0]])))
})

test('keeps every choice box read only and the rows from changing while the header Save runs', async () => {
	const answer = gate()
	server.use(
		http.patch('/api/groups/3/fields/style', async () => {
			await answer.held
			return HttpResponse.json(STYLE)
		}),
	)
	const { main, part } = await openStyle(STORED_PAIRS)
	await userEvent.type(boxOf(part, 1, 'Label'), ' Ale')

	await userEvent.click(saveOf(main))
	await waitBusy(main, 'Save')

	STORED_PAIRS.forEach((_, at) => {
		expect(boxOf(part, at + 1, 'Value')).toHaveAttribute('readonly')
		expect(boxOf(part, at + 1, 'Label')).toHaveAttribute('readonly')
		expect(buttonOf(part, at + 1, 'Remove choice')).toHaveAttribute('aria-disabled', 'true')
	})
	expect(within(part).getByRole('button', { name: 'Add choice' })).toHaveAttribute('aria-disabled', 'true')
	await userEvent.click(buttonOf(part, 1, 'Move choice down'))
	expect(valuesOf(part)).toEqual(['ipa', 'stout', 'porter'])
	answer.release()
	await waitFor(() => expect(saveOf(main).className).not.toMatch(/__is-loading\b/))
})

test('shows only the pairs a stray choices value actually holds', async () => {
	const { part } = await openStyle([{ value: 'ipa', label: 'IPA' }, 'stray', { value: 1, label: 'One' }])

	expect(valuesOf(part)).toEqual(['ipa'])
	expect(within(part).queryByRole('group', { name: 'Choice 2' })).not.toBeInTheDocument()
})
