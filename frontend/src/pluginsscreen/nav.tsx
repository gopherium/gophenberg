// SPDX-License-Identifier: Apache-2.0

import { pluginsIcon } from '@gophenberg/frontend-sdk'
import type { MenuItem } from '@gophenberg/frontend-sdk'
import { _x } from '@wordpress/i18n'

import type { PluginsPageProps } from '../pluginspage'

/** A state a tab of the Plugins list narrows it to. */
export type PluginTab = NonNullable<PluginsPageProps['status']>

/** The states a tab narrows the Plugins list to, in the order the tabs show them after All. */
const PLUGIN_TABS = ['active', 'paused'] as const satisfies readonly PluginTab[]

/** The Plugins section, where every plugin the site holds is listed. */
export const pluginsMenuItem: MenuItem = {
	get label() {
		return _x('Plugins', 'admin section', 'gophenberg')
	},
	to: '/plugins',
	icon: pluginsIcon,
}

/**
 * Keeps the state an address names when a tab of the Plugins list narrows it to that state.
 * @param raw - The search the router parsed from the address.
 * @returns The state, undefined for the All tab so it overwrites any other value the address holds.
 */
export function pluginStatusSearch(raw: Record<string, unknown>): { status?: PluginTab } {
	return { status: PLUGIN_TABS.find((held) => held === raw.status) }
}
