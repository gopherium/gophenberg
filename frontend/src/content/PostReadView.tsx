// SPDX-License-Identifier: Apache-2.0

import { Notice, Text } from '@gophenberg/frontend-sdk'
import {
	BlockEditorProvider,
	BlockPreview,
	CANVAS_LAYOUT,
	CANVAS_STYLES,
	parse,
} from '@gophenberg/frontend-sdk/editor'
import type { Block } from '@gophenberg/frontend-sdk/editor'
import { Page, PageBreadcrumb, PageBreadcrumbs } from '@gopherium/godmin'
import { Link } from '@tanstack/react-router'
import { __ } from '@wordpress/i18n'

import type { PostDetail } from './api'
import { TrashedNotice } from './TrashedNotice'
import { pluralOf } from './typeText'
import { useContentType } from './useContentType'

/** The preview viewport width for an unscaled post. */
const AT_CONTAINER_WIDTH = 0

/** The block editor settings that give the preview the styles of the editor canvas. */
const PREVIEW_SETTINGS = { styles: CANVAS_STYLES }

/** The column the preview draws the post in, as wide as the editor draws its content. */
const PREVIEW_COLUMN = { maxWidth: CANVAS_LAYOUT.contentSize }

/** The blocks of the provider around the preview, which draws its own. */
const NO_BLOCKS: Block[] = []

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
			<div style={PREVIEW_COLUMN}>
				<BlockEditorProvider value={NO_BLOCKS} settings={PREVIEW_SETTINGS}>
					<BlockPreview blocks={parse(stored.content)} viewportWidth={AT_CONTAINER_WIDTH} />
				</BlockEditorProvider>
			</div>
		</Page>
	)
}
