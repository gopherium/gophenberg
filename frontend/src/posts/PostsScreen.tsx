// SPDX-License-Identifier: Apache-2.0

import { Notice } from '@gophenberg/frontend-sdk'
import { DataViews } from '@gophenberg/frontend-sdk/dataviews'
import type { View } from '@gophenberg/frontend-sdk/dataviews'
import { useQuery } from '@tanstack/react-query'
import { useState } from 'react'

import { listPosts } from './api'
import { postFields } from './fields'

const PER_PAGE = 20

const INITIAL_VIEW: View = {
	type: 'table',
	fields: ['author', 'date'],
	titleField: 'title',
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
	const posts = useQuery({
		queryKey: ['posts', view.page, view.sort],
		queryFn: () =>
			listPosts({
				page: view.page,
				orderBy: view.sort?.field,
				order: view.sort?.direction,
			}),
	})
	if (posts.isError) {
		return (
			<Notice.Root intent="error" role="alert">
				<Notice.Description>Could not load posts.</Notice.Description>
			</Notice.Root>
		)
	}
	return (
		<DataViews
			data={posts.data?.items ?? []}
			fields={postFields}
			view={view}
			onChangeView={setView}
			isLoading={posts.isPending}
			getItemId={(post) => post.id}
			paginationInfo={{
				totalItems: posts.data?.total ?? 0,
				totalPages: Math.max(1, Math.ceil((posts.data?.total ?? 0) / PER_PAGE)),
			}}
			defaultLayouts={{ table: {} }}
		/>
	)
}
