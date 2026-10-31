// SPDX-License-Identifier: Apache-2.0

import '@gophenberg/frontend-sdk/dataviews'
import type { View } from '@gophenberg/frontend-sdk/dataviews'
import { offeredSize } from '@gopherium/godmin'
import { useListView } from '@gopherium/godmin/router'
import { UsersPage, usersListDefaults } from '@gopherium/react-auth/wpds/users'
import { Link } from '@tanstack/react-router'
import { useMemo } from 'react'

import { useListSettings } from '../settings/useListSettings'
import { usePostsColumns } from './postsColumn'
import { roleLabels } from './roles'

/**
 * Renders the account administration screen, the shared users list over the view the address holds.
 * @returns The users screen element.
 */
export function UsersScreen() {
	const settings = useListSettings()
	const sizes = settings.data?.list_page_sizes
	const size = settings.data?.list_page_size
	const columns = usePostsColumns()
	const labels = useMemo(() => roleLabels(), [])
	const list = useListView<View>(usersListDefaults({ perPage: size, columns: columns.map((column) => column.id) }))
	const view = { ...list.view, perPage: offeredSize(list.view.perPage, sizes, size) }
	return (
		<UsersPage
			list={{ ...list, view }}
			pageSizes={sizes}
			sizesFailed={settings.isError}
			showAvatars={settings.data?.show_avatars}
			roleLabels={labels}
			addLink={<Link to="/users/new" />}
			headingLevel={2}
			columns={columns}
		/>
	)
}
