// SPDX-License-Identifier: Apache-2.0

import type { NavItem } from '@gophenberg/frontend-sdk'
import { __ } from '@wordpress/i18n'

const settingsIcon = (
	<svg viewBox="0 0 24 24" xmlns="http://www.w3.org/2000/svg" fill="currentColor">
		<path
			d="M12 8a4 4 0 100 8 4 4 0 000-8zm0 2a2 2 0 110 4 2 2 0 010-4zm-1.6-8h3.2l.4 2.6c.5.2 1
			.4 1.4.7l2.4-1.1 1.6 2.8-2 1.7c0 .3.1.5.1.8s0 .5-.1.8l2 1.7-1.6 2.8-2.4-1.1c-.4.3-.9.5-1.4
			.7l-.4 2.6h-3.2l-.4-2.6c-.5-.2-1-.4-1.4-.7l-2.4 1.1-1.6-2.8 2-1.7c0-.3-.1-.5-.1-.8s0-.5.1-.8l-2-1.7
			1.6-2.8 2.4 1.1c.4-.3.9-.5 1.4-.7L10.4 2z"
		/>
	</svg>
)

export const settingsNavItem: NavItem = {
	get label() {
		return __('Settings', 'gophenberg')
	},
	to: '/settings',
	icon: settingsIcon,
}
