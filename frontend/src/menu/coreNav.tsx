// SPDX-License-Identifier: Apache-2.0

import type { MenuItem } from '@gophenberg/frontend-sdk'
import { usersMenuItem } from '@gopherium/react-auth/wpds'

import { languageMenuItem } from '../i18n/nav'
import { mediaMenuItem } from '../media/nav'
import { groupsMenuItem, typesMenuItem } from '../content/nav'
import { pluginsMenuItem } from '../pluginsscreen/nav'
import { settingsMenuItem } from '../settings/nav'
import { themesMenuItem } from '../themes/nav'

export const coreNav: MenuItem[] = [
	mediaMenuItem,
	themesMenuItem,
	pluginsMenuItem,
	typesMenuItem,
	groupsMenuItem,
	usersMenuItem,
	languageMenuItem,
	settingsMenuItem,
]
