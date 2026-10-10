// SPDX-License-Identifier: Apache-2.0

import { errorText as refusalText } from '@gopherium/gottext'
import { __ } from '@wordpress/i18n'
import { z } from 'zod'

import { errorTemplates } from '../i18n/errorTemplates'
import { errorText } from '../i18n/errors'

const POSTS_PER_PAGE = 20

const MAX_POSTS_PER_PAGE = 100

const MAX_LISTING_PAGES = 100

const postSchema = z.object({
	id: z.string(),
	type: z.string(),
	parent_id: z.string().nullable().optional(),
	path: z.string().optional(),
	slug: z.string(),
	title: z.string(),
	status: z.string(),
	excerpt: z.string().optional(),
	author_id: z.string().optional(),
	author_name: z.string().optional(),
	published_at: z.string().nullable().optional(),
	created_at: z.string().optional(),
	updated_at: z.string().optional(),
	fields: z.record(z.string(), z.unknown()).optional(),
})

const detailSchema = postSchema.extend({
	content: z.string(),
	fields: z.record(z.string(), z.unknown()).optional(),
	field_totals: z.record(z.string(), z.number()).optional(),
})

const pageSchema = z.object({ items: z.array(postSchema), total: z.number() })

const errorSchema = z.object({
	error: z.string(),
	code: z.string().optional(),
	meta: z.record(z.string(), z.unknown()).optional(),
})

const countsSchema = z.record(z.string(), z.number())

const emptiedSchema = z.object({ deleted: z.number(), kept: z.number() })

export interface Post {
	id: string
	type: string
	parentId: string | null
	path: string
	slug: string
	title: string
	status: string
	excerpt: string
	authorId: string
	authorName: string
	publishedAt: string | null
	createdAt: string
	updatedAt: string
	fields?: Record<string, unknown>
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
	fields?: Record<string, string>
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
		authorId: row.author_id ?? '',
		authorName: row.author_name ?? '',
		publishedAt: row.published_at ?? null,
		createdAt: row.created_at ?? '',
		updatedAt: row.updated_at ?? '',
		fields: row.fields ?? {},
	}
}

/**
 * Creates a draft of the given type.
 * @param type - The post type to create.
 * @param fields - The values the draft starts with.
 * @returns The stored draft.
 */
export async function createPost(type = 'post', fields: Record<string, unknown> = {}): Promise<Post> {
	const response = await fetch('/api/content', {
		method: 'POST',
		headers: { 'content-type': 'application/json' },
		body: JSON.stringify({ type, title: '', fields }),
	})
	if (!response.ok) {
		throw new Error(`creating a post failed with status ${response.status}`)
	}
	return toPost(postSchema.parse(await response.json()))
}

export interface PostDetail extends Post {
	content: string
	fields: Record<string, unknown>
	fieldTotals: Record<string, number>
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
	return {
		...toPost(row),
		content: row.content,
		fields: row.fields ?? {},
		fieldTotals: row.field_totals ?? {},
	}
}

/**
 * Returns the message an error response carried.
 * @param response - The response the API refused with.
 * @returns The message, or a stand in when the body carries none.
 */
async function messageFrom(response: Response): Promise<string> {
	const parsed = errorSchema.safeParse(await response.json().catch(() => null))
	if (!parsed.success) {
		return errorText({ error: '' })
	}
	return errorText(parsed.data)
}

/**
 * Returns the reason a refused answer names in the reader's language, never the server's own prose.
 * @param response - The response the API refused with.
 * @param fallback - The words to show when the answer names no reason the admin knows.
 * @returns The translated reason, or the fallback.
 */
async function reasonOf(response: Response, fallback: string): Promise<string> {
	const parsed = errorSchema.safeParse(await response.json().catch(() => null))
	const named = parsed.success ? { code: parsed.data.code, meta: parsed.data.meta } : {}
	return refusalText({ message: '', ...named }, errorTemplates(), fallback)
}

/**
 * Returns the answer to a write the server accepted, or throws the refusal in the reader's language.
 * @param url - Where the write goes.
 * @param method - The method the write uses.
 * @param fallback - The words to show when the answer names no reason the admin knows, or no answer came.
 * @returns The answer.
 */
async function accepted(url: string, method: string, fallback: string): Promise<Response> {
	const response = await fetch(url, { method }).catch(() => null)
	if (response === null) {
		throw new Error(fallback)
	}
	if (!response.ok) {
		throw new Error(await reasonOf(response, fallback))
	}
	return response
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

export interface AutosaveOutcome extends Autosave {
	target: string
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
 * Returns the autosave an answer describes.
 * @param row - The answer as the server wrote it.
 * @returns The autosave.
 */
function toAutosave(row: z.infer<typeof autosaveSchema>): Autosave {
	return {
		title: row.title,
		content: row.content,
		excerpt: row.excerpt,
		fields: row.fields ?? {},
		savedAt: row.saved_at,
	}
}

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
	return toAutosave(autosaveSchema.parse(await response.json()))
}

/**
 * Parks the given buffer as the caller's autosave of a post.
 * @param id - The post the buffer belongs to.
 * @param buffer - The words to park.
 * @param version - The updatedAt the buffer was prepared against.
 * @param keepalive - Whether the request should outlive the page.
 * @returns Where the buffer landed, the words it holds there and the time the server stamped it.
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
	return { ...toAutosave(row), target: row.target }
}

/**
 * Moves a post to the trash.
 * @param id - The post to trash.
 * @returns The trashed post.
 */
export async function trashPost(id: string): Promise<Post> {
	const fallback = __('The item could not be moved to the trash.', 'gophenberg')
	const response = await accepted(`/api/content/${id}`, 'DELETE', fallback)
	return toPost(postSchema.parse(await response.json()))
}

/**
 * Returns a trashed post to draft.
 * @param id - The post to restore.
 * @returns The restored post.
 */
export async function restorePost(id: string): Promise<Post> {
	const fallback = __('The item could not be restored.', 'gophenberg')
	const response = await accepted(`/api/content/${id}/restore`, 'POST', fallback)
	return toPost(postSchema.parse(await response.json()))
}

/**
 * Removes a post for good.
 * @param id - The post to delete.
 */
export async function deletePost(id: string): Promise<void> {
	const fallback = __('The item could not be permanently deleted.', 'gophenberg')
	await accepted(`/api/content/${id}?force=true`, 'DELETE', fallback)
}

/**
 * Returns the query parameters a listing request carries.
 * @param query - The filters, sort and page to ask for.
 * @returns The parameters.
 */
function listingParams(query: PostQuery): URLSearchParams {
	const params = new URLSearchParams({ per_page: String(query.perPage ?? POSTS_PER_PAGE) })
	const named: Record<string, string | undefined> = {
		type: query.type,
		status: query.status,
		search: query.search,
		orderby: query.orderBy,
		order: query.order,
	}
	for (const [name, value] of Object.entries(named)) {
		if (value) {
			params.set(name, value)
		}
	}
	if (query.page && query.page > 1) {
		params.set('page', String(query.page))
	}
	for (const [key, value] of Object.entries(query.fields ?? {})) {
		params.set(`field[${key}]`, value)
	}
	return params
}

/**
 * Returns one page of posts matching the query.
 * @param query - The filters, sort and page to ask for.
 * @returns The page and the total number of matches.
 */
export async function listPosts(query: PostQuery): Promise<PostPage> {
	const params = listingParams(query)
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

/** How many items emptying a type's trash deleted for good and how many it left there. */
export type Emptied = z.infer<typeof emptiedSchema>

/**
 * Removes for good every trashed item of a type the session may change.
 * @param type - The content type whose trash to empty.
 * @returns How many items went and how many stayed.
 */
export async function emptyTrash(type: string): Promise<Emptied> {
	const fallback = __('The trash could not be emptied.', 'gophenberg')
	const response = await accepted(`/api/content/trash?${new URLSearchParams({ type })}`, 'DELETE', fallback)
	return emptiedSchema.parse(await response.json())
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
