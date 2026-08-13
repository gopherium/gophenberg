// SPDX-License-Identifier: Apache-2.0

import { Badge, Stack, Text } from '@gophenberg/frontend-sdk'
import type { Field } from '@gophenberg/frontend-sdk/dataviews'
import { Link } from '@tanstack/react-router'

import type { Post } from './api'

/**
 * Returns the label shown beside the title of a post that is not published.
 * @param status - The status the post holds.
 * @returns The label, or nothing when the status needs no marking.
 */
function stateLabel(status: string): string | null {
	switch (status) {
		case 'draft':
			return 'Draft'
		case 'pending':
			return 'Pending'
		case 'private':
			return 'Private'
		case 'scheduled':
			return 'Scheduled'
		case 'trash':
			return 'Trash'
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
		? { caption: 'Published', at: post.publishedAt }
		: { caption: 'Last Modified', at: post.updatedAt }
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
				{item.title === '' ? '(no title)' : item.title}
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
			{caption} {at === '' ? '' : new Date(at).toLocaleDateString()}
		</Text>
	)
}

export const postFields: Field<Post>[] = [
	{ id: 'title', label: 'Title', render: TitleCell, enableSorting: true, enableHiding: false },
	{ id: 'author', label: 'Author', render: AuthorCell, enableSorting: false },
	{ id: 'date', label: 'Date', render: DateCell, enableSorting: true },
]
