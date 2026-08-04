// SPDX-License-Identifier: Apache-2.0

import { env } from 'node:process'

/** Where the instance serves its own stylesheets and icon. */
const assetPath = '/gophenberg'

/** Where the instance serves the feed a page links.  */
const feedPath = '/api/plugins/feed/rss.xml'

/**
 * Returns the origin the instance's own files are loaded from.
 * @returns The origin, empty when the theme shares it.
 */
function assetOrigin(): string {
	return (env.GOPHENBERG_ASSET_ORIGIN ?? '').replace(/\/$/, '')
}

/** The addresses a themed page loads the instance's own files from. */
export interface SiteAssetUrls {
	blocks: string
	presets: string
	favicon: string
	feed: string
}

/**
 * Returns where a themed page loads the instance's own files from.
 * @returns The addresses to put in the document head.
 */
export function siteAssetUrls(): SiteAssetUrls {
	const origin = assetOrigin()
	return {
		blocks: `${origin}${assetPath}/blocks.css`,
		presets: `${origin}${assetPath}/presets.css`,
		favicon: `${origin}${assetPath}/favicon.svg`,
		feed: `${origin}${feedPath}`,
	}
}
