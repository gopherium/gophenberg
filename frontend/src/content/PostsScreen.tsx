// SPDX-License-Identifier: Apache-2.0

import { Button, CHANGE_OTHERS_WORK, can, postIcon, trashIcon } from '@gophenberg/frontend-sdk'
import type { Session } from '@gophenberg/frontend-sdk'
import { DataViews } from '@gophenberg/frontend-sdk/dataviews'
import type { Action, Field, View } from '@gophenberg/frontend-sdk/dataviews'
import { __, sprintf } from '@wordpress/i18n'
import {
	ErrorNotice,
	ListEmpty,
	ListNoResults,
	Page,
	offeredSize,
	offeredView,
	paginationOf,
	sizedView,
	useListRun,
	useToaster,
	viewNarrows,
} from '@gopherium/godmin'
import type { OfferedView } from '@gopherium/godmin'
import { openOnTap, useListView } from '@gopherium/godmin/router'
import type { ListView } from '@gopherium/godmin/router'
import { useSession } from '@gopherium/react-auth'
import { useQuery } from '@tanstack/react-query'
import type { QueryKey, UseQueryResult } from '@tanstack/react-query'
import { Link, useNavigate, useSearch } from '@tanstack/react-router'
import { useMemo } from 'react'

import type { ListSettings } from '../settings/api'
import { useListSettings } from '../settings/useListSettings'
import { usePostActions } from './actions'
import { listPosts } from './api'
import type { Post, PostPage, PostQuery } from './api'
import { EmptyTrash } from './EmptyTrash'
import {
	OPENING_SORT,
	PHONE_COLUMNS,
	POST_VIEW_GUARD,
	TABLE_LAYOUT,
	levelOf,
	openingColumns,
	postFields,
	postNarrowing,
} from './fields'
import { typesQueryKey } from './nav'
import { StatusTabs } from './StatusTabs'
import { addLabelOf, pluralOf, typeText } from './typeText'
import { listTypes } from './types'
import type { ContentType } from './types'
import { useAddNew } from './useAddNew'
import { useContentType } from './useContentType'

/** The rows a list holds before its first page arrives. */
const NO_POSTS: Post[] = []

/** What the All tab asks for: every status but the trash, the scheduled one included. */
const EVERY_STATUS_BUT_TRASH = 'draft,pending,private,scheduled,published'

/**
 * Reports whether the list offers to empty the trash.
 * @param status - The status the list shows.
 * @param page - The page the list shows, absent until it arrives.
 * @param held - How many items the trash of the type holds, 0 until the count arrives.
 * @param session - The signed in account, or nothing.
 * @returns Whether the empty trash control shows, which takes a page showing rows, as WordPress offers it, and a
 *   count for its confirm to name.
 */
function offersEmptyTrash(
	status: string,
	page: PostPage | undefined,
	held: number,
	session: Session | null | undefined,
): boolean {
	return status === 'trash' && (page?.items.length ?? 0) > 0 && held > 0 && can(session, CHANGE_OTHERS_WORK)
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
 * @returns The page query, whether the list waits for the registry and whether the registry holds the type.
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
	return { posts, waiting: registry.isPending, registered: registry.data?.some((held) => held.key === type) === true }
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
function listingOf(view: OfferedView<View>, settings: ListSettings | undefined): PostQuery {
	return {
		search: view.search,
		...sizedView(view, settings?.list_page_sizes),
		orderBy: view.sort?.field,
		order: view.sort?.direction,
		orderHierarchy: view.showLevels,
		...postNarrowing(view.filters),
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
		layoutSettings: { table: TABLE_LAYOUT },
	})
	const offered = offeredView(list.view, columns, POST_VIEW_GUARD)
	const view = {
		...offered,
		perPage: offeredSize(offered.perPage, settings?.list_page_sizes, settings?.list_page_size),
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
	return typeText(listed, 'description', listed.description)
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
 * Returns the words an empty tab shows, read fresh so the loaded catalogue answers.
 * @returns The title of each tab other than All and Trash, keyed by its status.
 */
function emptyTabTitles(): Record<string, string> {
	return {
		published: __('No published items', 'gophenberg'),
		draft: __('No drafts', 'gophenberg'),
		pending: __('No pending items', 'gophenberg'),
		private: __('No private items', 'gophenberg'),
	}
}

/**
 * Renders what a list shows when it holds no item.
 * @param props - The view and the status the list shows, and the type its add button adds, if any.
 * @returns The empty state: no results for a search or a filter, the words of a tab, else the add button.
 */
function PostsEmpty({ view, status, adding }: {
	view: OfferedView<View>
	status: string
	adding: ContentType | undefined
}) {
	if (viewNarrows(view)) {
		return <ListNoResults />
	}
	if (status === 'trash') {
		return <ListEmpty icon={trashIcon} title={__('Trash is empty', 'gophenberg')} />
	}
	if (status !== '') {
		return <ListEmpty icon={postIcon} title={emptyTabTitles()[status]} />
	}
	return (
		<ListEmpty
			icon={postIcon}
			title={__('No items yet', 'gophenberg')}
			actions={adding === undefined ? null : <HeaderAddNew listed={adding} size="default" />}
		/>
	)
}

/**
 * Renders the rows of a type's list, or the failure of their read.
 * @param props - The type and status shown, the page query, whether the list waits for the type registry and whether
 *   the registry holds the type, the list view, the view the columns draw, the columns, the settings, the row actions
 *   and whether a write reloads the list.
 * @returns The list, or the failure notice.
 */
function PostsTable(props: {
	listed: ContentType
	status: string
	posts: UseQueryResult<PostPage>
	waiting: boolean
	registered: boolean
	list: ListView<View>
	view: OfferedView<View>
	columns: Field<Post>[]
	settings: ListSettings | undefined
	actions: Action<Post>[]
	refreshing: boolean
}) {
	const navigate = useNavigate()
	const { listed, posts, list } = props
	if (posts.isError) {
		return <ErrorNotice>{__('Items could not be loaded.', 'gophenberg')}</ErrorNotice>
	}
	return (
		<DataViews
			key={listed.key}
			{...pageProps(listed, posts.data, props.settings)}
			empty={<PostsEmpty view={props.view} status={props.status} adding={props.registered ? listed : undefined} />}
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
			isLoading={posts.isPending || posts.isPlaceholderData || props.waiting || props.refreshing}
			getItemId={(post) => post.id}
			searchLabel={sprintf(__('Search %(type)s…', 'gophenberg'), { type: pluralOf(listed).toLowerCase() })}
		/>
	)
}

/**
 * Renders the add button of a list, its failure shown in a toast like the failure of any write.
 * @param props - The type the list shows and the button size, compact when absent, as a page header button.
 * @returns The add button.
 */
function HeaderAddNew({ listed, size = 'compact' }: { listed: ContentType, size?: 'compact' | 'default' }) {
	const addNew = useAddNew(listed)
	const toaster = useToaster()
	return (
		<Button
			variant="solid"
			size={size}
			loading={addNew.isPending}
			onClick={() => addNew.mutate(undefined, { onError: (failure) => toaster.show(failure.message) })}
		>
			{addLabelOf(listed)}
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
	const { list, columns, view, settings, posts, waiting, registered } = usePostList(listed, status)
	const { run, region, refreshing } = useListRun(`${listed.key} ${status}`, list.onChangeSelection)
	const actions = usePostActions(status, listed, run)
	const held = useTrashTotal(listed.key, status)
	const session = useSession().data
	return (
		<Page
			title={pluralOf(listed)}
			headingLevel={2}
			subtitle={waiting ? undefined : subtitleOf(listed)}
			actions={
				<>
					{offersEmptyTrash(status, posts.data, held, session) ? (
						<EmptyTrash type={listed.key} count={held} list={run} />
					) : null}
					{registered ? <HeaderAddNew listed={listed} /> : null}
				</>
			}
			tabs={<StatusTabs current={current} />}
			list
			listLabel={pluralOf(listed)}
			listRef={region}
		>
			<PostsTable
				listed={listed}
				status={status}
				posts={posts}
				waiting={waiting}
				registered={registered}
				list={list}
				view={view}
				columns={columns}
				settings={settings}
				actions={actions}
				refreshing={refreshing}
			/>
		</Page>
	)
}
