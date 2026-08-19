// SPDX-License-Identifier: Apache-2.0

import { Text } from '@gophenberg/frontend-sdk'
import { Page } from '@gopherium/godmin'
import { __ } from '@wordpress/i18n'

/**
 * Renders the application's home screen.
 * @returns The home screen element.
 */
export function Home() {
	return (
		<Page title={__('Home', 'gophenberg')}>
			<Text>{__('Welcome to Gophenberg.', 'gophenberg')}</Text>
			<Text>{__('This line proves the translation pipeline and is removed again.', 'gophenberg')}</Text>
		</Page>
	)
}
