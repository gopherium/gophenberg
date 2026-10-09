// SPDX-License-Identifier: Apache-2.0

import { errorText as refusalText } from '@gopherium/gottext'
import { __ } from '@wordpress/i18n'
import { z } from 'zod'

import { errorTemplates } from '../i18n/errorTemplates'
import { errorText } from '../i18n/errors'

const postSchema = z.object({
	id: z.string(),
	type: z.string(),
	parent_id: z.string().nullable().optional(),
	parent_title: z.string().optional(),
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

const pageSchema = z.object({ items: z.array(postSchema), total: z.number(), per_page: z.number().optional() })

const errorSchema = z.object({
	error: z.string(),
	code: z.string().optional(),
	meta: z.record(z.string(), z.unknown()).optional(),
})

const emptiedSchema = z.object({ deleted: z.number(), kept: z.number() })

export interface Post {
	id: string
	type: string
	parentId: string | null
	parentTitle: string
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
	date: string
	fields?: Record<string, unknown>
}

export interface PostPage {
	items: Post[]
	total: number
	perPage: number
}

export interface PostQuery {
	type?: string
	status?: string
	search?: string
	page?: number
	perPage?: number
	orderBy?: string
	order?: string
	orderHierarchy?: boolean
	fields?: Record<string, string>
	author?: string[]
	authorExclude?: string[]
	before?: string
	after?: string
}

/**
 * Returns the moments an API row carries.
 * @param row - The row as the API sent it.
 * @returns The moments, the list date being the one the server sorts by: published, else last saved.
 */
function momentsOf(row: z.infer<typeof postSchema>): Pick<Post, 'publishedAt' | 'createdAt' | 'updatedAt' | 'date'> {
	return {
		publishedAt: row.published_at ?? null,
		createdAt: row.created_at ?? '',
		updatedAt: row.updated_at ?? '',
		date: row.published_at ?? row.updated_at ?? '',
	}
}

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
		parentTitle: row.parent_title ?? '',
		path: row.path ?? '',
		slug: row.slug,
		title: row.title,
		status: row.status,
		excerpt: row.excerpt ?? '',
		authorId: row.author_id ?? '',
		authorName: row.author_name ?? '',
		...momentsOf(row),
		fields: row.fields ?? {},
	}
}

/** What a new draft starts with, each part it leaves out left to the server. */
export interface PostDraft {
	title?: string
	content?: string
	excerpt?: string
	parentId?: string
	fields?: Record<string, unknown>
}

/**
 * Creates a draft of the given type, sending only the parts the draft names.
 * @param type - The post type to create.
 * @param draft - What the draft starts with.
 * @param fallback - The words a refused create throws when the server names no reason the admin knows.
 * @returns The stored draft.
 */
export async function createPost(type: string, draft: PostDraft, fallback: string): Promise<Post> {
	const body = {
		type,
		title: draft.title,
		content: draft.content,
		excerpt: draft.excerpt,
		parent_id: draft.parentId,
		fields: draft.fields,
	}
	const response = await accepted(
		'/api/content',
		{ method: 'POST', headers: { 'content-type': 'application/json' }, body: JSON.stringify(body) },
		fallback,
	)
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
 * Returns the answer to a request the server accepted, or throws the refusal in the reader's language.
 * @param url - Where the request goes.
 * @param init - The method, headers and body of the request.
 * @param fallback - The words to show when the answer names no reason the admin knows, when no answer came, or
 *   when another save got there first, which only the caller can word.
 * @returns The answer.
 */
async function accepted(url: string, init: RequestInit, fallback: string): Promise<Response> {
	const response = await fetch(url, init).catch(() => null)
	if (response === null || response.status === 409) {
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
 * @param fallback - The words a failed read throws when the server names no reason the admin knows.
 * @returns The stored post.
 */
export async function fetchPost(id: string, fallback = ''): Promise<PostDetail> {
	const response = await accepted(`/api/content/${id}`, {}, fallback)
	return toDetail(detailSchema.parse(await response.json()))
}

/**
 * Writes a new title over the version of an item read just before the write, so the newest name wins.
 * @param id - The item to rename.
 * @param title - The new title.
 * @returns The renamed item.
 */
export async function renamePost(id: string, title: string): Promise<Post> {
	const fallback = __('The name could not be updated.', 'gophenberg')
	const current = await fetchPost(id, fallback)
	const response = await accepted(
		`/api/content/${id}`,
		{
			method: 'PATCH',
			headers: { 'content-type': 'application/json' },
			body: JSON.stringify({ title, updated_at: current.updatedAt }),
		},
		fallback,
	)
	return toPost(postSchema.parse(await response.json()))
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
	const response = await accepted(`/api/content/${id}`, { method: 'DELETE' }, fallback)
	return toPost(postSchema.parse(await response.json()))
}

/**
 * Returns a trashed post to draft.
 * @param id - The post to restore.
 * @returns The restored post.
 */
export async function restorePost(id: string): Promise<Post> {
	const fallback = __('The item could not be restored.', 'gophenberg')
	const response = await accepted(`/api/content/${id}/restore`, { method: 'POST' }, fallback)
	return toPost(postSchema.parse(await response.json()))
}

/**
 * Removes a post for good.
 * @param id - The post to delete.
 */
export async function deletePost(id: string): Promise<void> {
	const fallback = __('The item could not be permanently deleted.', 'gophenberg')
	await accepted(`/api/content/${id}?force=true`, { method: 'DELETE' }, fallback)
}

/**
 * Returns the named parameters a listing request carries, undefined for each one the query leaves out.
 * @param query - The filters, sort and page to ask for.
 * @returns The parameters by name.
 */
function namedParams(query: PostQuery): Record<string, string | undefined> {
	return {
		per_page: query.perPage?.toString(),
		type: query.type,
		status: query.status,
		search: query.search,
		orderby: query.orderBy,
		order: query.order,
		author: query.author?.join(','),
		author_exclude: query.authorExclude?.join(','),
		before: query.before,
		after: query.after,
	}
}

/**
 * Returns the query parameters a listing request carries.
 * @param query - The filters, sort and page to ask for.
 * @returns The parameters.
 */
function listingParams(query: PostQuery): URLSearchParams {
	const params = new URLSearchParams()
	for (const [name, value] of Object.entries(namedParams(query))) {
		if (value) {
			params.set(name, value)
		}
	}
	if (query.page && query.page > 1) {
		params.set('page', String(query.page))
	}
	if (query.orderHierarchy) {
		params.set('orderby_hierarchy', 'true')
	}
	for (const [key, value] of Object.entries(query.fields ?? {})) {
		params.set(`field[${key}]`, value)
	}
	return params
}

/**
 * Returns one page of posts matching the query.
 * @param query - The filters, sort and page to ask for, the server's own page size when it names none.
 * @returns The page, the total number of matches and the page size the server used, 0 when it names none.
 */
export async function listPosts(query: PostQuery): Promise<PostPage> {
	const params = listingParams(query)
	const response = await fetch(`/api/content?${params}`)
	if (!response.ok) {
		throw new Error(`listing posts failed with status ${response.status}`)
	}
	const page = pageSchema.parse(await response.json())
	return { items: page.items.map(toPost), total: page.total, perPage: page.per_page ?? 0 }
}

/**
 * Returns the posts the query names, asking page after page no further than the total of the first answer names.
 * @param query - The listing to read, without paging.
 * @param sizes - The page sizes the settings name, the largest asked for, none to take the server's own size.
 * @returns The posts the listing holds, the ones read before a page came back empty.
 */
export async function listEveryPost(query: PostQuery, sizes?: readonly number[]): Promise<Post[]> {
	const perPage = sizes === undefined ? undefined : Math.max(...sizes)
	const first = await listPosts({ ...query, perPage })
	const held = [...first.items]
	const pages = first.perPage > 0 ? Math.ceil(first.total / first.perPage) : 1
	for (let page = 2; page <= pages; page += 1) {
		const read = await listPosts({ ...query, page, perPage })
		if (read.items.length === 0) {
			return held
		}
		held.push(...read.items)
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
	const response = await accepted(`/api/content/trash?${new URLSearchParams({ type })}`, { method: 'DELETE' }, fallback)
	return emptiedSchema.parse(await response.json())
}
