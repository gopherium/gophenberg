// SPDX-License-Identifier: Apache-2.0

import { __ } from '@wordpress/i18n'
import { z } from 'zod'

import { errorText } from '../i18n/errors'

const subSizeSchema = z.object({
	file: z.string(),
	width: z.number(),
	height: z.number(),
	mime_type: z.string(),
	filesize: z.number(),
})

const mediaSchema = z.object({
	id: z.number(),
	type: z.string(),
	file: z.string(),
	title: z.string().optional(),
	alt_text: z.string().optional(),
	caption: z.string().optional(),
	description: z.string().optional(),
	mime_type: z.string(),
	width: z.number().optional(),
	height: z.number().optional(),
	filesize: z.number().optional(),
	sizes: z.record(z.string(), subSizeSchema).optional(),
	author_id: z.string().optional(),
	author_name: z.string().optional(),
	author_avatar: z.string().optional(),
	created_at: z.string().optional(),
	updated_at: z.string().optional(),
})

const pageSchema = z.object({ items: z.array(mediaSchema), total: z.number() })

const listingSchema = pageSchema.extend({ per_page: z.number() })

const errorSchema = z.object({
	error: z.string(),
	code: z.string().optional(),
	meta: z.record(z.string(), z.unknown()).optional(),
})

interface MediaSubSize {
	file: string
	width: number
	height: number
	mimeType: string
	filesize: number
}

export interface MediaItem {
	id: number
	type: string
	file: string
	title: string
	altText: string
	caption: string
	description: string
	mimeType: string
	width: number
	height: number
	filesize: number
	sizes: Record<string, MediaSubSize>
	authorId: string
	authorName: string
	authorAvatar: string
	createdAt: string
	updatedAt: string
}

export interface MediaPage {
	items: MediaItem[]
	total: number
	perPage: number
}

export interface MediaQuery {
	mimes?: string[]
	search?: string
	page?: number
	perPage?: number
	orderBy?: string
	order?: string
	author?: string[]
	authorExclude?: string[]
	before?: string
	after?: string
	modifiedBefore?: string
	modifiedAfter?: string
}

export interface MediaDescriptions {
	title?: string
	altText?: string
	caption?: string
	description?: string
}

export const mediaQueryKey = ['media']

/** The name the server reads each description under. */
const DESCRIPTION_KEYS: [keyof MediaDescriptions, string][] = [
	['title', 'title'],
	['altText', 'alt_text'],
	['caption', 'caption'],
	['description', 'description'],
]

/**
 * Returns the sub-sizes carried by an API row.
 * @param sizes - The sub-sizes as the API sent them.
 * @returns The sub-sizes in the shape screens use.
 */
function toSizes(sizes: z.infer<typeof mediaSchema>['sizes']): Record<string, MediaSubSize> {
	const mapped: Record<string, MediaSubSize> = {}
	for (const [slug, subSize] of Object.entries(sizes ?? {})) {
		mapped[slug] = {
			file: subSize.file,
			width: subSize.width,
			height: subSize.height,
			mimeType: subSize.mime_type,
			filesize: subSize.filesize,
		}
	}
	return mapped
}

/**
 * Returns the descriptions carried by an API row.
 * @param row - The row as the API sent it.
 * @returns The descriptions in the shape screens use.
 */
function toDescriptions(row: z.infer<typeof mediaSchema>): Required<MediaDescriptions> {
	return {
		title: row.title ?? '',
		altText: row.alt_text ?? '',
		caption: row.caption ?? '',
		description: row.description ?? '',
	}
}

/**
 * Returns the measurements carried by an API row.
 * @param row - The row as the API sent it.
 * @returns The measurements in the shape screens use.
 */
function toMeasurements(row: z.infer<typeof mediaSchema>): {
	width: number
	height: number
	filesize: number
} {
	return { width: row.width ?? 0, height: row.height ?? 0, filesize: row.filesize ?? 0 }
}

/**
 * Returns the media item carried by an API row.
 * @param row - The row as the API sent it.
 * @returns The item in the shape screens use.
 */
function toMedia(row: z.infer<typeof mediaSchema>): MediaItem {
	return {
		id: row.id,
		type: row.type,
		file: row.file,
		mimeType: row.mime_type,
		sizes: toSizes(row.sizes),
		authorId: row.author_id ?? '',
		authorName: row.author_name ?? '',
		authorAvatar: row.author_avatar ?? '',
		createdAt: row.created_at ?? '',
		updatedAt: row.updated_at ?? '',
		...toDescriptions(row),
		...toMeasurements(row),
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
 * Returns the address a stored file is served from.
 * @param file - The library relative path of the file.
 * @returns The public address of the file.
 */
export function mediaSrc(file: string): string {
	return `/media/${file}`
}

/**
 * Returns the named parameters a listing request carries, undefined or empty for each one the query leaves out.
 * @param query - The filters, sort and page size to ask for.
 * @returns The parameters by the names the server reads.
 */
function namedParams(query: MediaQuery): Record<string, string | undefined> {
	return {
		per_page: query.perPage?.toString(),
		mime: query.mimes?.join(','),
		search: query.search,
		orderby: query.orderBy,
		order: query.order,
		author: query.author?.join(','),
		author_exclude: query.authorExclude?.join(','),
		before: query.before,
		after: query.after,
		modified_before: query.modifiedBefore,
		modified_after: query.modifiedAfter,
	}
}

/**
 * Returns the query parameters a listing request carries.
 * @param query - The filters, sort, page and page size to ask for.
 * @returns The parameters, the first page left unnamed.
 */
function listingParams(query: MediaQuery): URLSearchParams {
	const params = new URLSearchParams()
	for (const [name, value] of Object.entries(namedParams(query))) {
		if (value) {
			params.set(name, value)
		}
	}
	if (query.page && query.page > 1) {
		params.set('page', String(query.page))
	}
	return params
}

/**
 * Returns one page of media matching the query.
 * @param query - The filters, sort, page and page size to ask for, the server's own page size when it names none.
 * @returns The page, the total number of matches and the page size the server used.
 */
export async function listMedia(query: MediaQuery): Promise<MediaPage> {
	const params = listingParams(query)
	const response = await fetch(`/api/media?${params}`)
	if (!response.ok) {
		throw new Error(`listing media failed with status ${response.status}`)
	}
	const page = listingSchema.parse(await response.json())
	return { items: page.items.map(toMedia), total: page.total, perPage: page.per_page }
}

/**
 * Returns the stored files the identities name, in that order, leaving out any the library no longer holds.
 * @param ids - The identities a field holds.
 * @param sizes - The page sizes the settings name, the largest being one call's worth, none to ask in one call.
 * @returns The files, none when no identity was named.
 */
export async function listMediaByIDs(ids: number[], sizes?: readonly number[]): Promise<MediaItem[]> {
	const perCall = sizes === undefined ? ids.length : Math.max(...sizes)
	const held: MediaItem[] = []
	for (let from = 0; from < ids.length; from += perCall) {
		held.push(...(await namedMedia(ids.slice(from, from + perCall))))
	}
	return held
}

/**
 * Returns the stored files one request may name.
 * @param ids - The identities to ask for, at most one call's worth.
 * @returns The files the library holds among them.
 */
async function namedMedia(ids: number[]): Promise<MediaItem[]> {
	const params = new URLSearchParams({ ids: ids.join(',') })
	const response = await fetch(`/api/media?${params}`)
	if (!response.ok) {
		throw new Error(`listing media by identity failed with status ${response.status}`)
	}
	return pageSchema.parse(await response.json()).items.map(toMedia)
}

/**
 * Returns the answer to a request the server accepted, or throws the refusal in the reader's words.
 * @param url - Where the request goes.
 * @param init - The method, headers and body of the request.
 * @param unreached - The words to throw when no answer came.
 * @returns The answer.
 */
async function answered(url: string, init: RequestInit, unreached: string): Promise<Response> {
	const response = await fetch(url, init).catch(() => null)
	if (response === null) {
		throw new Error(unreached)
	}
	if (!response.ok) {
		throw new Error(await messageFrom(response))
	}
	return response
}

/**
 * Stores an uploaded file in the media library, or throws the reason it was not stored in the reader's words.
 * @param file - The file the administrator chose.
 * @returns The stored item.
 */
export async function uploadMedia(file: File): Promise<MediaItem> {
	const body = new FormData()
	body.append('file', file, file.name)
	const unreached = __('The server could not be reached, so nothing was uploaded.', 'gophenberg')
	return storedItem(await answered('/api/media', { method: 'POST', body }, unreached))
}

/**
 * Returns the item an answer of the API carries, or throws a stand in when the answer cannot be read.
 * @param response - The answer holding one stored item.
 * @returns The stored item.
 */
async function storedItem(response: Response): Promise<MediaItem> {
	const stored = mediaSchema.safeParse(await response.json().catch(() => null))
	if (!stored.success) {
		throw new Error(errorText({ error: '' }))
	}
	return toMedia(stored.data)
}

/**
 * Returns the body of an edit, the version it writes over and each description it changes, under the server's names.
 * @param edits - The descriptions the reader changed.
 * @param version - The updatedAt the edit writes over.
 * @returns The body.
 */
function editBody(edits: MediaDescriptions, version: string): Record<string, string> {
	const body: Record<string, string> = { updated_at: version }
	for (const [field, key] of DESCRIPTION_KEYS) {
		const value = edits[field]
		if (value !== undefined) {
			body[key] = value
		}
	}
	return body
}

/**
 * Writes the descriptions a reader changed over the version read just before the write, so the newest write wins.
 * @param id - The file to write to.
 * @param edits - The descriptions the reader changed, the others left as they are.
 * @param unreached - The words to throw when no answer came.
 * @returns The stored file, or a throw of the reason it was not written in the reader's words.
 */
export async function editMedia(id: number, edits: MediaDescriptions, unreached: string): Promise<MediaItem> {
	const current = await storedItem(await answered(`/api/media/${id}`, {}, unreached))
	const response = await answered(
		`/api/media/${id}`,
		{
			method: 'PATCH',
			headers: { 'content-type': 'application/json' },
			body: JSON.stringify(editBody(edits, current.updatedAt)),
		},
		unreached,
	)
	return storedItem(response)
}

/**
 * Removes a media item and its files for good, or throws the reason it stayed in the reader's words.
 * @param id - The item to delete.
 */
export async function deleteMedia(id: number): Promise<void> {
	const unreached = __('The item could not be permanently deleted.', 'gophenberg')
	await answered(`/api/media/${id}`, { method: 'DELETE' }, unreached)
}
