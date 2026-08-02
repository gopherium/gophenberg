// SPDX-License-Identifier: Apache-2.0

import { Text } from '@gophenberg/frontend-sdk'
import { Page } from '@gopherium/godmin'

/**
 * Renders the application's home screen.
 * @returns The home screen element.
 */
export function Home() {
	return (
		<Page title="Home">
			<Text>Welcome to Gophenberg.</Text>
		</Page>
	)
}
