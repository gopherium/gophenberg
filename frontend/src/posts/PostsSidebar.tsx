// SPDX-License-Identifier: Apache-2.0

import { Button, SidebarNavigationScreen, Stack } from '@gophenberg/frontend-sdk'
import { Link } from '@tanstack/react-router'

/**
 * Renders the posts section sidebar screen.
 * @returns The drill-down screen listing the section's entries.
 */
export function PostsSidebar() {
	return (
		<SidebarNavigationScreen title="Posts" backTo="/">
			<Stack direction="column" gap="xs" render={<ul />}>
				<li>
					<Link to="/posts" className="gophenberg-nav-screen__entry">
						All Posts
					</Link>
				</li>
				<li>
					<Button variant="minimal">Add New</Button>
				</li>
			</Stack>
		</SidebarNavigationScreen>
	)
}
