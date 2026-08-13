// SPDX-License-Identifier: Apache-2.0

import { SelectControl } from '@gophenberg/frontend-sdk'
import { useQuery } from '@tanstack/react-query'

import { listPosts } from './api'

/** What the picker offers when the item hangs at the root of its type. */
const rootItem = { label: 'No parent', value: '' }

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
	const items = [
		rootItem,
		...(held.data?.items ?? [])
			.filter((candidate) => candidate.id !== props.postId)
			.map((candidate) => ({ label: candidate.title, value: candidate.id })),
	]
	const selected = items.find((item) => item.value === (props.parentId ?? '')) ?? rootItem
	return (
		<SelectControl
			label="Parent"
			items={items}
			value={selected}
			onValueChange={(item) => props.onChange(item?.value === '' ? null : (item?.value ?? null))}
		/>
	)
}
