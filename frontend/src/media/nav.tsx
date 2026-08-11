// SPDX-License-Identifier: Apache-2.0

import type { NavItem } from '@gophenberg/frontend-sdk'

const mediaIcon = (
	<svg viewBox="0 0 24 24" xmlns="http://www.w3.org/2000/svg" fill="currentColor">
		<path d="M4 5h16v14H4V5zm2 2v7l3.5-3.5L13 14l2.5-2.5L18 14V7H6zm0 10h12l-2.5-2.5L13 17l-3.5-3.5L6 17z" />
	</svg>
)

export const mediaNavItem: NavItem = {
	label: 'Media',
	to: '/media',
	icon: mediaIcon,
}
