// SPDX-License-Identifier: Apache-2.0

import { screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { expect } from 'vitest'

/**
 * Opens the Sort by select of the open view options and returns its list of fields, which carries no name of its own.
 * @returns The list the select opened, the one its trigger controls.
 */
async function openedSortList(): Promise<HTMLElement> {
	const trigger = await screen.findByRole('combobox', { name: 'Sort by' })
	await userEvent.click(trigger)
	return waitFor(() => {
		const list = document.getElementById(trigger.getAttribute('aria-controls') ?? '')
		expect(list).toHaveAttribute('role', 'listbox')
		return list as HTMLElement
	})
}

/**
 * Returns the fields the Sort by select of the open view options offers, in order.
 * @returns The labels of the fields.
 */
export async function sortFields(): Promise<string[]> {
	const list = await openedSortList()
	return within(list)
		.getAllByRole('option')
		.map((option) => option.textContent)
}

/**
 * Sorts the list by the field named, through the Sort by select of the open view options.
 * @param label - The label of the field.
 */
export async function sortBy(label: string) {
	const list = await openedSortList()
	await userEvent.click(within(list).getByRole('option', { name: label }))
}
