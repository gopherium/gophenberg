// SPDX-License-Identifier: Apache-2.0

import { z } from 'zod'

const POSTS_PER_PAGE = 20

const postSchema = z.object({
	id: z.string(),
	type: z.string(),
	slug: z.string(),
	title: z.string(),
	status: z.string(),
	excerpt: z.string().optional(),
	author_name: z.string().optional(),
	published_at: z.string().nullable().optional(),
	created_at: z.string().optional(),
	updated_at: z.string().optional(),
})

const pageSchema = z.object({ items: z.array(postSchema), total: z.number() })

const countsSchema = z.record(z.string(), z.number())

export interface Post {
	id: string
	type: string
	slug: string
	title: string
	status: string
	excerpt: string
	authorName: string
	publishedAt: string | null
	createdAt: string
	updatedAt: string
}

export interface PostPage {
	items: Post[]
	total: number
}

export interface PostQuery {
	status?: string
	search?: string
	page?: number
	orderBy?: string
	order?: string
}

export type PostCounts = Record<string, number>

/**
 * Returns the post carried by an API row.
 * @param row - The row as the API sent it.
 * @returns The post in the shape screens use.
 */
function toPost(row: z.infer<typeof postSchema>): Post {
	return {
		id: row.id,
		type: row.type,
		slug: row.slug,
		title: row.title,
		status: row.status,
		excerpt: row.excerpt ?? '',
		authorName: row.author_name ?? '',
		publishedAt: row.published_at ?? null,
		createdAt: row.created_at ?? '',
		updatedAt: row.updated_at ?? '',
	}
}

/**
 * Creates a draft of the given type.
 * @param type - The post type to create.
 * @returns The stored draft.
 */
export async function createPost(type = 'post'): Promise<Post> {
	const response = await fetch('/api/posts', {
		method: 'POST',
		headers: { 'content-type': 'application/json' },
		body: JSON.stringify({ type, title: '' }),
	})
	if (!response.ok) {
		throw new Error(`creating a post failed with status ${response.status}`)
	}
	return toPost(postSchema.parse(await response.json()))
}

/**
 * Moves a post to the trash.
 * @param id - The post to trash.
 * @returns The trashed post.
 */
export async function trashPost(id: string): Promise<Post> {
	const response = await fetch(`/api/posts/${id}`, { method: 'DELETE' })
	if (!response.ok) {
		throw new Error(`trashing a post failed with status ${response.status}`)
	}
	return toPost(postSchema.parse(await response.json()))
}

/**
 * Returns a trashed post to draft.
 * @param id - The post to restore.
 * @returns The restored post.
 */
export async function restorePost(id: string): Promise<Post> {
	const response = await fetch(`/api/posts/${id}/restore`, { method: 'POST' })
	if (!response.ok) {
		throw new Error(`restoring a post failed with status ${response.status}`)
	}
	return toPost(postSchema.parse(await response.json()))
}

/**
 * Removes a post for good.
 * @param id - The post to delete.
 */
export async function deletePost(id: string): Promise<void> {
	const response = await fetch(`/api/posts/${id}?force=true`, { method: 'DELETE' })
	if (!response.ok) {
		throw new Error(`deleting a post failed with status ${response.status}`)
	}
}

/**
 * Returns one page of posts matching the query.
 * @param query - The filters, sort and page to ask for.
 * @returns The page and the total number of matches.
 */
export async function listPosts(query: PostQuery): Promise<PostPage> {
	const params = new URLSearchParams({ per_page: String(POSTS_PER_PAGE) })
	if (query.status) {
		params.set('status', query.status)
	}
	if (query.search) {
		params.set('search', query.search)
	}
	if (query.page && query.page > 1) {
		params.set('page', String(query.page))
	}
	if (query.orderBy) {
		params.set('orderby', query.orderBy)
	}
	if (query.order) {
		params.set('order', query.order)
	}
	const response = await fetch(`/api/posts?${params}`)
	if (!response.ok) {
		throw new Error(`listing posts failed with status ${response.status}`)
	}
	const page = pageSchema.parse(await response.json())
	return { items: page.items.map(toPost), total: page.total }
}

/**
 * Returns how many posts hold each status.
 * @returns The count of posts per status.
 */
export async function fetchPostCounts(): Promise<PostCounts> {
	const response = await fetch('/api/posts/counts')
	if (!response.ok) {
		throw new Error(`counting posts failed with status ${response.status}`)
	}
	return countsSchema.parse(await response.json())
}
