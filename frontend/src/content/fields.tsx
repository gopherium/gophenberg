// SPDX-License-Identifier: Apache-2.0

import { Badge, Stack, Text } from '@gophenberg/frontend-sdk'
import type { Field } from '@gophenberg/frontend-sdk/dataviews'
import { __, _x } from '@wordpress/i18n'
import { Link } from '@tanstack/react-router'

import { formatDate } from '@gopherium/gottext'
import type { Post } from './api'

/**
 * Returns the label shown beside the title of a post that is not published.
 * @param status - The status the post holds.
 * @returns The label, or nothing when the status needs no marking.
 */
function stateLabel(status: string): string | null {
	switch (status) {
		case 'draft':
			return _x('Draft', 'post status', 'gophenberg')
		case 'pending':
			return __('Pending', 'gophenberg')
		case 'private':
			return __('Private', 'gophenberg')
		case 'scheduled':
			return __('Scheduled', 'gophenberg')
		case 'trash':
			return _x('Trash', 'post status', 'gophenberg')
		default:
			return null
	}
}

/**
 * Returns the date a listing shows for a post, and what that date means.
 * @param post - The post to describe.
 * @returns The caption and the date to show.
 */
function dateLine(post: Post): { caption: string, at: string } {
	return post.publishedAt !== null
		? { caption: _x('Published', 'date caption', 'gophenberg'), at: post.publishedAt }
		: { caption: __('Last Modified', 'gophenberg'), at: post.updatedAt }
}

/**
 * Renders the title of a post as a link to its editor.
 * @param props - The row being rendered.
 * @returns The title cell.
 */
function TitleCell({ item }: { item: Post }) {
	const state = stateLabel(item.status)
	return (
		<Stack direction="row" gap="xs" align="center">
			<Link to="/content/$typeKey/$postId/edit" params={{ typeKey: item.type, postId: item.id }}>
				{item.title === '' ? __('(no title)', 'gophenberg') : item.title}
			</Link>
			{state !== null && <Badge>{state}</Badge>}
		</Stack>
	)
}

/**
 * Renders the author of a post.
 * @param props - The row being rendered.
 * @returns The author cell.
 */
function AuthorCell({ item }: { item: Post }) {
	return <Text>{item.authorName}</Text>
}

/**
 * Renders the date of a post with the caption explaining it.
 * @param props - The row being rendered.
 * @returns The date cell.
 */
function DateCell({ item }: { item: Post }) {
	const { caption, at } = dateLine(item)
	return (
		<Text>
			{caption} {formatDate(at)}
		</Text>
	)
}

export const postFields: Field<Post>[] = [
	{
		id: 'title',
		label: __('Title', 'gophenberg'),
		render: TitleCell,
		enableSorting: true,
		enableHiding: false,
	},
	{ id: 'author', label: __('Author', 'gophenberg'), render: AuthorCell, enableSorting: false },
	{ id: 'date', label: _x('Date', 'column', 'gophenberg'), render: DateCell, enableSorting: true },
]
