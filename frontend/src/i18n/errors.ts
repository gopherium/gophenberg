// SPDX-License-Identifier: Apache-2.0

import { errorText as templateText } from '@gopherium/gottext'
import { __ } from '@wordpress/i18n'

import { authTemplates, errorTemplates, serverTemplates, typeTemplates, updateTemplates } from './errorTemplates'
import { everyDecimal } from './numbers'

/** What the server answers when it turns a request away. */
export interface Refused {
	error: string
	code?: string
	meta?: Record<string, unknown>
}

/**
 * Returns what a reader is told when the server said nothing readable at all.
 * @returns The message to show.
 */
function unexplained(): string {
	return __('Something went wrong. Try again.', 'gophenberg')
}

/**
 * Returns the message each code the server answers with stands for, every table read fresh.
 * @returns The messages, keyed by code.
 */
export function codeTemplates(): Record<string, string> {
	return { ...errorTemplates(), ...serverTemplates(), ...authTemplates(), ...typeTemplates(), ...updateTemplates() }
}

/**
 * Returns the data an error carries, every number written in the format locale.
 * @param meta - The data the error carries.
 * @returns The data a template is filled from.
 */
function writtenMeta(meta: Record<string, unknown>): Record<string, unknown> {
	return Object.fromEntries(
		Object.entries(meta).map(([name, value]) => [name, typeof value === 'number' ? everyDecimal(value) : value]),
	)
}

/**
 * Returns the message the code of a rejected request stands for in the reader's language, or the fallback.
 * @param rejected - What the server answered.
 * @param fallback - The words to show when no template the answer can fill stands for its code.
 * @returns The message to show.
 */
export function codeText(rejected: Pick<Refused, 'code' | 'meta'>, fallback: string): string {
	const named = { message: '', code: rejected.code, meta: writtenMeta(rejected.meta ?? {}) }
	return templateText(named, codeTemplates(), fallback)
}

/**
 * Returns the message a reader is shown for a refused request, in their own language.
 * @param refused - What the server answered.
 * @returns The message to show.
 */
export function errorText(refused: Refused): string {
	return codeText(refused, refused.error === '' ? unexplained() : refused.error)
}
