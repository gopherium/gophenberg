// SPDX-License-Identifier: Apache-2.0

import type { NavItem } from '@gophenberg/frontend-sdk'
import { __ } from '@wordpress/i18n'

const themesIcon = (
	<svg viewBox="0 0 24 24" xmlns="http://www.w3.org/2000/svg" fill="currentColor">
		<path d="M3 4h18v16H3V4zm2 4v10h4V8H5zm6 0v10h8V8h-8z" />
	</svg>
)

export const themesNavItem: NavItem = {
	get label() {
		return __('Themes', 'gophenberg')
	},
	to: '/themes',
	icon: themesIcon,
}
