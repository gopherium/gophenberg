// SPDX-License-Identifier: Apache-2.0

import { ADMIN, AUTHOR, EDITOR } from '@gophenberg/frontend-sdk'
import { _x } from '@wordpress/i18n'

/** The rank a new account starts under. */
export const NARROWEST = AUTHOR

/**
 * Returns the ranks an account may hold, labelled for the reader.
 * @returns The rank options, widest first.
 */
export function rankOptions(): { value: string, label: string }[] {
	return [
		{ value: ADMIN, label: _x('Administrator', 'rank', 'gophenberg') },
		{ value: EDITOR, label: _x('Editor', 'rank', 'gophenberg') },
		{ value: AUTHOR, label: _x('Author', 'rank', 'gophenberg') },
	]
}

/**
 * Returns the label a rank reads as, falling back to what the server stored.
 * @param rank - The rank the account holds.
 * @returns The label.
 */
export function rankLabel(rank: string): string {
	if (rank === '') {
		return _x('Unranked', 'rank', 'gophenberg')
	}
	return rankOptions().find((option) => option.value === rank)?.label ?? rank
}
