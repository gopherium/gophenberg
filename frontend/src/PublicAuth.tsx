// SPDX-License-Identifier: Apache-2.0

import { ACCESS_ADMIN, can } from '@gophenberg/frontend-sdk'
import type { User } from '@gopherium/react-auth'
import { SetPasswordScreen } from '@gopherium/react-auth/wpds'
import type { ReactNode } from 'react'

import { adminBasepath } from './basepath'

/** The product name the sign in and activation pages carry. */
export const BRAND = 'Gophenberg'

/** The admin page an activation link opens, the path the server mails. */
const ACTIVATE_PATH = adminBasepath + '/activate'

/**
 * Returns where an activated account lands, the admin for one that may open it and the public site otherwise.
 * @param user - The account the activation signed in.
 * @returns The path the account lands on.
 */
function landingOf(user: User): string {
	return can(user, ACCESS_ADMIN) ? adminBasepath + '/' : '/'
}

/**
 * Returns the page an activation link opens, or null for every other path.
 * @param url - The address the reader arrived at.
 * @param go - Opens the path the reader lands on once the password is set.
 * @returns The activation page, or null when the path carries no link.
 */
export function publicAuthScreen(url: URL, go: (path: string) => void): ReactNode | null {
	if (url.pathname !== ACTIVATE_PATH) {
		return null
	}
	return (
		<SetPasswordScreen
			brand={BRAND}
			token={url.searchParams.get('token') ?? ''}
			onAccepted={(user) => go(landingOf(user))}
		/>
	)
}
