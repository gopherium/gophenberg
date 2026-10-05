// SPDX-License-Identifier: Apache-2.0

import { __, sprintf } from '@wordpress/i18n'

import { errorTemplates } from './errorTemplates'
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

/** The named places a template asks the error to fill in. */
const PLACEHOLDERS = /%\((\w+)\)[sd]/g

/**
 * Reports whether the error carries every value the template names.
 * @param template - The message to fill in.
 * @param meta - The data the error carries.
 * @returns True when nothing the template asks for is missing.
 */
function filled(template: string, meta: Record<string, unknown>): boolean {
	for (const match of template.matchAll(PLACEHOLDERS)) {
		if (meta[match[1]] === undefined) {
			return false
		}
	}
	return true
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
 * Returns the message a reader is shown for a refused request, in their own language.
 * @param refused - What the server answered.
 * @returns The message to show.
 */
export function errorText(refused: Refused): string {
	const spoken = refused.error === '' ? unexplained() : refused.error
	const template = refused.code === undefined ? undefined : errorTemplates()[refused.code]
	if (template === undefined || !filled(template, refused.meta ?? {})) {
		return spoken
	}
	return sprintf(template, writtenMeta(refused.meta ?? {}) as never)
}
