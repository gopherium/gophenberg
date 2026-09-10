// SPDX-License-Identifier: Apache-2.0

import type { LinkValue, Post } from './content.ts'

/** One link field of an item and the address it points at. */
export interface LinkField {
	key: string
	link: LinkValue
}

/**
 * Returns the address a value points at, or nothing when it holds no link.
 * @param value - The value an item holds under a field key.
 * @returns The link, or nothing.
 */
export function linkValue(value: unknown): LinkValue | undefined {
	if (typeof value !== 'object' || value === null) {
		return undefined
	}
	const held = value as Record<string, unknown>
	if (Object.keys(held).length !== 3) {
		return undefined
	}
	if (typeof held.url !== 'string' || typeof held.title !== 'string') {
		return undefined
	}
	return typeof held.new_tab === 'boolean' ? (held as unknown as LinkValue) : undefined
}

/**
 * Returns the link fields an item carries, keyed in the order their keys read.
 * @param post - The item to read.
 * @returns Each link field and the address it points at.
 */
export function linkFields(post: Post): LinkField[] {
	const held: LinkField[] = []
	for (const key of Object.keys(post.fields).sort()) {
		const link = linkValue(post.fields[key])
		if (link !== undefined && link.url !== '') {
			held.push({ key, link })
		}
	}
	return held
}
