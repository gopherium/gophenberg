// SPDX-License-Identifier: Apache-2.0

import { InputControl, SelectControl, Stack, TextareaControl } from '@gophenberg/frontend-sdk'
import { __, _x } from '@wordpress/i18n'
import { useMemo } from 'react'

import { FieldsPanel } from './FieldsPanel'
import { ParentPicker } from './ParentPicker'
import { TrashPost } from './TrashPost'
import { useContentType } from './useContentType'
import type { EditorBuffer } from './useEditorBuffer'

interface StatusItem {
	label: string
	value: string
}

/**
 * Returns the statuses an author can put a post into.
 * @returns The items the select offers.
 */
function authoredStatuses(): StatusItem[] {
	return [
		{ label: _x('Draft', 'post status', 'gophenberg'), value: 'draft' },
		{ label: __('Pending', 'gophenberg'), value: 'pending' },
		{ label: __('Private', 'gophenberg'), value: 'private' },
	]
}

/**
 * Returns the label each status is shown under.
 * @returns The label of every status, keyed by the slug the server sends.
 */
function statusLabels(): Record<string, string> {
	return {
		draft: _x('Draft', 'post status', 'gophenberg'),
		pending: __('Pending', 'gophenberg'),
		private: __('Private', 'gophenberg'),
		published: _x('Published', 'post status', 'gophenberg'),
		trash: _x('Trash', 'post status', 'gophenberg'),
	}
}

/**
 * Returns the statuses the select offers and the item holding the current one.
 * @param status - The status the post holds.
 * @returns The items to offer and the selected item, which is one of them.
 */
function statusItems(status: string): { items: StatusItem[], selected: StatusItem } {
	const offered = authoredStatuses()
	const authored = offered.find((item) => item.value === status)
	if (authored !== undefined) {
		return { items: offered, selected: authored }
	}
	const held = { label: statusLabels()[status] ?? status, value: status }
	return { items: [...offered, held], selected: held }
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
				label={_x('Status', 'post', 'gophenberg')}
				items={items}
				value={selected}
				onValueChange={(item) => buffer.setStatus(chosenStatus(item, buffer.status))}
			/>
			<InputControl label={__('Slug', 'gophenberg')} value={buffer.slug} onValueChange={buffer.setSlug} />
			{listed.hierarchical && (
				<ParentPicker
					postId={postId}
					type={listed.key}
					parentId={buffer.parentId}
					onChange={buffer.setParentId}
				/>
			)}
			<TextareaControl
				label={__('Excerpt', 'gophenberg')}
				value={buffer.excerpt}
				onValueChange={buffer.setExcerpt}
			/>
			<FieldsPanel postId={postId} declared={listed.fields} buffer={buffer} />
			<TrashPost postId={postId} title={buffer.title} />
		</Stack>
	)
}
