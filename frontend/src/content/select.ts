// SPDX-License-Identifier: Apache-2.0

/** One choice a select offers. */
export interface Choice {
	label: string
	value: string
}

/**
 * Returns the choice a select change asks for.
 * @param item - The item the select reported, or nothing.
 * @param offered - The choices the select holds.
 * @param current - The choice held.
 * @returns The choice to hold.
 */
export function chosenOf(
	item: { value: string | null } | null,
	offered: Choice[],
	current: Choice,
): Choice {
	if (item === null || item.value === null) {
		return current
	}
	return offered.find((choice) => choice.value === item.value) ?? current
}
