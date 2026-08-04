// SPDX-License-Identifier: Apache-2.0

/** The shape of a numbered segment the renderer parses. */
const numberedSegment = /^[+-]?[0-9]+$/

/** The lowest page value the renderer parses. */
const minPage = -9223372036854775808n

/** The highest page value the renderer parses. */
const maxPage = 9223372036854775807n

/**
 * Reports whether a value sits inside the range the renderer parses.
 * @param value - The parsed segment value.
 * @returns True when the renderer would parse it too.
 */
function withinRange(value: bigint): boolean {
	return value >= minPage && value <= maxPage
}

/**
 * Returns the page a listing segment addresses, or nothing when it addresses none.
 * @param raw - The page segment of the address, absent on the front page.
 * @returns The page to list, or undefined when the segment is not a page.
 */
export function pageNumber(raw: string | undefined): number | undefined {
	if (raw === undefined) {
		return 1
	}
	if (!numberedSegment.test(raw)) {
		return undefined
	}
	const value = BigInt(raw)
	if (!withinRange(value)) {
		return undefined
	}
	return value < 1n ? 1 : Number(value)
}
