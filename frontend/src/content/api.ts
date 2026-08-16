// SPDX-License-Identifier: Apache-2.0

import { z } from 'zod'

import { refusalText } from '../i18n/refusal'

const POSTS_PER_PAGE = 20

const MAX_POSTS_PER_PAGE = 100

const MAX_LISTING_PAGES = 100

const MAX_EMPTY_ROUNDS = 50

const postSchema = z.object({
	id: z.string(),
	type: z.string(),
	parent_id: z.string().nullable().optional(),
	path: z.string().optional(),
	slug: z.string(),
	title: z.string(),
	status: z.string(),
	excerpt: z.string().optional(),
	author_name: z.string().optional(),
	published_at: z.string().nullable().optional(),
	created_at: z.string().optional(),
	updated_at: z.string().optional(),
})

const detailSchema = postSchema.extend({
	content: z.string(),
	fields: z.record(z.string(), z.unknown()).optional(),
})

const pageSchema = z.object({ items: z.array(postSchema), total: z.number() })

const errorSchema = z.object({
	error: z.string(),
	code: z.string().optional(),
	meta: z.record(z.string(), z.unknown()).optional(),
})

const countsSchema = z.record(z.string(), z.number())

export interface Post {
	id: string
	type: string
	parentId: string | null
	path: string
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
	type?: string
	status?: string
	search?: string
	page?: number
	perPage?: number
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
		parentId: row.parent_id ?? null,
		path: row.path ?? '',
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
	const response = await fetch('/api/content', {
		method: 'POST',
		headers: { 'content-type': 'application/json' },
		body: JSON.stringify({ type, title: '' }),
	})
	if (!response.ok) {
		throw new Error(`creating a post failed with status ${response.status}`)
	}
	return toPost(postSchema.parse(await response.json()))
}

export interface PostDetail extends Post {
	content: string
	fields: Record<string, unknown>
}

export interface PostChanges {
	title?: string
	content?: string
	excerpt?: string
	slug?: string
	status?: string
	parent_id?: string | null
	fields?: Record<string, unknown>
}

export type SaveOutcome =
	| { kind: 'saved', post: PostDetail }
	| { kind: 'conflict', current: PostDetail }
	| { kind: 'rejected', message: string }

/**
 * Returns the post carried by an API detail row.
 * @param row - The row as the API sent it.
 * @returns The post with its content.
 */
function toDetail(row: z.infer<typeof detailSchema>): PostDetail {
	return { ...toPost(row), content: row.content, fields: row.fields ?? {} }
}

/**
 * Returns the message an error response carried.
 * @param response - The response the API refused with.
 * @returns The message, or a stand in when the body carries none.
 */
async function messageFrom(response: Response): Promise<string> {
	const parsed = errorSchema.safeParse(await response.json().catch(() => null))
	if (!parsed.success) {
		return refusalText({ error: '' })
	}
	return refusalText(parsed.data)
}

/**
 * Returns one post with its content.
 * @param id - The post to read.
 * @returns The stored post.
 */
export async function fetchPost(id: string): Promise<PostDetail> {
	const response = await fetch(`/api/content/${id}`)
	if (!response.ok) {
		throw new Error(`reading a post failed with status ${response.status}`)
	}
	return toDetail(detailSchema.parse(await response.json()))
}

/**
 * Writes the given changes to a post.
 * @param id - The post to write to.
 * @param changes - The fields to change.
 * @param version - The updatedAt the changes were prepared against.
 * @returns What the server made of the write.
 */
export async function savePost(
	id: string,
	changes: PostChanges,
	version: string,
): Promise<SaveOutcome> {
	const response = await fetch(`/api/content/${id}`, {
		method: 'PATCH',
		headers: { 'content-type': 'application/json' },
		body: JSON.stringify({ ...changes, updated_at: version }),
	})
	if (response.ok) {
		return { kind: 'saved', post: toDetail(detailSchema.parse(await response.json())) }
	}
	if (response.status === 409) {
		return { kind: 'conflict', current: await fetchPost(id) }
	}
	return { kind: 'rejected', message: await messageFrom(response) }
}

export interface AutosaveBuffer {
	title: string
	content: string
	excerpt: string
	fields: Record<string, unknown>
}

export interface Autosave extends AutosaveBuffer {
	savedAt: string
}

export interface AutosaveOutcome {
	target: string
	savedAt: string
}

const autosaveSchema = z.object({
	target: z.string(),
	title: z.string(),
	content: z.string(),
	excerpt: z.string(),
	fields: z.record(z.string(), z.unknown()).optional(),
	saved_at: z.string(),
})

/**
 * Returns the autosave the server holds for a post.
 * @param id - The post to read the autosave of.
 * @returns The autosave, or nothing when the server holds none.
 */
export async function fetchAutosave(id: string): Promise<Autosave | null> {
	const response = await fetch(`/api/content/${id}/autosave`)
	if (!response.ok) {
		return null
	}
	const row = autosaveSchema.parse(await response.json())
	return {
		title: row.title,
		content: row.content,
		excerpt: row.excerpt,
		fields: row.fields ?? {},
		savedAt: row.saved_at,
	}
}

/**
 * Parks the given buffer as the caller's autosave of a post.
 * @param id - The post the buffer belongs to.
 * @param buffer - The words to park.
 * @param version - The updatedAt the buffer was prepared against.
 * @param keepalive - Whether the request should outlive the page.
 * @returns Where the buffer landed and the time the server stamped it.
 */
export async function autosavePost(
	id: string,
	buffer: AutosaveBuffer,
	version: string,
	keepalive = false,
): Promise<AutosaveOutcome> {
	const response = await fetch(`/api/content/${id}/autosave`, {
		method: 'POST',
		headers: { 'content-type': 'application/json' },
		body: JSON.stringify({ ...buffer, updated_at: version }),
		keepalive,
	})
	if (!response.ok) {
		throw new Error(`autosaving a post failed with status ${response.status}`)
	}
	const row = autosaveSchema.parse(await response.json())
	return { target: row.target, savedAt: row.saved_at }
}

/**
 * Moves a post to the trash.
 * @param id - The post to trash.
 * @returns The trashed post.
 */
export async function trashPost(id: string): Promise<Post> {
	const response = await fetch(`/api/content/${id}`, { method: 'DELETE' })
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
	const response = await fetch(`/api/content/${id}/restore`, { method: 'POST' })
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
	const response = await fetch(`/api/content/${id}?force=true`, { method: 'DELETE' })
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
	const params = new URLSearchParams({ per_page: String(query.perPage ?? POSTS_PER_PAGE) })
	if (query.type) {
		params.set('type', query.type)
	}
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
	const response = await fetch(`/api/content?${params}`)
	if (!response.ok) {
		throw new Error(`listing posts failed with status ${response.status}`)
	}
	const page = pageSchema.parse(await response.json())
	return { items: page.items.map(toPost), total: page.total }
}

/**
 * Returns the posts the query names, asking page after page up to the reading ceiling.
 * @param query - The listing to read, without paging.
 * @returns The posts the listing holds, up to the pages the ceiling allows.
 */
export async function listEveryPost(query: PostQuery): Promise<Post[]> {
	const held: Post[] = []
	for (let page = 1; page <= MAX_LISTING_PAGES; page += 1) {
		const read = await listPosts({ ...query, page, perPage: MAX_POSTS_PER_PAGE })
		held.push(...read.items)
		if (held.length >= read.total || read.items.length === 0) {
			return held
		}
	}
	return held
}

/**
 * Removes every trashed post for good.
 */
export async function emptyTrash(): Promise<void> {
	for (let round = 0; round < MAX_EMPTY_ROUNDS; round += 1) {
		const page = await listPosts({ status: 'trash' })
		if (page.items.length === 0) {
			return
		}
		await Promise.all(page.items.map((post) => deletePost(post.id)))
	}
	throw new Error('emptying the trash did not finish')
}

/**
 * Returns how many posts of a type hold each status.
 * @param type - The content type to count, the default type when absent.
 * @returns The count of posts per status.
 */
export async function fetchPostCounts(type?: string): Promise<PostCounts> {
	const params = new URLSearchParams(type ? { type } : {})
	const response = await fetch(`/api/content/counts?${params}`)
	if (!response.ok) {
		throw new Error(`counting posts failed with status ${response.status}`)
	}
	return countsSchema.parse(await response.json())
}
