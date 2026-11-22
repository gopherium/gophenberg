// SPDX-License-Identifier: Apache-2.0

import { DataViews, filterSortAndPaginate } from '@gophenberg/frontend-sdk/dataviews'
import type { Action, Field, View } from '@gophenberg/frontend-sdk/dataviews'
import { ErrorNotice, ListEmpty, ListNoResults, Page, offeredView, viewNarrows } from '@gopherium/godmin'
import type { ListEmptyProps, ListView, OfferedView } from '@gopherium/godmin'
import { useMemo } from 'react'
import type { ReactNode } from 'react'

import { PLUGINS_VIEW_GUARD } from './defaults'
import { pluginFields } from './fields'
import type { FieldWords, InstalledPlugin, PluginStatus } from './fields'

/** A state a tab narrows the list to, the All tab naming none. */
type PluginTab = Extract<PluginStatus, 'active' | 'paused'>

/** Every word the page shows, each one in the reader's language. */
export interface PluginsWords extends FieldWords {
	/** The page title, which names the list region too. */
	title: string
	/** The sentence under the title. */
	subtitle: string
	/** The label of the search box. */
	searchLabel: string
	/** What shows in place of the list when the plugins could not be read. */
	loadFailed: string
	/** What the All tab says when the site holds no plugin. */
	noPlugins: string
	/** What each other tab says when no plugin stands in its state. */
	emptyTabs: Record<PluginTab, string>
}

/** What an app hands the plugins page: the view it holds, the rows, the tabs, its words and its glyph, all as data. */
export interface PluginsPageProps {
	/** The view godmin's useListView holds in the address, opened on pluginsListDefaults. */
	list: ListView<View>
	/** Every installed plugin, its name and description already in the reader's language. */
	plugins: InstalledPlugin[]
	/** Whether the plugins are still being read. */
	loading: boolean
	/** Whether the plugins could not be read. */
	failed: boolean
	/** The state the tab on screen narrows the list to, none for All. */
	status?: PluginTab
	/** The tabs drawn under the header. */
	tabs?: ReactNode
	/** The page sizes the list offers, none while the settings load. */
	pageSizes?: number[]
	/** The row actions, each saying with isEligible which plugins it fits. */
	actions?: Action<InstalledPlugin>[]
	/** The words the page shows. */
	words: PluginsWords
	/** The glyph the empty list shows. */
	icon: ListEmptyProps['icon']
	/** The level of the page title, 2 on a screen beside the sidebar. */
	headingLevel?: 1 | 2
}

/** The page sizes of a list whose app offers none yet. */
const NO_SIZES: number[] = []

/**
 * Returns the fields a tab draws, the status only on All, since every other tab holds one state.
 * @param fields - Every field the list offers.
 * @param status - The state the tab narrows the list to, none for All.
 * @returns The fields DataViews draws.
 */
function fieldsOn(fields: Field<InstalledPlugin>[], status: PluginTab | undefined): Field<InstalledPlugin>[] {
	return status === undefined ? fields : fields.filter((field) => field.id !== 'status')
}

/**
 * Returns the plugins a tab holds, before the list searches, sorts and pages them.
 * @param plugins - Every installed plugin.
 * @param status - The state the tab narrows the list to, none for All.
 * @returns The plugins in that state, or every plugin on All.
 */
function pluginsOn(plugins: InstalledPlugin[], status: PluginTab | undefined): InstalledPlugin[] {
	return status === undefined ? plugins : plugins.filter((plugin) => plugin.status === status)
}

/**
 * Renders what the list shows when it holds no plugin.
 * @param props - The view the list shows, the state its tab holds, the page's words and the glyph.
 * @returns No results for a search, else the words of the tab.
 */
function PluginsEmpty({ view, status, words, icon }: {
	view: OfferedView<View>
	status: PluginTab | undefined
	words: PluginsWords
	icon: ListEmptyProps['icon']
}) {
	if (viewNarrows(view)) {
		return <ListNoResults />
	}
	return <ListEmpty icon={icon} title={status === undefined ? words.noPlugins : words.emptyTabs[status]} />
}

/**
 * Renders the plugins of the tab on screen, searched, sorted and paged in the browser.
 * @param props - What the app hands the page.
 * @returns The list.
 */
function PluginsList({ list, words, status, ...props }: PluginsPageProps) {
	const fields = useMemo(() => fieldsOn(pluginFields(words), status), [words, status])
	const view = offeredView(list.view, fields, PLUGINS_VIEW_GUARD)
	const shown = filterSortAndPaginate(pluginsOn(props.plugins, status), view, fields)
	return (
		<DataViews<InstalledPlugin>
			data={shown.data}
			paginationInfo={shown.paginationInfo}
			config={{ perPageSizes: props.pageSizes ?? NO_SIZES }}
			empty={<PluginsEmpty view={view} status={status} words={words} icon={props.icon} />}
			fields={fields}
			actions={props.actions}
			view={view}
			onChangeView={list.onChangeView}
			defaultLayouts={list.defaultLayouts}
			selection={list.selection}
			onChangeSelection={list.onChangeSelection}
			isLoading={props.loading}
			searchLabel={words.searchLabel}
		/>
	)
}

/**
 * Renders the Plugins page: the header, the tabs the app draws and the installed plugins as a list.
 * @param props - The view the app holds, the rows, the tabs, its words and its glyph.
 * @returns The page, the failure notice in place of the list when the plugins could not be read.
 */
export function PluginsPage(props: PluginsPageProps) {
	const { words } = props
	return (
		<Page
			title={words.title}
			headingLevel={props.headingLevel}
			subtitle={words.subtitle}
			tabs={props.tabs}
			list
			listLabel={words.title}
		>
			{props.failed ? <ErrorNotice>{words.loadFailed}</ErrorNotice> : <PluginsList {...props} />}
		</Page>
	)
}
