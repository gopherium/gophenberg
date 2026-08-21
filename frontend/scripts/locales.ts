// SPDX-License-Identifier: Apache-2.0

import { readFileSync } from 'node:fs'
import { join } from 'node:path'

/** Where the application declares the languages it answers in. */
const LOCALE_SOURCE = ['internal', 'content', 'locale.go']

/**
 * Returns the languages the application answers in, as it declares them.
 * @param root - The repository root the declaration sits under.
 * @returns The languages, the fallback first.
 */
export function supportedLocales(root: string): string[] {
	const source = readFileSync(join(root, ...LOCALE_SOURCE), 'utf8')
	const declared = /DefaultLocale\s*=\s*"([^"]+)"/.exec(source)
	const listed = /SupportedLocales\s*=\s*\[\]string\{([^}]*)\}/.exec(source)
	if (declared === null || listed === null) {
		throw new Error('the application declares no supported languages')
	}
	return listed[1]
		.split(',')
		.map((held) => held.trim())
		.filter((held) => held !== '')
		.map((held) => (held === 'DefaultLocale' ? declared[1] : held.replace(/"/g, '')))
}
