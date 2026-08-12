// SPDX-License-Identifier: Apache-2.0

import type { NavItem } from '@gophenberg/frontend-sdk'

const postsIcon = (
	<svg viewBox="0 0 24 24" xmlns="http://www.w3.org/2000/svg" fill="currentColor">
		<path d="M5 3h11l3 3v15H5V3zm2 2v14h10V7h-3V5H7zm2 4h6v2H9V9zm0 4h6v2H9v-2z" />
	</svg>
)

export const postsNavItem: NavItem = {
	label: 'Posts',
	to: '/posts',
	icon: postsIcon,
}
