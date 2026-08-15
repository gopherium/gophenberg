// SPDX-License-Identifier: Apache-2.0

import type { Post, RelatedItem } from './content.ts'

/** One relation field of an item and the content it points at. */
export interface RelatedField {
	key: string
	items: RelatedItem[]
}

/**
 * Returns the items a value names, or nothing when it names no content.
 * @param value - The value an item holds under a field key.
 * @returns The items named, or nothing.
 */
export function relatedItems(value: unknown): RelatedItem[] | undefined {
	if (!Array.isArray(value)) {
		return undefined
	}
	if (value.every(isRelatedItem)) {
		return value
	}
	return undefined
}

/**
 * Reports whether a listed value names one item.
 * @param held - One entry of a field value.
 * @returns True when the entry names an item.
 */
function isRelatedItem(held: unknown): held is RelatedItem {
	if (typeof held !== 'object' || held === null) {
		return false
	}
	const named = held as Record<string, unknown>
	return (
		typeof named.id === 'string' &&
		typeof named.title === 'string' &&
		typeof named.path === 'string'
	)
}

/**
 * Returns the relation fields an item carries, keyed in the order their keys read.
 * @param post - The item to read.
 * @returns Each relation field and what it points at.
 */
export function relatedFields(post: Post): RelatedField[] {
	const held: RelatedField[] = []
	for (const key of Object.keys(post.fields).sort()) {
		const items = relatedItems(post.fields[key])
		if (items !== undefined && items.length > 0) {
			held.push({ key, items })
		}
	}
	return held
}
