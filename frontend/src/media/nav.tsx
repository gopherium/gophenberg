// SPDX-License-Identifier: Apache-2.0

import { media } from '@gophenberg/frontend-sdk'
import type { MenuItem } from '@gophenberg/frontend-sdk'
import { _x } from '@wordpress/i18n'

export const mediaMenuItem: MenuItem = {
	get label() {
		return _x('Media', 'admin section', 'gophenberg')
	},
	to: '/media',
	icon: media,
}
