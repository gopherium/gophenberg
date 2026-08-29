// SPDX-License-Identifier: Apache-2.0

import type { ContentField } from './types'

/** The value a control starts a fresh item with. */
export type SeededValue = string | number | boolean | string[]

/** The shape a kind's default has to hold, the kinds taking none left out. */
const DEFAULT_SHAPES: Record<string, 'string' | 'number' | 'boolean'> = {
	text: 'string',
	number: 'number',
	boolean: 'boolean',
}

/**
 * Returns the default a field seeds a fresh item with, or nothing when it seeds none.
 * @param field - The declared field to read.
 * @returns The value to seed, or undefined.
 */
function seeded(field: ContentField): SeededValue | undefined {
	if (field.kind === 'choice') {
		return seededChoice(field)
	}
	const wanted = DEFAULT_SHAPES[field.kind]
	const held = field.settings.default
	if (wanted === undefined || held === undefined || typeof held !== wanted) {
		return undefined
	}
	return held as SeededValue
}

/**
 * Returns the default a choice field seeds, shaped by whether it holds many.
 * @param field - The declared choice field to read.
 * @returns The value to seed, or undefined.
 */
function seededChoice(field: ContentField): SeededValue | undefined {
	const held = field.settings.default
	if (field.settings.multiple === true) {
		const listed = Array.isArray(held) ? held.filter((member) => typeof member === 'string') : []
		return Array.isArray(held) && listed.length === held.length ? listed : undefined
	}
	return typeof held === 'string' ? held : undefined
}

/**
 * Returns the values a fresh item starts with, the defaults its fields name.
 * @param declared - The fields the type declares.
 * @returns The values to store as the item is created.
 */
export function seededValues(declared: ContentField[]): Record<string, SeededValue> {
	const values: Record<string, SeededValue> = {}
	for (const field of declared) {
		const held = seeded(field)
		if (held !== undefined) {
			values[field.key] = held
		}
	}
	return values
}
