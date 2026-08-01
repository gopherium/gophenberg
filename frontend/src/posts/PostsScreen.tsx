// SPDX-License-Identifier: Apache-2.0

import { Notice, Stack } from '@gophenberg/frontend-sdk'
import { DataViews } from '@gophenberg/frontend-sdk/dataviews'
import type { View } from '@gophenberg/frontend-sdk/dataviews'
import { useQuery } from '@tanstack/react-query'
import { useCallback, useState } from 'react'

import { usePostActions, useRefresh } from './actions'
import type { PostNotice } from './actions'
import { fetchPostCounts, listPosts } from './api'
import type { PostCounts } from './api'
import { EmptyTrash } from './EmptyTrash'
import { postFields } from './fields'
import { PostsNotice } from './PostsNotice'
import { StatusViews } from './StatusViews'

const PER_PAGE = 20

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
 * Renders the posts list screen.
 * @returns The list screen element.
 */
export function PostsScreen() {
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
	const counts = useQuery({ queryKey: ['post-counts'], queryFn: fetchPostCounts })
	const posts = useQuery({
		queryKey: ['posts', status, view.search, view.page, view.sort],
		queryFn: () =>
			listPosts({
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
	if (posts.isError) {
		return (
			<Notice.Root intent="error" role="alert">
				<Notice.Description>Could not load posts.</Notice.Description>
			</Notice.Root>
		)
	}
	const items = posts.data?.items ?? []
	const total = posts.data?.total ?? 0
	return (
		<Stack direction="column" gap="md">
			<PostsHeader
				counts={counts.data}
				status={status}
				onChoose={chooseStatus}
				onEmptied={refresh}
			/>
			{notice !== null && <PostsNotice notice={notice} report={setNotice} />}
			<DataViews
				data={items}
				fields={postFields}
				actions={actions}
				view={view}
				onChangeView={setView}
				selection={selection}
				onChangeSelection={setSelection}
				isLoading={posts.isPending}
				getItemId={(post) => post.id}
				searchLabel="Search posts"
				config={{ perPageSizes: [PER_PAGE] }}
				paginationInfo={{
					totalItems: total,
					totalPages: Math.max(1, Math.ceil(total / PER_PAGE)),
				}}
				defaultLayouts={{ table: {} }}
			/>
		</Stack>
	)
}

/**
 * Renders the status filter row above the list.
 * @param props - The counts, the current status, and the row handlers.
 * @returns The header row element.
 */
function PostsHeader({
	counts,
	status,
	onChoose,
	onEmptied,
}: {
	counts: PostCounts | undefined
	status: string
	onChoose: (chosen: string) => void
	onEmptied: () => Promise<unknown>
}) {
	return (
		<Stack direction="row" gap="md" align="center" justify="space-between">
			{counts !== undefined && (
				<StatusViews counts={counts} current={status} onSelect={onChoose} />
			)}
			{status === 'trash' && <EmptyTrash onEmptied={onEmptied} />}
		</Stack>
	)
}
