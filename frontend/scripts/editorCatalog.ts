// SPDX-License-Identifier: AGPL-3.0-or-later

import { readFileSync } from 'node:fs'
import { join } from 'node:path'

import type { Catalog } from './jed.ts'

/** The Gutenberg release whose packages this repository pins. */
export const EDITOR_RELEASE = '23.7.2'

/** The gettext calls the editor packages make, each without a text domain. */
const CALLS = /\b_?_?n?x?\(\s*(['"`])((?:\\.|(?!\1)[^\\])*)\1/g

/**
 * Returns the export naming the upstream catalogue for a locale.
 * @param locale - The language to read in.
 * @returns The address the catalogue is fetched from.
 */
export function editorCatalogURL(locale: string): string {
	return `https://translate.wordpress.org/projects/wp-plugins/gutenberg/stable/${wordpressLocale(locale)}/default/export-translations/?format=jed1x`
}

/**
 * Returns the locale slug the translation platform names a language by.
 * @param locale - The language as the API names it.
 * @returns The platform's own slug.
 */
export function wordpressLocale(locale: string): string {
	return locale.split('-')[0]
}

/**
 * Returns every source string the pinned editor packages hold.
 * @param root - The repository root the packages sit under.
 * @param packages - The package directories to read.
 * @returns The strings the packages pass to a gettext call.
 */
export function editorStrings(root: string, packages: string[]): Set<string> {
	const found = new Set<string>()
	for (const held of packages) {
		const source = readFileSync(join(root, held), 'utf8')
		for (const match of source.matchAll(CALLS)) {
			found.add(match[2])
		}
	}
	return found
}

/**
 * Returns the message a catalogue key names, dropping any context before it.
 * @param key - The catalogue key.
 * @returns The message alone.
 */
function messageOf(key: string): string {
	const parts = key.split('')
	return parts[parts.length - 1]
}

/**
 * Returns the upstream catalogue keeping only the strings the pinned packages hold.
 * @param upstream - The catalogue as the platform exported it.
 * @param wanted - The strings the pinned packages hold.
 * @returns The filtered catalogue.
 */
export function filterEditorCatalog(upstream: Catalog, wanted: Set<string>): Catalog {
	const kept: Catalog = {}
	for (const [key, value] of Object.entries(upstream)) {
		if (key === '' || wanted.has(messageOf(key))) {
			kept[key] = value
		}
	}
	return kept
}
