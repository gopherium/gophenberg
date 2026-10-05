// SPDX-License-Identifier: Apache-2.0

import { formatNumber } from '@gopherium/gottext'

/** How a number is written, to every digit it holds. */
const EVERY_DIGIT: Intl.NumberFormatOptions = { maximumSignificantDigits: 21 }

/**
 * Returns a number as the site writes it, to every decimal it holds.
 * @param value - The number to write.
 * @returns The written number.
 */
export function everyDecimal(value: number): string {
	return formatNumber(value, EVERY_DIGIT)
}
