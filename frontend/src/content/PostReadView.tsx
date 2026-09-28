// SPDX-License-Identifier: Apache-2.0

import { Notice, Text } from '@gophenberg/frontend-sdk'
import { BlockPreview, parse } from '@gophenberg/frontend-sdk/editor'
import { Page } from '@gopherium/godmin'
import { __ } from '@wordpress/i18n'

import type { PostDetail } from './api'

/**
 * Renders a post the session may not edit as a reading view without a form.
 * @param props - The post to read and whether the session could otherwise change it.
 * @returns The reading view element.
 */
export function PostReadView({ stored, mine }: { stored: PostDetail, mine: boolean }) {
	return (
		<Page title={stored.title === '' ? __('(no title)', 'gophenberg') : stored.title}>
			{mine ? (
				<Notice.Root intent="warning">
					<Notice.Description>
						{__('This item is in the trash. Restore it first to work on it again.', 'gophenberg')}
					</Notice.Description>
				</Notice.Root>
			) : (
				<Notice.Root intent="info">
					<Notice.Description>
						{__('Another account wrote this, so you are reading it rather than editing it.', 'gophenberg')}
					</Notice.Description>
				</Notice.Root>
			)}
			{stored.excerpt !== '' && <Text>{stored.excerpt}</Text>}
			<BlockPreview blocks={parse(stored.content)} />
		</Page>
	)
}
