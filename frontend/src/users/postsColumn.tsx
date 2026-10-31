// SPDX-License-Identifier: Apache-2.0

import { Link } from '@gophenberg/frontend-sdk'
import { formatNumber } from '@gopherium/gottext'
import { fetchUsers, usersQueryKey } from '@gopherium/react-auth/admin'
import type { UsersColumn, UsersColumnCell } from '@gopherium/react-auth/wpds/users'
import { useQuery } from '@tanstack/react-query'
import { Link as RouterLink } from '@tanstack/react-router'
import { _n, sprintf } from '@wordpress/i18n'
import { useMemo } from 'react'
import { z } from 'zod'

import { typesQueryKey } from '../content/nav'
import { pluralOf } from '../content/typeText'
import { listTypes } from '../content/types'
import type { ContentType } from '../content/types'

/** The key of the type whose items the Posts column counts, the type WordPress counts in its Users list. */
const POST_TYPE = 'post'

/** The query naming how many posts each account wrote. */
const postCountsQueryKey = ['user-post-counts'] as const

/** How many posts each account wrote, as the server answers it. */
const postCountsSchema = z.object({ counts: z.record(z.string(), z.number()) })

/** The columns of a list whose site holds no active post type. */
const NO_COLUMNS: readonly UsersColumn[] = []

/**
 * Returns how many published and private posts each account wrote, raising when they cannot be read.
 * @param signal - Aborts the in-flight request.
 * @returns The count of each account that wrote one, keyed by account id.
 */
async function fetchPostCounts(signal?: AbortSignal): Promise<Record<string, number>> {
	const response = await fetch('/api/users/post-counts', { signal })
	if (!response.ok) {
		throw new Error(`counting posts failed with status ${response.status}`)
	}
	return postCountsSchema.parse(await response.json()).counts
}

/**
 * Returns the Posts cell of one account, a link to its posts when it wrote any.
 * @param posts - The type the count holds items of.
 * @param id - The account id.
 * @param count - How many posts the account wrote.
 * @returns The cell, a plain 0 for an account that wrote none, as WordPress draws it.
 */
function postsCell(posts: ContentType, id: string, count: number): UsersColumnCell {
	const text = formatNumber(count)
	if (count === 0) {
		return { text }
	}
	return {
		text,
		hidden: sprintf(_n('%s post by this author', '%s posts by this author', count, 'gophenberg'), text),
		link: (
			<Link
				render={
					<RouterLink
						to="/content/$typeKey"
						params={{ typeKey: posts.key }}
						search={{ filters: [{ field: 'author', operator: 'isAny', value: [id] }] }}
					/>
				}
			/>
		),
	}
}

/**
 * Returns the Posts column of the users list, named after the post type and linking each count to that author's posts.
 * @returns The column, none while the registry holds no active post type, its cells empty until the counts arrive.
 */
export function usePostsColumns(): readonly UsersColumn[] {
	const types = useQuery({ queryKey: typesQueryKey, queryFn: listTypes })
	const accounts = useQuery({ queryKey: usersQueryKey, queryFn: ({ signal }) => fetchUsers(signal) })
	const counts = useQuery({ queryKey: postCountsQueryKey, queryFn: ({ signal }) => fetchPostCounts(signal) })
	const posts = types.data?.find((held) => held.key === POST_TYPE && held.active)
	return useMemo(() => {
		if (posts === undefined) {
			return NO_COLUMNS
		}
		const held = counts.data
		const cells =
			held === undefined
				? {}
				: Object.fromEntries(
					(accounts.data ?? []).map((account) => [account.id, postsCell(posts, account.id, held[account.id] ?? 0)]),
				)
		return [{ id: 'posts', label: pluralOf(posts), cells }]
	}, [posts, accounts.data, counts.data])
}
