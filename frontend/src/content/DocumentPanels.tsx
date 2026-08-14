// SPDX-License-Identifier: Apache-2.0

import { InputControl, SelectControl, Stack, TextareaControl } from '@gophenberg/frontend-sdk'
import { useMemo } from 'react'

import { ParentPicker } from './ParentPicker'
import { TrashPost } from './TrashPost'
import { useContentType } from './useContentType'
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

interface StatusItem {
	label: string
	value: string
}

/**
 * Returns the statuses the select offers and the item holding the current one.
 * @param status - The status the post holds.
 * @returns The items to offer and the selected item, which is one of them.
 */
function statusItems(status: string): { items: StatusItem[], selected: StatusItem } {
	const authored = AUTHORED_STATUSES.find((item) => item.value === status)
	if (authored !== undefined) {
		return { items: AUTHORED_STATUSES, selected: authored }
	}
	const held = { label: LABELS[status] ?? status, value: status }
	return { items: [...AUTHORED_STATUSES, held], selected: held }
}

/**
 * Returns the status a select change asks for.
 * @param item - The item the select reported, or nothing.
 * @param current - The status the post holds.
 * @returns The status to hold.
 */
export function chosenStatus(item: { value: string | null } | null, current: string): string {
	if (item === null || item.value === null) {
		return current
	}
	return item.value
}

/**
 * Renders the summary panels of the post being edited.
 * @param props - The buffer the panels drive.
 * @returns The panels element.
 */
export function DocumentPanels({ postId, buffer }: { postId: string, buffer: EditorBuffer }) {
	const { items, selected } = useMemo(() => statusItems(buffer.status), [buffer.status])
	const listed = useContentType()
	return (
		<Stack direction="column" gap="md">
			<SelectControl
				label="Status"
				items={items}
				value={selected}
				onValueChange={(item) => buffer.setStatus(chosenStatus(item, buffer.status))}
			/>
			<InputControl label="Slug" value={buffer.slug} onValueChange={buffer.setSlug} />
			{listed.hierarchical && (
				<ParentPicker
					postId={postId}
					type={listed.key}
					parentId={buffer.parentId}
					onChange={buffer.setParentId}
				/>
			)}
			<TextareaControl
				label="Excerpt"
				value={buffer.excerpt}
				onValueChange={buffer.setExcerpt}
			/>
			<TrashPost postId={postId} title={buffer.title} />
		</Stack>
	)
}
