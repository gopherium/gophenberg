// SPDX-License-Identifier: Apache-2.0

import { Button, SelectControl, Stack, Text } from '@gophenberg/frontend-sdk'
import { __, sprintf } from '@wordpress/i18n'
import { useQuery } from '@tanstack/react-query'

import { listEveryPost } from './api'
import { FieldLabel } from './FieldLabel'
import type { ContentField } from './types'

/** The status an item carries while it waits in the trash. */
const trashed = 'trash'

/**
 * Returns the identities a stored relation value holds.
 * @param value - The value the buffer holds under the field key.
 * @returns The targets, or none when the field holds nothing.
 */
export function targetsHeld(value: unknown): string[] {
	if (!Array.isArray(value)) {
		return []
	}
	return value.filter((held): held is string => typeof held === 'string')
}

/**
 * Returns the targets after the choice a single target select reported.
 * @param item - The item the select reported, or nothing.
 * @returns The targets to hold.
 */
export function chosenTarget(item: { value: string | null } | null): string[] {
	if (item === null || item.value === null || item.value === '') {
		return []
	}
	return [item.value]
}

/**
 * Renders the control pointing a relation field at the content it names.
 * @param props - The field, the item being edited, the targets held, and what to do with a choice.
 * @returns The picker element.
 */
export function RelationPicker(props: {
	field: ContentField
	postId: string
	targets: string[]
	onChange: (targets: string[]) => void
}) {
	const held = useQuery({
		queryKey: ['relation-targets', props.field.relatesTo],
		queryFn: () => listEveryPost({ type: props.field.relatesTo }),
	})
	const candidates = (held.data ?? []).filter(
		(item) => item.id !== props.postId && item.status !== trashed,
	)
	if (props.field.many) {
		return <ManyTargets {...props} candidates={candidates} />
	}
	const noTarget = { label: __('None', 'gophenberg'), value: '' }
	const items = [noTarget, ...candidates.map((item) => ({ label: item.title, value: item.id }))]
	const selected = items.find((item) => item.value === (props.targets[0] ?? '')) ?? noTarget
	return (
		<SelectControl
			label={props.field.label}
			items={items}
			value={selected}
			onValueChange={(item) => props.onChange(chosenTarget(item))}
		/>
	)
}

/** One item a relation field may point at. */
interface Candidate {
	id: string
	title: string
}

/**
 * Renders the control holding many targets, with what is held above what may be added.
 * @param props - The field, the targets held, the candidates, and what to do with a change.
 * @returns The control element.
 */
function ManyTargets(props: {
	field: ContentField
	targets: string[]
	candidates: Candidate[]
	onChange: (targets: string[]) => void
}) {
	const named = props.targets.map((id) => ({
		id,
		title: props.candidates.find((candidate) => candidate.id === id)?.title ?? id,
	}))
	const open = props.candidates.filter((candidate) => !props.targets.includes(candidate.id))
	const addLabel = sprintf(__('Add %(type)s', 'gophenberg'), { type: props.field.label })
	const items = [
		{ label: addLabel, value: '' },
		...open.map((candidate) => ({ label: candidate.title, value: candidate.id })),
	]
	return (
		<Stack direction="column" gap="xs">
			<FieldLabel field={props.field} />
			<ul className="gophenberg-editor__targets" aria-label={props.field.label}>
				{named.map((target) => (
					<li key={target.id}>
						<Text>{target.title}</Text>
						<Button
							variant="outline"
							onClick={() => props.onChange(props.targets.filter((id) => id !== target.id))}
						>
							{sprintf(__('Remove %(title)s', 'gophenberg'), { title: target.title })}
						</Button>
					</li>
				))}
			</ul>
			<SelectControl
				label={addLabel}
				items={items}
				value={items[0]}
				onValueChange={(item) => {
					const chosen = chosenTarget(item)
					if (chosen.length > 0) {
						props.onChange([...props.targets, chosen[0]])
					}
				}}
			/>
		</Stack>
	)
}
