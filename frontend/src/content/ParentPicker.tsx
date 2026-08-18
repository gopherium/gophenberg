// SPDX-License-Identifier: Apache-2.0

import { SelectControl } from '@gophenberg/frontend-sdk'
import { __ } from '@wordpress/i18n'
import { useQuery } from '@tanstack/react-query'

import { listPosts } from './api'

/**
 * Returns the parent a select change asks for, or nothing at the root.
 * @param item - The item the select reported, or nothing.
 * @returns The parent to file under, or null at the root.
 */
export function chosenParent(item: { value: string | null } | null): string | null {
	if (item === null || item.value === null || item.value === '') {
		return null
	}
	return item.value
}

/**
 * Renders the control filing an item under another of its type.
 * @param props - The item being edited, its type, and what to do with the choice.
 * @returns The picker element.
 */
export function ParentPicker(props: {
	postId: string
	type: string
	parentId: string | null
	onChange: (parentId: string | null) => void
}) {
	const held = useQuery({
		queryKey: ['parents', props.type],
		queryFn: () => listPosts({ type: props.type }),
	})
	const rootItem = { label: __('No parent', 'gophenberg'), value: '' }
	const items = [
		rootItem,
		...(held.data?.items ?? [])
			.filter((candidate) => candidate.id !== props.postId)
			.map((candidate) => ({ label: candidate.title, value: candidate.id })),
	]
	const selected = items.find((item) => item.value === (props.parentId ?? '')) ?? rootItem
	return (
		<SelectControl
			label={__('Parent', 'gophenberg')}
			items={items}
			value={selected}
			onValueChange={(item) => props.onChange(chosenParent(item))}
		/>
	)
}
