// SPDX-License-Identifier: Apache-2.0

import { formatNumber } from '@gopherium/gottext'

/** How a number is written, to every decimal it holds. */
const EVERY_DECIMAL: Intl.NumberFormatOptions = { maximumFractionDigits: 20 }

/**
 * Returns a number as the site writes it, to every decimal it holds.
 * @param value - The number to write.
 * @returns The written number.
 */
export function everyDecimal(value: number): string {
	return formatNumber(value, EVERY_DECIMAL)
}
