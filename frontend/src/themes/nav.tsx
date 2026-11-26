// SPDX-License-Identifier: Apache-2.0

import { brush } from '@gophenberg/frontend-sdk'
import type { MenuItem } from '@gophenberg/frontend-sdk'
import { _x } from '@wordpress/i18n'

export const themesMenuItem: MenuItem = {
	get label() {
		return _x('Themes', 'admin section', 'gophenberg')
	},
	to: '/themes',
	icon: brush,
}
