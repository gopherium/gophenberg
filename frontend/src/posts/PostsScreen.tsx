// SPDX-License-Identifier: Apache-2.0

import { Notice, Stack } from '@gophenberg/frontend-sdk'
import { DataViews } from '@gophenberg/frontend-sdk/dataviews'
import type { View } from '@gophenberg/frontend-sdk/dataviews'
import { useQuery } from '@tanstack/react-query'
import { useState } from 'react'

import { usePostActions, useRefresh } from './actions'
import type { PostNotice } from './actions'
import { fetchPostCounts, listPosts } from './api'
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
	const actions = usePostActions(status, setNotice)
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
		setView((current) => ({ ...current, page: 1 }))
	}
	if (posts.isError) {
		return (
			<Notice.Root intent="error" role="alert">
				<Notice.Description>Could not load posts.</Notice.Description>
			</Notice.Root>
		)
	}
	return (
		<Stack direction="column" gap="md">
			<Stack direction="row" gap="md" align="center" justify="space-between">
				{counts.data !== undefined && (
					<StatusViews counts={counts.data} current={status} onSelect={chooseStatus} />
				)}
				{status === 'trash' && <EmptyTrash onEmptied={refresh} />}
			</Stack>
			{notice !== null && <PostsNotice notice={notice} report={setNotice} />}
			<DataViews
				data={posts.data?.items ?? []}
				fields={postFields}
				actions={actions}
				view={view}
				onChangeView={setView}
				isLoading={posts.isPending}
				getItemId={(post) => post.id}
				searchLabel="Search posts"
				config={{ perPageSizes: [PER_PAGE] }}
				paginationInfo={{
					totalItems: posts.data?.total ?? 0,
					totalPages: Math.max(1, Math.ceil((posts.data?.total ?? 0) / PER_PAGE)),
				}}
				defaultLayouts={{ table: {} }}
			/>
		</Stack>
	)
}
