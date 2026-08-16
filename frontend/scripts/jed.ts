// SPDX-License-Identifier: AGPL-3.0-or-later

import { po } from 'gettext-parser'

/** The metadata entry the runtime reads its plural rule from. */
const METADATA = ''

/** A compiled catalogue, keyed by message with the metadata entry first. */
export type Catalog = Record<string, string[] | Record<string, string>>

/**
 * Returns the key the runtime looks a message up under.
 * @param context - The context telling two senses of one word apart, if any.
 * @param msgid - The source message.
 * @returns The lookup key.
 */
function keyOf(context: string, msgid: string): string {
	return context === '' ? msgid : `${context}${msgid}`
}

/**
 * Returns the catalogue a PO source compiles to, ready for setLocaleData.
 * @param source - The catalogue as PO text.
 * @returns The compiled catalogue.
 */
export function compileCatalog(source: string): Catalog {
	const parsed = po.parse(source)
	const entries: [string, string[]][] = []
	for (const [context, held] of Object.entries(parsed.translations)) {
		for (const [msgid, entry] of Object.entries(held)) {
			if (msgid === METADATA) {
				continue
			}
			entries.push([keyOf(context, msgid), entry.msgstr])
		}
	}
	entries.sort(([left], [right]) => Number(left > right) - Number(left < right))
	const compiled = new Map<string, string[] | Record<string, string>>()
	compiled.set(METADATA, {
		lang: parsed.headers.Language ?? '',
		'plural-forms': parsed.headers['Plural-Forms'] ?? '',
	})
	for (const [key, msgstr] of entries) {
		compiled.set(key, msgstr)
	}
	return Object.fromEntries(compiled) as Catalog
}

/**
 * Returns the catalogue as the bytes a locale chunk ships, metadata entry first.
 * @param catalog - The compiled catalogue.
 * @returns The catalogue as JSON text.
 */
export function serializeCatalog(catalog: Catalog): string {
	const keys = Object.keys(catalog).filter((held) => held !== METADATA)
	keys.sort((left, right) => Number(left > right) - Number(left < right))
	const lines = [JSON.stringify(METADATA) + ':' + JSON.stringify(catalog[METADATA])]
	for (const key of keys) {
		lines.push(JSON.stringify(key) + ':' + JSON.stringify(catalog[key]))
	}
	return '{' + lines.join(',') + '}\n'
}
