// SPDX-License-Identifier: Apache-2.0

/** The values of a type's declared fields, keyed by field key. */
export type FieldValues = Record<string, unknown>

/**
 * Reports whether two sets of field values hold the same things.
 * @param held - The values one side holds.
 * @param other - The values the other side holds.
 * @returns True when both sides declare the same keys and every key holds an equal value.
 */
export function sameFieldValues(held: FieldValues, other: FieldValues): boolean {
	const keys = Object.keys(held)
	if (keys.length !== Object.keys(other).length) {
		return false
	}
	return keys.every((key) => sameValue(held[key], other[key]))
}

/**
 * Returns the field values that moved away from the ones the server last reported.
 * @param held - The values the buffer holds.
 * @param settled - The values the server last reported.
 * @returns Only the keys whose value changed.
 */
export function changedFieldValues(held: FieldValues, settled: FieldValues): FieldValues {
	return Object.fromEntries(
		Object.entries(held).filter(([key, value]) => !sameValue(value, settled[key])),
	)
}

/**
 * Reports whether two field values stand for the same thing.
 * @param held - The value one side holds.
 * @param other - The value the other side holds.
 * @returns True when the values match, comparing relation lists by their ordered targets.
 */
function sameValue(held: unknown, other: unknown): boolean {
	if (Array.isArray(held) || Array.isArray(other)) {
		return (
			Array.isArray(held) &&
			Array.isArray(other) &&
			held.length === other.length &&
			held.every((target, i) => target === other[i])
		)
	}
	if (held === null || held === undefined) {
		return other === null || other === undefined
	}
	return held === other
}
