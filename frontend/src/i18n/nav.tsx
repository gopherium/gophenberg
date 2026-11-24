// SPDX-License-Identifier: Apache-2.0

import { language } from '@gophenberg/frontend-sdk'
import type { MenuItem } from '@gophenberg/frontend-sdk'
import { _x } from '@wordpress/i18n'

export const languageMenuItem: MenuItem = {
	get label() {
		return _x('Language', 'admin section', 'gophenberg')
	},
	to: '/language',
	icon: language,
}
