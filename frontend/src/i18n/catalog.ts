// SPDX-License-Identifier: AGPL-3.0-or-later

/** The language the sources are written in, which ships no catalogue. */
export const DEFAULT_LOCALE = 'en-US'

/** A compiled catalogue, keyed by message. */
export type Catalog = Record<string, string[] | Record<string, string>>

/** The catalogues built from the committed sources, one lazy chunk each. */
const catalogs = import.meta.glob<{ default: Catalog }>('../languages/*.json')

/**
 * Returns the catalogue a locale ships, or nothing when it ships none.
 * @param locale - The language to read in.
 * @returns The catalogue, or nothing.
 */
export async function catalogFor(locale: string): Promise<Catalog | undefined> {
	const load = catalogs[`../languages/${locale}.json`]
	if (locale === DEFAULT_LOCALE || load === undefined) {
		return undefined
	}
	return (await load()).default
}
