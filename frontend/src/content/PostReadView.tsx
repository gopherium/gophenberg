// SPDX-License-Identifier: Apache-2.0

import { Notice, Text } from '@gophenberg/frontend-sdk'
import { BlockPreview, parse } from '@gophenberg/frontend-sdk/editor'
import { Page } from '@gopherium/godmin'
import { __ } from '@wordpress/i18n'

import type { PostDetail } from './api'

/**
 * Renders another account's post as a reading view without a form.
 * @param props - The post to read.
 * @returns The reading view element.
 */
export function PostReadView({ stored }: { stored: PostDetail }) {
	return (
		<Page title={stored.title === '' ? __('(no title)', 'gophenberg') : stored.title}>
			<Notice.Root intent="info">
				<Notice.Description>
					{__('Another account wrote this, so you are reading it rather than editing it.', 'gophenberg')}
				</Notice.Description>
			</Notice.Root>
			{stored.excerpt !== '' && <Text>{stored.excerpt}</Text>}
			<BlockPreview blocks={parse(stored.content)} />
		</Page>
	)
}
