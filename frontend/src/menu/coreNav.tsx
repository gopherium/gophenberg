// SPDX-License-Identifier: Apache-2.0

import type { NavItem } from '@gophenberg/frontend-sdk'
import { useSession } from '@gopherium/react-auth'
import { usersNavItem } from '@gopherium/react-auth/wpds'

import { languageNavItem } from '../i18n/nav'
import { mediaNavItem } from '../media/nav'
import { typesNavItem } from '../content/nav'
import { themesNavItem } from '../themes/nav'
import { isAdmin } from '../users/ranks'

export const coreNav: NavItem[] = [
	mediaNavItem,
	themesNavItem,
	typesNavItem,
	usersNavItem,
	languageNavItem,
]

/** The screens only an administrator reaches. */
export const gatedPaths = ['/themes', '/content-types', '/users']

/**
 * Returns the core nav entries the signed in rank may reach.
 * @returns The entries to show.
 */
export function useCoreNav(): NavItem[] {
	const rank = useSession().data?.rank
	if (isAdmin(rank)) {
		return coreNav
	}
	return coreNav.filter((item) => !gatedPaths.includes(item.to))
}
