// SPDX-License-Identifier: Apache-2.0

import { LIST_CHROME_DOMAIN, listChromeCatalogFor } from '@gopherium/godmin'
import { DOMAIN as BRICK_DOMAIN, catalogFor as brickCatalogFor } from '@gopherium/react-auth'

import { startLocale as start } from '@gopherium/gottext'

import { fetchLocale } from './api'
import { DEFAULT_LOCALE, catalogFor, editorCatalogFor, type Catalog } from './catalog'
import { startDates } from './dates'

/** The text domain every Gophenberg owned string names. */
export const DOMAIN = 'gophenberg'

/** Where the catalogues a language reads come from. */
export interface Catalogs {
	own: (locale: string) => Promise<Catalog | undefined>
	editor: (locale: string) => Promise<Catalog | undefined>
	brick: (locale: string) => Promise<Catalog | undefined>
	chrome: (locale: string) => Promise<Catalog | undefined>
}

/** The catalogues the admin reads in a browser. */
const shipped: Catalogs = {
	own: catalogFor,
	editor: editorCatalogFor,
	brick: brickCatalogFor,
	chrome: listChromeCatalogFor,
}

/**
 * Loads the catalogues the admin reads, hands the dates their words and clock, and returns the language it settled on.
 * @param from - Where the catalogues come from, the shipped ones by default.
 * @returns The language the admin reads in.
 */
export async function startLocale(from: Catalogs = shipped): Promise<string> {
	const locale = await start(
		async () => (await fetchLocale()).locale,
		[
			{ domain: DOMAIN, load: from.own },
			{ load: from.editor },
			{ domain: BRICK_DOMAIN, load: from.brick },
			{ domain: LIST_CHROME_DOMAIN, load: from.chrome },
		],
		{ defaultLocale: DEFAULT_LOCALE },
	)
	startDates(locale)
	return locale
}
