// SPDX-License-Identifier: AGPL-3.0-or-later

import { __, _n, _nx, _x } from '@wordpress/i18n'

/**
 * Returns a plain label, proving a bare call is extracted.
 * @returns The translated label.
 */
export function plainLabel(): string {
	return __('Zulu label', 'gophenberg')
}

/**
 * Returns a label whose context tells two identical words apart.
 * @returns The translated label.
 */
export function contextLabel(): string {
	return _x('Alpha label', 'dialog title', 'gophenberg')
}

/**
 * Returns a counted label, proving a plural pair is extracted.
 * @param held - How many items are held.
 * @returns The translated label.
 */
export function countedLabel(held: number): string {
	return _n('%d item held', '%d items held', held, 'gophenberg')
}

/**
 * Returns a counted label under a context, proving both ride together.
 * @param held - How many items are held.
 * @returns The translated label.
 */
export function countedContextLabel(held: number): string {
	return _nx('%d draft waits', '%d drafts wait', held, 'sidebar summary', 'gophenberg')
}
