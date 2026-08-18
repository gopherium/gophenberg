// SPDX-License-Identifier: Apache-2.0

import { DataViews } from '@gophenberg/frontend-sdk/dataviews'
import type { View } from '@gophenberg/frontend-sdk/dataviews'
import { __, sprintf } from '@wordpress/i18n'
import { ErrorNotice, Page } from '@gopherium/godmin'
import { useQuery } from '@tanstack/react-query'
import { useCallback, useState } from 'react'

import { usePostActions, useRefresh } from './actions'
import type { PostNotice } from './actions'
import { fetchPostCounts, listPosts } from './api'
import type { PostCounts, PostPage } from './api'
import { EmptyTrash } from './EmptyTrash'
import { postFields } from './fields'
import { PostsNotice } from './PostsNotice'
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
 * Renders the list screen of a content type.
 * @returns The list screen element.
 */
export function PostsScreen() {
	const listed = useContentType()
	const [view, setView] = useState<View>(INITIAL_VIEW)
	const [status, setStatus] = useState('')
	const [notice, setNotice] = useState<PostNotice | null>(null)
	const [selection, setSelection] = useState<string[]>([])
	const report = useCallback((next: PostNotice | null) => {
		setNotice(next)
		if (next?.intent === 'success') {
			setSelection([])
		}
	}, [])
	const actions = usePostActions(status, report)
	const refresh = useRefresh()
	const counts = useQuery({
		queryKey: ['post-counts', listed.key],
		queryFn: () => fetchPostCounts(listed.key),
	})
	const posts = useQuery({
		queryKey: ['posts', listed.key, status, view.search, view.page, view.sort],
		queryFn: () =>
			listPosts({
				type: listed.key,
				status,
				search: view.search,
				page: view.page,
				orderBy: view.sort?.field,
				order: view.sort?.direction,
			}),
	})
	/**
	 * Filters the list by the given status and returns to the first page.
	 * @param chosen - The status to filter by, empty for every status.
	 */
	function chooseStatus(chosen: string) {
		setStatus(chosen)
		setNotice(null)
		setSelection([])
		setView((current) => ({ ...current, page: 1 }))
	}
	const page = posts.data ?? EMPTY_PAGE
	return (
		<Page
			title={listed.pluralLabel}
			actions={status === 'trash' ? <EmptyTrash onEmptied={refresh} /> : undefined}
		>
			<StatusRow
				counts={counts.data}
				failed={counts.isError}
				current={status}
				onSelect={chooseStatus}
			/>
			{notice !== null && <PostsNotice notice={notice} report={setNotice} />}
			{posts.isError ? (
				<ErrorNotice>
					{sprintf(__('Could not load %s.', 'gophenberg'), listed.pluralLabel.toLowerCase())}
				</ErrorNotice>
			) : (
				<div
					className="godmin-table-scroll"
					role="region"
					aria-label={listed.pluralLabel}
					tabIndex={0}
				>
					<DataViews
						data={page.items}
						fields={postFields}
						actions={actions}
						view={view}
						onChangeView={setView}
						selection={selection}
						onChangeSelection={setSelection}
						isLoading={posts.isPending}
						getItemId={(post) => post.id}
						searchLabel={sprintf(__('Search %s', 'gophenberg'), listed.pluralLabel.toLowerCase())}
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
