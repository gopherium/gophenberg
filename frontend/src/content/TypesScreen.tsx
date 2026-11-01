// SPDX-License-Identifier: Apache-2.0

import { Button } from '@gophenberg/frontend-sdk'
import { DataViews, filterSortAndPaginate } from '@gophenberg/frontend-sdk/dataviews'
import type { Action, Field, View } from '@gophenberg/frontend-sdk/dataviews'
import { ErrorNotice, ListNoResults, Page, offeredSize, offeredView, useListRun } from '@gopherium/godmin'
import type { ListRun, OfferedView } from '@gopherium/godmin'
import { useListView } from '@gopherium/godmin/router'
import type { ListView } from '@gopherium/godmin/router'
import { useQuery } from '@tanstack/react-query'
import type { UseQueryResult } from '@tanstack/react-query'
import { __, sprintf } from '@wordpress/i18n'
import { useMemo, useState } from 'react'

import type { ListSettings } from '../settings/api'
import { useListSettings } from '../settings/useListSettings'
import { typesQueryKey } from './nav'
import { TYPE_COLUMNS, TYPE_PHONE_COLUMNS, TYPE_SORT, TYPE_VIEW_GUARD, originsOf, typeFields } from './typeFields'
import { useTypeActions } from './typeActions'
import { AddTypeDialog } from './TypeModals'
import { listTypes } from './types'
import type { ContentType } from './types'

/** The types a list holds before the registry answers. */
const NO_TYPES: ContentType[] = []

/**
 * Returns the columns of the types list, built again only when the plugins that declared a type change.
 * @param types - The types the registry holds, none before it answers.
 * @returns The columns.
 */
function useTypeFields(types: ContentType[] | undefined): Field<ContentType>[] {
	const origins = JSON.stringify(originsOf(types ?? NO_TYPES))
	return useMemo(() => typeFields(JSON.parse(origins) as string[]), [origins])
}

/**
 * Holds the view of the types list in the address, sized by the settings once they are known.
 * @param fields - The columns the list offers.
 * @returns The list view, the view the columns can draw and the settings.
 */
function useTypesView(fields: Field<ContentType>[]) {
	const settings = useListSettings().data
	const list = useListView<View>({
		fields: TYPE_COLUMNS,
		phoneFields: TYPE_PHONE_COLUMNS,
		titleField: 'title',
		descriptionField: 'key',
		sort: { ...TYPE_SORT },
		perPage: settings?.list_page_size,
	})
	const offered = offeredView(list.view, fields, TYPE_VIEW_GUARD)
	const view = {
		...offered,
		perPage: offeredSize(offered.perPage, settings?.list_page_sizes, settings?.list_page_size),
	}
	return { list, view, settings }
}

/**
 * Renders the rows of the registry, searched, sorted and paged in the browser, or the failure of its read.
 * @param props - The registry query, the list view, the view the columns draw, the columns, the settings, the row
 *   actions and whether a write reloads the list.
 * @returns The list, or the failure notice.
 */
function TypesTable(props: {
	types: UseQueryResult<ContentType[]>
	list: ListView<View>
	view: OfferedView<View>
	fields: Field<ContentType>[]
	settings: ListSettings | undefined
	actions: Action<ContentType>[]
	refreshing: boolean
}) {
	const { types, list, view, fields } = props
	if (types.isError) {
		return <ErrorNotice>{__('The content types could not be loaded.', 'gophenberg')}</ErrorNotice>
	}
	const shown = filterSortAndPaginate(types.data ?? NO_TYPES, view, fields)
	return (
		<DataViews
			data={shown.data}
			paginationInfo={shown.paginationInfo}
			config={{ perPageSizes: props.settings?.list_page_sizes ?? [] }}
			empty={<ListNoResults />}
			fields={fields}
			actions={props.actions}
			view={view}
			onChangeView={list.onChangeView}
			defaultLayouts={list.defaultLayouts}
			selection={list.selection}
			onChangeSelection={list.onChangeSelection}
			isLoading={types.isPending || props.refreshing}
			getItemId={(type) => type.key}
			searchLabel={sprintf(__('Search %(type)s…', 'gophenberg'), {
				type: __('Content Types', 'gophenberg').toLowerCase(),
			})}
		/>
	)
}

/**
 * Renders the header button adding a content type, and its dialog while it asks.
 * @param props - What the list does around a reload.
 * @returns The button and the dialog.
 */
function AddType({ list }: { list: ListRun }) {
	const [asking, setAsking] = useState(false)
	return (
		<>
			<Button variant="solid" size="compact" onClick={() => setAsking(true)}>
				{__('Add Content Type', 'gophenberg')}
			</Button>
			{asking ? <AddTypeDialog list={list} onClose={() => setAsking(false)} /> : null}
		</>
	)
}

/**
 * Renders the content type registry as a WordPress list.
 * @returns The registry screen element.
 */
export function TypesScreen() {
	const types = useQuery({ queryKey: typesQueryKey, queryFn: listTypes })
	const fields = useTypeFields(types.data)
	const { list, view, settings } = useTypesView(fields)
	const { run, region, refreshing } = useListRun('content-types', list.onChangeSelection)
	const actions = useTypeActions(run)
	return (
		<Page
			title={__('Content Types', 'gophenberg')}
			headingLevel={2}
			subtitle={__('Every kind of content this site holds.', 'gophenberg')}
			actions={<AddType list={run} />}
			list
			listLabel={__('Content Types', 'gophenberg')}
			listRef={region}
		>
			<TypesTable
				types={types}
				list={list}
				view={view}
				fields={fields}
				settings={settings}
				actions={actions}
				refreshing={refreshing}
			/>
		</Page>
	)
}
