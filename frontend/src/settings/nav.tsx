// SPDX-License-Identifier: Apache-2.0

import { settings } from '@gophenberg/frontend-sdk'
import type { MenuItem } from '@gophenberg/frontend-sdk'
import { _x } from '@wordpress/i18n'

export const settingsMenuItem: MenuItem = {
	get label() {
		return _x('Settings', 'admin section', 'gophenberg')
	},
	to: '/settings',
	icon: settings,
}
