// SPDX-License-Identifier: AGPL-3.0-or-later

import { DEFAULT_LOCALE } from './catalog'

let settled: string = DEFAULT_LOCALE

/**
 * Remembers the language dates and numbers are shown in.
 * @param locale - The language the reader settled on.
 */
export function rememberLocale(locale: string): void {
	settled = locale
}

/**
 * Returns the language dates and numbers are shown in.
 * @returns The language the reader settled on.
 */
export function displayLocale(): string {
	return settled
}

/**
 * Returns a stored timestamp as a date in the reader's language.
 * @param at - The timestamp as the server stored it.
 * @returns The date to show, empty when there is no timestamp.
 */
export function formatDate(at: string): string {
	return at === '' ? '' : new Date(at).toLocaleDateString(settled)
}
