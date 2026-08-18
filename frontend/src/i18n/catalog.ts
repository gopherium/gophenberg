// SPDX-License-Identifier: Apache-2.0

/** The language the sources are written in, which ships no catalogue. */
export const DEFAULT_LOCALE = 'en-US'

/** A compiled catalogue, keyed by message. */
export type Catalog = Record<string, string[] | Record<string, string>>

/** The lazy chunks a build produced, keyed by path. */
export type Chunks = Record<string, () => Promise<{ default: Catalog }>>

/** The catalogues built from the committed sources. */
const own = import.meta.glob<{ default: Catalog }>('../languages/*.json')

/** The catalogues a build fetched from upstream, absent until it runs. */
const editor = import.meta.glob<{ default: Catalog }>('../languages/editor/*.json')

/**
 * Returns the catalogue a locale ships under the path, or nothing when it ships none.
 * @param chunks - The lazy chunks a build produced.
 * @param path - Where the locale's catalogue would sit.
 * @param locale - The language to read in.
 * @returns The catalogue, or nothing.
 */
export async function catalogAt(
	chunks: Chunks,
	path: string,
	locale: string,
): Promise<Catalog | undefined> {
	const load = chunks[path]
	if (locale === DEFAULT_LOCALE || load === undefined) {
		return undefined
	}
	return (await load()).default
}

/**
 * Returns the catalogue of Gophenberg's own strings for a locale.
 * @param locale - The language to read in.
 * @returns The catalogue, or nothing.
 */
export async function catalogFor(locale: string): Promise<Catalog | undefined> {
	return catalogAt(own, `../languages/${locale}.json`, locale)
}

/**
 * Returns the catalogue the editor packages read for a locale.
 * @param locale - The language to read in.
 * @returns The catalogue, or nothing.
 */
export async function editorCatalogFor(locale: string): Promise<Catalog | undefined> {
	return catalogAt(editor, `../languages/editor/${locale}.json`, locale)
}
