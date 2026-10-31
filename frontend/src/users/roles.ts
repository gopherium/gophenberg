// SPDX-License-Identifier: Apache-2.0

import { ADMIN, AUTHOR, EDITOR, MEMBER } from '@gophenberg/frontend-sdk'
import type { RoleLabels } from '@gopherium/react-auth/wpds/users'
import { _x } from '@wordpress/i18n'

/**
 * Returns the label each core role reads as, read fresh so the loaded catalogue answers.
 * @returns The labels, keyed by role.
 */
export function roleLabels(): RoleLabels {
	return {
		[ADMIN]: _x('Administrator', 'role', 'gophenberg'),
		[EDITOR]: _x('Editor', 'role', 'gophenberg'),
		[AUTHOR]: _x('Author', 'role', 'gophenberg'),
		[MEMBER]: _x('Member', 'role', 'gophenberg'),
	}
}

/**
 * Returns the role a new account starts under, the site's default when the reader may give it, else the narrowest.
 * @param grantable - The roles the reader may give, widest first.
 * @param defaultRole - The role the settings start new accounts on, none while unknown.
 * @returns The starting role, empty when the reader may give none.
 */
export function startingRole(grantable: readonly string[], defaultRole: string | undefined): string {
	return grantable.find((role) => role === defaultRole) ?? grantable.at(-1) ?? ''
}
