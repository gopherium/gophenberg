// SPDX-License-Identifier: Apache-2.0

import { Notice, Text } from '@gophenberg/frontend-sdk'
import { BlockPreview, parse } from '@gophenberg/frontend-sdk/editor'
import { Page, PageBreadcrumb, PageBreadcrumbs } from '@gopherium/godmin'
import { Link } from '@tanstack/react-router'
import { __ } from '@wordpress/i18n'

import type { PostDetail } from './api'
import { TrashedNotice } from './TrashedNotice'
import { pluralOf } from './typeText'
import { useContentType } from './useContentType'

/**
 * Renders a post the session may not edit as a reading view without a form.
 * @param props - The post to read and whether the session could otherwise change it.
 * @returns The reading view element.
 */
export function PostReadView({ stored, mine }: { stored: PostDetail, mine: boolean }) {
	const listed = useContentType(stored.type)
	const title = stored.title === '' ? __('(no title)', 'gophenberg') : stored.title
	return (
		<Page
			title={title}
			headingLevel={2}
			breadcrumbs={
				<PageBreadcrumbs label={__('Breadcrumbs', 'gophenberg')}>
					<PageBreadcrumb
						render={
							<Link to="/content/$typeKey" params={{ typeKey: listed.key }} activeOptions={{ exact: true }} />
						}
					>
						{pluralOf(listed)}
					</PageBreadcrumb>
				</PageBreadcrumbs>
			}
		>
			{mine ? (
				<TrashedNotice postId={stored.id} title={title} />
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
