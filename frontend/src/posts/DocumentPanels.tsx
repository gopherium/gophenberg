// SPDX-License-Identifier: Apache-2.0

import { InputControl, SelectControl, Stack, TextareaControl } from '@gophenberg/frontend-sdk'

import { TrashPost } from './TrashPost'
import type { EditorBuffer } from './useEditorBuffer'

const AUTHORED_STATUSES = [
	{ label: 'Draft', value: 'draft' },
	{ label: 'Pending', value: 'pending' },
	{ label: 'Private', value: 'private' },
]

const LABELS: Record<string, string> = {
	draft: 'Draft',
	pending: 'Pending',
	private: 'Private',
	published: 'Published',
	trash: 'Trash',
}

/**
 * Returns the statuses the select offers for a post.
 * @param status - The status the post holds.
 * @returns The statuses to offer.
 */
function statusesFor(status: string): { label: string, value: string }[] {
	if (AUTHORED_STATUSES.some((item) => item.value === status)) {
		return AUTHORED_STATUSES
	}
	return [...AUTHORED_STATUSES, itemFor(status)]
}

/**
 * Returns the status item shown for the given value.
 * @param status - The status the post holds.
 * @returns The item naming that status.
 */
function itemFor(status: string): { label: string, value: string } {
	return { label: LABELS[status] ?? status, value: status }
}

/**
 * Renders the summary panels of the post being edited.
 * @param props - The buffer the panels drive.
 * @returns The panels element.
 */
export function DocumentPanels({ postId, buffer }: { postId: string, buffer: EditorBuffer }) {
	return (
		<Stack direction="column" gap="md">
			<SelectControl
				label="Status"
				items={statusesFor(buffer.status)}
				value={itemFor(buffer.status)}
				onValueChange={(item) => buffer.setStatus((item as { value: string }).value)}
			/>
			<InputControl label="Slug" value={buffer.slug} onValueChange={buffer.setSlug} />
			<TextareaControl
				label="Excerpt"
				value={buffer.excerpt}
				onValueChange={buffer.setExcerpt}
			/>
			<TrashPost postId={postId} title={buffer.title} />
		</Stack>
	)
}
