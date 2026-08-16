// SPDX-License-Identifier: AGPL-3.0-or-later

import type { NavItem } from '@gophenberg/frontend-sdk'
import { __ } from '@wordpress/i18n'

const languageIcon = (
	<svg viewBox="0 0 24 24" xmlns="http://www.w3.org/2000/svg" fill="currentColor">
		<path
			d="M12 2a10 10 0 100 20 10 10 0 000-20zm0 2c1.7 0 3.2 2.9 3.7 7h-7.4C8.8 6.9
			10.3 4 12 4zm0 16c-1.7 0-3.2-2.9-3.7-7h7.4c-.5 4.1-2 7-3.7 7z"
		/>
	</svg>
)

export const languageNavItem: NavItem = {
	label: __('Language', 'gophenberg'),
	to: '/language',
	icon: languageIcon,
}
