// SPDX-License-Identifier: Apache-2.0

import type { Post } from './content.ts'

/** The values a section or one repeater row holds, keyed by sub field key. */
export type HeldValues = Record<string, unknown>

/** One step of the address reaching a value inside a container. */
export type HeldStep = string | number

/**
 * Returns the values a section holds, or nothing when the item carries none under the key.
 * @param post - The item to read.
 * @param key - The section's field key.
 * @returns The values the section holds, or nothing.
 */
export function heldSection(post: Post, key: string): HeldValues | undefined {
	return asSection(post.fields[key])
}

/**
 * Returns the rows a repeater holds, empty when the item carries none under the key.
 * @param post - The item to read.
 * @param key - The repeater's field key.
 * @returns The rows the repeater holds.
 */
export function heldRows(post: Post, key: string): HeldValues[] {
	const held = post.fields[key]
	if (!Array.isArray(held)) {
		return []
	}
	return held.filter((row): row is HeldValues => asSection(row) !== undefined)
}

/**
 * Returns the value the address reaches, or nothing when the item carries none there.
 * @param post - The item to read.
 * @param path - The keys and row numbers addressing the value.
 * @returns The value addressed, or nothing.
 */
export function heldValue(post: Post, path: HeldStep[]): unknown {
	if (path.length === 0) {
		return undefined
	}
	let at: unknown = post.fields
	for (const step of path) {
		at = stepInto(at, step)
		if (at === undefined) {
			return undefined
		}
	}
	return at
}

/**
 * Returns what the step reaches inside the value, or nothing when it reaches none.
 * @param held - The value to step into.
 * @param step - The key or row number to follow.
 * @returns The value reached, or nothing.
 */
function stepInto(held: unknown, step: HeldStep): unknown {
	if (typeof step === 'number') {
		return Array.isArray(held) ? held[step] : undefined
	}
	const inside = asSection(held)
	return inside === undefined ? undefined : inside[step]
}

/**
 * Returns the value as the values a container holds, or nothing when it holds none.
 * @param held - The value to read.
 * @returns The values, or nothing.
 */
function asSection(held: unknown): HeldValues | undefined {
	if (typeof held !== 'object' || held === null || Array.isArray(held)) {
		return undefined
	}
	return held as HeldValues
}
