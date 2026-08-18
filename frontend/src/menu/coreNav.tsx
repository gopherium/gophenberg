// SPDX-License-Identifier: Apache-2.0

import type { NavItem } from '@gophenberg/frontend-sdk'
import { usersNavItem } from '@gopherium/react-auth/wpds'

import { languageNavItem } from '../i18n/nav'
import { mediaNavItem } from '../media/nav'
import { typesNavItem } from '../content/nav'
import { themesNavItem } from '../themes/nav'

export const coreNav: NavItem[] = [
	mediaNavItem,
	themesNavItem,
	typesNavItem,
	usersNavItem,
	languageNavItem,
]
