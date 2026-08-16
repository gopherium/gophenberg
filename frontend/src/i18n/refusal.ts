// SPDX-License-Identifier: AGPL-3.0-or-later

import { sprintf } from '@wordpress/i18n'

import { refusalTemplates } from './refusalTemplates'

/** What the server answers when it turns a request away. */
export interface Refused {
	error: string
	code?: string
	meta?: Record<string, unknown>
}

/** What a reader is told when the server said nothing readable at all. */
const unexplained = 'Something went wrong. Try again.'

/** The named places a template asks the refusal to fill in. */
const PLACEHOLDERS = /%\((\w+)\)[sd]/g

/**
 * Reports whether the refusal carries every value the template names.
 * @param template - The message to fill in.
 * @param meta - The data the refusal carries.
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
 * Returns the message a reader is shown for a refusal, in their own language.
 * @param refused - What the server answered.
 * @returns The message to show.
 */
export function refusalText(refused: Refused): string {
	const spoken = refused.error === '' ? unexplained : refused.error
	const template = refused.code === undefined ? undefined : refusalTemplates()[refused.code]
	if (template === undefined || !filled(template, refused.meta ?? {})) {
		return spoken
	}
	return sprintf(template, (refused.meta ?? {}) as never)
}
