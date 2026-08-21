// SPDX-License-Identifier: Apache-2.0

import { DOMAIN as BRICK_DOMAIN, catalogFor as brickCatalogFor } from '@gopherium/react-auth'
import { setLocaleData } from '@wordpress/i18n'

import { rememberLocale } from '@gopherium/gottext'

import { fetchLocale } from './api'
import { catalogFor, editorCatalogFor, type Catalog } from './catalog'

/** The text domain every Gophenberg owned string names. */
export const DOMAIN = 'gophenberg'

/** Where the catalogues a language reads come from. */
export interface Catalogs {
	own: (locale: string) => Promise<Catalog | undefined>
	editor: (locale: string) => Promise<Catalog | undefined>
	brick: (locale: string) => Promise<Catalog | undefined>
}

/** The catalogues the admin reads in a browser. */
const shipped: Catalogs = { own: catalogFor, editor: editorCatalogFor, brick: brickCatalogFor }

/**
 * Loads the catalogues the admin reads and returns the language it settled on.
 * @param from - Where the catalogues come from, the shipped ones by default.
 * @returns The language the admin reads in.
 */
export async function startLocale(from: Catalogs = shipped): Promise<string> {
	const { locale } = await fetchLocale()
	rememberLocale(locale)
	const [own, editor, brick] = await Promise.all([
		from.own(locale),
		from.editor(locale),
		from.brick(locale),
	])
	if (own !== undefined) {
		setLocaleData(own, DOMAIN)
	}
	if (editor !== undefined) {
		setLocaleData(editor)
	}
	if (brick !== undefined) {
		setLocaleData(brick, BRICK_DOMAIN)
	}
	return locale
}
