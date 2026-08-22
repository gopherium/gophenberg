// SPDX-License-Identifier: Apache-2.0

import { ADMIN, AUTHOR, EDITOR } from '@gophenberg/frontend-sdk'
import { _x } from '@wordpress/i18n'

/** The role a new account starts under. */
export const NARROWEST = AUTHOR

/**
 * Returns the roles an account may hold, labelled for the reader.
 * @returns The role options, widest first.
 */
export function roleOptions(): { value: string, label: string }[] {
	return [
		{ value: ADMIN, label: _x('Administrator', 'role', 'gophenberg') },
		{ value: EDITOR, label: _x('Editor', 'role', 'gophenberg') },
		{ value: AUTHOR, label: _x('Author', 'role', 'gophenberg') },
	]
}

/**
 * Returns the label a role reads as, falling back to what the server stored.
 * @param role - The role the account holds.
 * @returns The label.
 */
export function roleLabel(role: string): string {
	if (role === '') {
		return _x('No role', 'role', 'gophenberg')
	}
	return roleOptions().find((option) => option.value === role)?.label ?? role
}
