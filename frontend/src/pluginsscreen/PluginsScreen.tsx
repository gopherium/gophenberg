// SPDX-License-Identifier: Apache-2.0

import { pluginsIcon } from '@gophenberg/frontend-sdk'
import type { View } from '@gophenberg/frontend-sdk/dataviews'
import { PageTab, PageTabs, offeredSize } from '@gopherium/godmin'
import { useListView } from '@gopherium/godmin/router'
import { useQuery } from '@tanstack/react-query'
import { Link, useSearch } from '@tanstack/react-router'
import { useMemo } from 'react'

import { tabSearch } from '../content/StatusTabs'
import { PluginsPage, pluginsListDefaults } from '../pluginspage'
import type { InstalledPlugin } from '../pluginspage'
import { useListSettings } from '../settings/useListSettings'
import { listPlugins, pluginsQueryKey } from './api'
import { pluginStatusSearch } from './nav'
import type { PluginTab } from './nav'
import { pluginsTabs, pluginsWords, translatedPlugin } from './words'

/** The plugins a list holds before the server answers. */
const NO_PLUGINS: InstalledPlugin[] = []

/**
 * Renders the tabs narrowing the Plugins list to one state, each a link to its own address.
 * @param props - The state the list shows, none for All.
 * @returns The tab navigation.
 */
function PluginsTabs({ current }: { current: PluginTab | undefined }) {
	const { label, tabs } = pluginsTabs()
	return (
		<PageTabs label={label}>
			{tabs.map((tab) => (
				<PageTab
					key={tab.label}
					current={tab.status === current}
					render={
						<Link
							to="."
							search={(held: Record<string, unknown>) => tabSearch(held, tab.status)}
							activeOptions={{ exact: true }}
						/>
					}
				>
					{tab.label}
				</PageTab>
			))}
		</PageTabs>
	)
}

/**
 * Reads the plugins the site holds, their names and descriptions in the reader's language.
 * @returns The plugins, none before the server answers, and whether the read is pending or failed.
 */
function useInstalledPlugins() {
	const listed = useQuery({ queryKey: pluginsQueryKey, queryFn: ({ signal }) => listPlugins(signal) })
	const plugins = useMemo(() => (listed.data ?? NO_PLUGINS).map(translatedPlugin), [listed.data])
	return { plugins, loading: listed.isPending, failed: listed.isError }
}

/**
 * Renders the Plugins screen, the plugins page over the view and the tab the address holds.
 * @returns The plugins screen element.
 */
export function PluginsScreen() {
	const status = useSearch({ strict: false, select: (search) => pluginStatusSearch(search).status })
	const settings = useListSettings()
	const sizes = settings.data?.list_page_sizes
	const size = settings.data?.list_page_size
	const list = useListView<View>(pluginsListDefaults({ perPage: size }))
	const view = { ...list.view, perPage: offeredSize(list.view.perPage, sizes, size) }
	const words = useMemo(() => pluginsWords(), [])
	const { plugins, loading, failed } = useInstalledPlugins()
	return (
		<PluginsPage
			list={{ ...list, view }}
			plugins={plugins}
			loading={loading}
			failed={failed}
			status={status}
			tabs={<PluginsTabs current={status} />}
			pageSizes={sizes}
			words={words}
			icon={pluginsIcon}
			headingLevel={2}
		/>
	)
}
