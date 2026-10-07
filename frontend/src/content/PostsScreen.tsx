// SPDX-License-Identifier: Apache-2.0

import { CHANGE_OTHERS_WORK, can } from '@gophenberg/frontend-sdk'
import { DataViews } from '@gophenberg/frontend-sdk/dataviews'
import type { View } from '@gophenberg/frontend-sdk/dataviews'
import { __, sprintf } from '@wordpress/i18n'
import { ErrorNotice, Page } from '@gopherium/godmin'
import { useSession } from '@gopherium/react-auth'
import { useQuery } from '@tanstack/react-query'
import { useLayoutEffect, useMemo, useRef, useState } from 'react'
import type { RefObject } from 'react'

import { usePostActions } from './actions'
import type { ListRun } from './actions'
import { fetchPostCounts, listPosts } from './api'
import type { PostCounts, PostPage } from './api'
import { EmptyTrash } from './EmptyTrash'
import { fieldTerms, postFields } from './fields'
import { StatusGhost, StatusViews } from './StatusViews'
import { useContentType } from './useContentType'

const PER_PAGE = 20

const EMPTY_PAGE: PostPage = { items: [], total: 0 }

const INITIAL_VIEW: View = {
	type: 'table',
	fields: ['author', 'date'],
	titleField: 'title',
	search: '',
	page: 1,
	perPage: PER_PAGE,
	sort: { field: 'date', direction: 'desc' },
}

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
 * @param total - How many items the list holds.
 * @param role - The role the session carries.
 * @returns Whether the empty trash control shows.
 */
function offersEmptyTrash(status: string, total: number, role: string | undefined): boolean {
	return status === 'trash' && total > 0 && can(role, CHANGE_OTHERS_WORK)
}

/**
 * Renders the list screen of a content type.
 * @returns The list screen element.
 */
export function PostsScreen() {
	const listed = useContentType()
	const [view, setView] = useState<View>(INITIAL_VIEW)
	const [offered, setOffered] = useState('')
	const [status, setStatus] = useState('')
	const [selection, setSelection] = useState<string[]>([])
	const { failure, run, list } = useListRun(`${listed.key} ${status}`, setSelection)
	const actions = usePostActions(status, listed.key, run)
	const session = useSession().data
	const counts = useQuery({
		queryKey: ['post-counts', listed.key],
		queryFn: () => fetchPostCounts(listed.key),
	})
	const columns = useMemo(() => postFields(listed), [listed])
	const shown = columns.map((column) => column.id).join(',')
	const settled = listed.key + ' ' + shown
	if (offered !== settled) {
		setOffered(settled)
		setView((current) => ({
			...current,
			fields: shown.split(',').filter((id) => id !== 'title'),
			filters: [],
			page: 1,
		}))
	}
	const terms = fieldTerms(view.filters)
	const posts = useQuery({
		queryKey: ['posts', listed.key, status, view.search, view.page, view.sort, terms],
		queryFn: () =>
			listPosts({
				type: listed.key,
				status,
				search: view.search,
				page: view.page,
				orderBy: view.sort?.field,
				order: view.sort?.direction,
				fields: terms,
			}),
	})
	/**
	 * Filters the list by the given status and returns to the first page.
	 * @param chosen - The status to filter by, empty for every status.
	 */
	function chooseStatus(chosen: string) {
		setStatus(chosen)
		setView((current) => ({ ...current, page: 1 }))
	}
	const page = posts.data ?? EMPTY_PAGE
	return (
		<Page
			title={listed.pluralLabel}
			actions={
				offersEmptyTrash(status, page.total, session?.role) ? (
					<EmptyTrash type={listed.key} label={listed.pluralLabel} list={run} />
				) : undefined
			}
		>
			<StatusRow
				counts={counts.data}
				failed={counts.isError}
				current={status}
				onSelect={chooseStatus}
			/>
			{failure === undefined ? null : <ErrorNotice>{failure}</ErrorNotice>}
			{posts.isError ? (
				<ErrorNotice>
					{sprintf(__('Could not load %(type)s.', 'gophenberg'), {
						type: listed.pluralLabel.toLowerCase(),
					})}
				</ErrorNotice>
			) : (
				<div
					ref={list}
					className="godmin-table-scroll"
					role="region"
					aria-label={listed.pluralLabel}
					tabIndex={0}
				>
					<DataViews
						data={page.items}
						fields={columns}
						actions={actions}
						view={view}
						onChangeView={setView}
						selection={selection}
						onChangeSelection={setSelection}
						isLoading={posts.isPending}
						getItemId={(post) => post.id}
						searchLabel={sprintf(__('Search %(type)s', 'gophenberg'), {
							type: listed.pluralLabel.toLowerCase(),
						})}
						config={{ perPageSizes: [PER_PAGE] }}
						paginationInfo={{
							totalItems: page.total,
							totalPages: Math.max(1, Math.ceil(page.total / PER_PAGE)),
						}}
						defaultLayouts={{ table: {} }}
					/>
				</div>
			)}
		</Page>
	)
}

/**
 * Renders the status filter row, its ghost until the counts arrive, or nothing.
 * @param props - The counts, whether the read failed, the status and the handler.
 * @returns The status row element, its ghost, or null.
 */
function StatusRow({
	counts,
	failed,
	current,
	onSelect,
}: {
	counts: PostCounts | undefined
	failed: boolean
	current: string
	onSelect: (chosen: string) => void
}) {
	if (failed) {
		return null
	}
	if (counts === undefined) {
		return <StatusGhost />
	}
	return <StatusViews counts={counts} current={current} onSelect={onSelect} />
}
