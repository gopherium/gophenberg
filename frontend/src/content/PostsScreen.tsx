// SPDX-License-Identifier: Apache-2.0

import { Button, CHANGE_OTHERS_WORK, can, postIcon } from '@gophenberg/frontend-sdk'
import { DataViews } from '@gophenberg/frontend-sdk/dataviews'
import type { Action, Field, View } from '@gophenberg/frontend-sdk/dataviews'
import { __, sprintf } from '@wordpress/i18n'
import { ErrorNotice, ListEmpty, Page, paginationOf } from '@gopherium/godmin'
import { openOnTap, useListView } from '@gopherium/godmin/router'
import type { ListView } from '@gopherium/godmin/router'
import { useSession } from '@gopherium/react-auth'
import { useQuery } from '@tanstack/react-query'
import type { QueryKey, UseQueryResult } from '@tanstack/react-query'
import { Link, useNavigate, useSearch } from '@tanstack/react-router'
import { useLayoutEffect, useMemo, useRef, useState } from 'react'
import type { RefObject } from 'react'

import type { ListSettings } from '../settings/api'
import { useListSettings } from '../settings/useListSettings'
import { usePostActions } from './actions'
import type { ListRun } from './actions'
import { listPosts } from './api'
import type { Post, PostPage, PostQuery } from './api'
import { EmptyTrash } from './EmptyTrash'
import { OPENING_SORT, PHONE_COLUMNS, levelOf, narrowedBy, offeredView, openingColumns, postFields } from './fields'
import type { OfferedView } from './fields'
import { typesQueryKey } from './nav'
import { StatusTabs } from './StatusTabs'
import { listTypes } from './types'
import type { ContentType } from './types'
import { useAddNew } from './useAddNew'
import { useContentType } from './useContentType'

/** The rows a list holds before its first page arrives. */
const NO_POSTS: Post[] = []

/** What the All tab asks for: every status but the trash, the scheduled one included. */
const EVERY_STATUS_BUT_TRASH = 'draft,pending,private,scheduled,published'

/**
 * Holds the failure shown above the list of one type and status, and the list a finished run focuses.
 * @param scope - The type and status the list shows.
 * @param setTicks - Replaces the ticked rows.
 * @returns The failure on show, what the list does around a run, and the list element.
 */
function useListRun(
	scope: string,
	setTicks: (ids: string[]) => void,
): { failure?: string, run: ListRun, list: RefObject<HTMLDivElement | null> } {
	const [shown, setShown] = useState<{ scope: string, failure?: string }>({ scope })
	const list = useRef<HTMLDivElement>(null)
	const latest = useRef(0)
	useLayoutEffect(() => {
		latest.current += 1
	}, [scope])
	if (shown.scope !== scope) {
		setShown({ scope })
		setTicks([])
	}
	const run = useMemo<ListRun>(
		() => ({
			onStart: () => {
				const id = latest.current + 1
				latest.current = id
				setShown({ scope })
				return (failure, moved) => {
					if (latest.current !== id) {
						return
					}
					setShown({ scope, failure })
					if (moved) {
						list.current?.focus()
					}
				}
			},
		}),
		[scope],
	)
	return { failure: shown.failure, run, list }
}

/**
 * Reports whether the list offers to empty the trash.
 * @param status - The status the list shows.
 * @param page - The page the list shows, absent until it arrives.
 * @param held - How many items the trash of the type holds, 0 until the count arrives.
 * @param role - The role the session carries.
 * @returns Whether the empty trash control shows, which takes a page showing rows, as WordPress offers it, and a
 *   count for its confirm to name.
 */
function offersEmptyTrash(status: string, page: PostPage | undefined, held: number, role: string | undefined): boolean {
	return status === 'trash' && (page?.items.length ?? 0) > 0 && held > 0 && can(role, CHANGE_OTHERS_WORK)
}

/**
 * Reports whether a query read the list of the given type and status.
 * @param key - The key of the query, absent before any query ran.
 * @param type - The type the list shows.
 * @param status - The status the list shows.
 * @returns Whether the query read that same list.
 */
function readsList(key: QueryKey | undefined, type: string, status: string): boolean {
	return key?.[1] === type && key[2] === status
}

/**
 * Reads the page of posts a request asks for once the type registry answered.
 * @param type - The type the list shows.
 * @param status - The status the list shows.
 * @param asked - The search, sort, page and filters to ask for.
 * @returns The query of the page, and whether the list still waits for the registry.
 */
function usePostPage(type: string, status: string, asked: PostQuery) {
	const registry = useQuery({ queryKey: typesQueryKey, queryFn: listTypes })
	const query = { ...asked, type, status: status === '' ? EVERY_STATUS_BUT_TRASH : status }
	const posts = useQuery({
		queryKey: ['posts', type, status, query],
		queryFn: () => listPosts(query),
		enabled: !registry.isPending,
		placeholderData: (previous, last) => (readsList(last?.queryKey, type, status) ? previous : undefined),
	})
	return { posts, waiting: registry.isPending }
}

/**
 * Reads how many items the trash of a type holds, whatever narrows the list, under the posts family a run refreshes.
 * @param type - The type the list shows.
 * @param status - The status the list shows, the count read only in the trash.
 * @returns The count, 0 until it arrives.
 */
function useTrashTotal(type: string, status: string): number {
	const trash = useQuery({
		queryKey: ['posts', type, 'trash', 'total'],
		queryFn: () => listPosts({ type, status: 'trash', perPage: 1 }),
		enabled: status === 'trash',
		select: (page) => page.total,
	})
	return trash.data ?? 0
}

/**
 * Returns the listing request a view stands for.
 * @param view - The view the list shows.
 * @param settings - The list settings, absent when they could not be read.
 * @returns The request, leaving the page size to the server while the settings name none.
 */
function listingOf(view: OfferedView, settings: ListSettings | undefined): PostQuery {
	return {
		search: view.search,
		page: view.page,
		perPage: settings === undefined ? undefined : view.perPage,
		orderBy: view.sort?.field,
		order: view.sort?.direction,
		orderHierarchy: view.showLevels,
		...narrowedBy(view.filters),
	}
}

/**
 * Reports whether a list keeps the order it opens in, the only order a nesting type draws its levels in.
 * @param view - The view the address and the reader hold.
 * @returns Whether the view sorts as the list opens.
 */
function opensSorted(view: View): boolean {
	return view.sort?.field === OPENING_SORT.field && view.sort.direction === OPENING_SORT.direction
}

/**
 * Returns the page size a list shows, the one the address names only while the settings offer it.
 * @param asked - The page size the address names, absent when it names none.
 * @param settings - The list settings, absent when they could not be read.
 * @returns The page size, the one the settings name in place of a size they do not offer.
 */
function offeredSize(asked: number | undefined, settings: ListSettings | undefined): number | undefined {
	if (settings === undefined || settings.list_page_sizes.some((size) => size === asked)) {
		return asked
	}
	return settings.list_page_size
}

/**
 * Holds the view of a type's list in the address, sized by the settings once they are known, and reads its page.
 * @param listed - The type the list shows.
 * @param status - The status the list shows.
 * @returns The list view, the columns, the view the columns can draw, the settings and the page query.
 */
function usePostList(listed: ContentType, status: string) {
	const settings = useListSettings().data
	const columns = useMemo(() => postFields(listed), [listed])
	const list = useListView<View>({
		fields: openingColumns(columns),
		phoneFields: PHONE_COLUMNS,
		titleField: 'title',
		descriptionField: 'excerpt',
		sort: { ...OPENING_SORT },
		perPage: settings?.list_page_size,
	})
	const offered = offeredView(list.view, columns)
	const view = {
		...offered,
		perPage: offeredSize(offered.perPage, settings),
		showLevels: listed.hierarchical && opensSorted(offered),
	}
	return { list, columns, view, settings, ...usePostPage(listed.key, status, listingOf(view, settings)) }
}

/**
 * Returns the columns a status shows, its own status column only on All, since every other tab holds one status.
 * @param columns - The columns the list offers.
 * @param status - The status the list shows.
 * @returns The columns DataViews draws.
 */
function columnsOn(columns: Field<Post>[], status: string): Field<Post>[] {
	return status === '' ? columns : columns.filter((column) => column.id !== 'status')
}

/**
 * Returns the sentence shown under the title of a list.
 * @param listed - The type the list shows.
 * @returns The type's own description, or a sentence of the admin's when it holds none.
 */
function subtitleOf(listed: ContentType): string {
	if (listed.description.trim() === '') {
		return __('Manage the items of this content type.', 'gophenberg')
	}
	return listed.description
}

/**
 * Returns the rows, the paging, the page sizes and the levels DataViews draws the page of a list with.
 * @param listed - The type the list shows.
 * @param page - The page the server served, absent until it arrives.
 * @param settings - The list settings, absent when they could not be read.
 * @returns The props of the page.
 */
function pageProps(listed: ContentType, page: PostPage | undefined, settings: ListSettings | undefined) {
	return {
		data: page?.items ?? NO_POSTS,
		paginationInfo: paginationOf(page === undefined ? undefined : { total: page.total, limit: page.perPage }),
		config: { perPageSizes: settings?.list_page_sizes ?? [] },
		getItemLevel: listed.hierarchical ? levelOf(listed) : undefined,
	}
}

/**
 * Reports whether a filter holds a value, a pick of none being no value.
 * @param value - The value the filter holds.
 * @returns Whether the value narrows the list.
 */
function holdsValue(value: unknown): boolean {
	return Array.isArray(value) ? value.length > 0 : value !== undefined
}

/**
 * Reports whether a search, a filter holding a value, or a tab other than All narrows a list.
 * @param view - The view the list shows.
 * @param status - The status the list shows, empty for All.
 * @returns Whether the list shows fewer items than the type holds.
 */
function narrows(view: OfferedView, status: string): boolean {
	return status !== '' || Boolean(view.search) || view.filters.some((filter) => holdsValue(filter.value))
}

/**
 * Renders what a list shows when it holds no item.
 * @param props - Whether a search, a filter or a tab narrowed the list.
 * @returns The empty state, telling how to add an item when nothing narrowed the list.
 */
function PostsEmpty({ narrowed }: { narrowed: boolean }) {
	if (narrowed) {
		return <ListEmpty icon={postIcon} title={__('No items found.', 'gophenberg')} />
	}
	const hint = __('Add one with Add New.', 'gophenberg')
	return <ListEmpty icon={postIcon} title={__('No items yet.', 'gophenberg')} hint={hint} />
}

/**
 * Renders the rows of a type's list, or the failure of their read.
 * @param props - The type and status shown, the page query, whether the list waits for the type registry, the list
 *   view, the view the columns draw, the columns, the settings and the row actions.
 * @returns The list, or the failure notice.
 */
function PostsTable(props: {
	listed: ContentType
	status: string
	posts: UseQueryResult<PostPage>
	waiting: boolean
	list: ListView<View>
	view: OfferedView
	columns: Field<Post>[]
	settings: ListSettings | undefined
	actions: Action<Post>[]
}) {
	const navigate = useNavigate()
	const { listed, posts, list } = props
	if (posts.isError) {
		return <ErrorNotice>{__('Items could not be loaded.', 'gophenberg')}</ErrorNotice>
	}
	return (
		<DataViews
			key={`${listed.key} ${props.status}`}
			{...pageProps(listed, posts.data, props.settings)}
			empty={<PostsEmpty narrowed={narrows(props.view, props.status)} />}
			fields={columnsOn(props.columns, props.status)}
			actions={props.actions}
			view={props.view}
			onChangeView={list.onChangeView}
			defaultLayouts={list.defaultLayouts}
			selection={list.selection}
			onChangeSelection={openOnTap(list, (postId) => {
				void navigate({ to: '/content/$typeKey/$postId/edit', params: { typeKey: listed.key, postId } })
			})}
			renderItemLink={({ item, ...link }) => (
				<Link to="/content/$typeKey/$postId/edit" params={{ typeKey: item.type, postId: item.id }} {...link} />
			)}
			isLoading={posts.isFetching || props.waiting}
			getItemId={(post) => post.id}
			searchLabel={sprintf(__('Search %(type)s…', 'gophenberg'), { type: listed.pluralLabel.toLowerCase() })}
		/>
	)
}

/**
 * Renders the Add New of the list header, its failure shown above the list like the failure of any run.
 * @param props - The type the list shows and what the list does around a run.
 * @returns The header button.
 */
function HeaderAddNew({ listed, run }: { listed: ContentType, run: ListRun }) {
	const addNew = useAddNew(listed)
	return (
		<Button
			variant="solid"
			size="compact"
			loading={addNew.isPending}
			disabled={addNew.waiting}
			onClick={() => {
				const settle = run.onStart()
				addNew.mutate(undefined, { onError: (failure) => settle(failure.message, false) })
			}}
		>
			{__('Add New', 'gophenberg')}
		</Button>
	)
}

/**
 * Renders the list screen of a content type.
 * @returns The list screen element.
 */
export function PostsScreen() {
	const listed = useContentType()
	const current = useSearch({ strict: false, select: (search) => search.status })
	const status = current ?? ''
	const { list, columns, view, settings, posts, waiting } = usePostList(listed, status)
	const { failure, run, list: region } = useListRun(`${listed.key} ${status}`, list.onChangeSelection)
	const actions = usePostActions(status, listed, run)
	const held = useTrashTotal(listed.key, status)
	const session = useSession().data
	return (
		<Page
			title={listed.pluralLabel}
			subtitle={waiting ? undefined : subtitleOf(listed)}
			actions={
				<>
					{offersEmptyTrash(status, posts.data, held, session?.role) ? (
						<EmptyTrash type={listed.key} count={held} list={run} />
					) : null}
					<HeaderAddNew listed={listed} run={run} />
				</>
			}
			tabs={<StatusTabs current={current} />}
		>
			<div ref={region} className="godmin-page__list" role="region" aria-label={listed.pluralLabel} tabIndex={-1}>
				{failure === undefined ? null : <ErrorNotice>{failure}</ErrorNotice>}
				<PostsTable
					listed={listed}
					status={status}
					posts={posts}
					waiting={waiting}
					list={list}
					view={view}
					columns={columns}
					settings={settings}
					actions={actions}
				/>
			</div>
		</Page>
	)
}
