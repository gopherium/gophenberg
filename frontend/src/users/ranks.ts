// SPDX-License-Identifier: Apache-2.0

import { _x } from '@wordpress/i18n'

/** The rank holding every authority over the site. */
export const ADMIN = 'admin'

/** The rank working every account's content and media. */
export const EDITOR = 'editor'

/** The rank working only its own content and media. */
export const AUTHOR = 'author'

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
 * Reports whether a rank reaches the administration screens.
 * @param rank - The rank the account holds, missing counting as none.
 * @returns Whether the administration screens are open to it.
 */
export function isAdmin(rank: string | undefined): boolean {
	return rank === ADMIN
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
