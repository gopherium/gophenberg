// SPDX-License-Identifier: Apache-2.0

import type { ListDefaults, ViewGuard } from '@gopherium/godmin'

/** The order the list opens in, by name from A, and the one it keeps for a sort the fields never offer. */
const PLUGINS_SORT = { field: 'name', direction: 'asc' } as const

/** The order every view check falls back to. */
export const PLUGINS_VIEW_GUARD: ViewGuard = { opening: PLUGINS_SORT }

/** What an app names for the view the plugins list opens on. */
export interface PluginsListOptions {
	/** How many plugins a page holds, every plugin while it is unknown. */
	perPage?: number
}

/**
 * Returns the view the plugins list opens on, as data for godmin's useListView.
 * @param options - The page size.
 * @returns The fields, the phone fields, the title, the paused reason under it, the order and the page size.
 */
export function pluginsListDefaults({ perPage }: PluginsListOptions = {}): ListDefaults {
	return {
		fields: ['description', 'status', 'version'],
		phoneFields: ['description', 'status'],
		titleField: 'name',
		descriptionField: 'paused_reason',
		sort: { ...PLUGINS_SORT },
		perPage,
	}
}
