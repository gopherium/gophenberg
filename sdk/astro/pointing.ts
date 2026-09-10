// SPDX-License-Identifier: Apache-2.0

import type { Pointer, Post } from './content.ts'

/** One linked from field of an item and the items pointing at it. */
export interface PointingField {
	key: string
	items: Pointer[]
}

/**
 * Returns the items pointing this way, or nothing when the value names none.
 * @param value - The value an item holds under a field key.
 * @returns The items pointing here, or nothing.
 */
export function pointingItems(value: unknown): Pointer[] | undefined {
	if (!Array.isArray(value)) {
		return undefined
	}
	return value.every(isPointer) ? value : undefined
}

/**
 * Reports whether a listed value names one item pointing this way.
 * @param held - One entry of a field value.
 * @returns True when the entry names a pointing item.
 */
function isPointer(held: unknown): held is Pointer {
	if (typeof held !== 'object' || held === null) {
		return false
	}
	const named = held as Record<string, unknown>
	return (
		Object.keys(named).length === 4 &&
		typeof named.id === 'string' &&
		typeof named.title === 'string' &&
		typeof named.path === 'string' &&
		typeof named.type === 'string'
	)
}

/**
 * Returns the linked from fields an item carries, keyed in the order their keys read.
 * @param post - The item to read.
 * @returns Each linked from field and what points at the item through it.
 */
export function pointingFields(post: Post): PointingField[] {
	const held: PointingField[] = []
	for (const key of Object.keys(post.fields).sort()) {
		const items = pointingItems(post.fields[key])
		if (items !== undefined && items.length > 0) {
			held.push({ key, items })
		}
	}
	return held
}
