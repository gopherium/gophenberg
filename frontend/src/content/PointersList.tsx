// SPDX-License-Identifier: Apache-2.0

import { Stack, Text } from '@gophenberg/frontend-sdk'
import { Link } from '@tanstack/react-router'
import { __, sprintf } from '@wordpress/i18n'

import { FieldLabel } from './FieldLabel'
import type { ContentField } from './types'

/** One item pointing at the one being edited. */
export interface Pointer {
	id: string
	title: string
	type: string
}

/**
 * Returns the pointing item a value holds, or nothing when the value is not one.
 * @param held - One entry the field holds.
 * @returns The pointer, or undefined.
 */
function pointerOf(held: unknown): Pointer | undefined {
	if (typeof held !== 'object' || held === null) {
		return undefined
	}
	const row = held as Record<string, unknown>
	if (typeof row.id !== 'string' || typeof row.type !== 'string') {
		return undefined
	}
	return { id: row.id, title: typeof row.title === 'string' ? row.title : row.id, type: row.type }
}

/**
 * Returns the items a backlinks field holds under its key.
 * @param value - The value the item holds.
 * @returns The pointers, none for anything else.
 */
export function pointersHeld(value: unknown): Pointer[] {
	if (!Array.isArray(value)) {
		return []
	}
	return value.map(pointerOf).filter((held): held is Pointer => held !== undefined)
}

/**
 * Renders the items pointing at the one being edited, each linking to its own editor.
 * @param props - The field reading them, the pointers held, and how many point in all.
 * @returns The list element.
 */
export function PointersList(props: { field: ContentField; pointers: Pointer[]; total: number }) {
	return (
		<Stack direction="column" gap="xs">
			<FieldLabel field={props.field} />
			{props.pointers.length === 0 ? (
				<Text variant="body-sm">{__('Nothing points here yet.', 'gophenberg')}</Text>
			) : (
				<ul className="gophenberg-editor__pointers" aria-label={props.field.label}>
					{props.pointers.map((pointer) => (
						<li key={pointer.id}>
							<Link
								to="/content/$typeKey/$postId/edit"
								params={{ typeKey: pointer.type, postId: pointer.id }}
							>
								{pointer.title}
							</Link>
						</li>
					))}
				</ul>
			)}
			{props.total > props.pointers.length && (
				<Text variant="body-sm">
					{sprintf(__('Showing %(held)d of %(total)d.', 'gophenberg'), {
						held: props.pointers.length,
						total: props.total,
					} as never)}
				</Text>
			)}
		</Stack>
	)
}
