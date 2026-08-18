// SPDX-License-Identifier: Apache-2.0

import { z } from 'zod'

import { refusalText } from '../i18n/refusal'

const MEDIA_PER_PAGE = 20

const renditionSchema = z.object({
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
	sizes: z.record(z.string(), renditionSchema).optional(),
	author_id: z.string().optional(),
	created_at: z.string().optional(),
	updated_at: z.string().optional(),
})

const pageSchema = z.object({ items: z.array(mediaSchema), total: z.number() })

const errorSchema = z.object({
	error: z.string(),
	code: z.string().optional(),
	meta: z.record(z.string(), z.unknown()).optional(),
})

interface Rendition {
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
	sizes: Record<string, Rendition>
	authorId: string
	createdAt: string
	updatedAt: string
}

export interface MediaPage {
	items: MediaItem[]
	total: number
}

export interface MediaQuery {
	type?: string
	mimes?: string[]
	search?: string
	page?: number
}

export interface MediaDescriptions {
	title?: string
	altText?: string
	caption?: string
	description?: string
}

export type UploadOutcome =
	| { kind: 'stored', item: MediaItem }
	| { kind: 'refused', reason: string }

export type DescribeOutcome =
	| { kind: 'saved', item: MediaItem }
	| { kind: 'stale' }
	| { kind: 'rejected', message: string }

export const mediaQueryKey = ['media']

/**
 * Returns the renditions carried by an API row.
 * @param sizes - The renditions as the API sent them.
 * @returns The renditions in the shape screens use.
 */
function toSizes(sizes: z.infer<typeof mediaSchema>['sizes']): Record<string, Rendition> {
	const mapped: Record<string, Rendition> = {}
	for (const [slug, size] of Object.entries(sizes ?? {})) {
		mapped[slug] = {
			file: size.file,
			width: size.width,
			height: size.height,
			mimeType: size.mime_type,
			filesize: size.filesize,
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
		return refusalText({ error: '' })
	}
	return refusalText(parsed.data)
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
 * Returns one page of media matching the query.
 * @param query - The filters and page to ask for.
 * @returns The page and the total number of matches.
 */
export async function listMedia(query: MediaQuery): Promise<MediaPage> {
	const params = new URLSearchParams({ per_page: String(MEDIA_PER_PAGE) })
	if (query.type) {
		params.set('type', query.type)
	}
	if (query.mimes && query.mimes.length > 0) {
		params.set('mime', query.mimes.join(','))
	}
	if (query.search) {
		params.set('search', query.search)
	}
	if (query.page && query.page > 1) {
		params.set('page', String(query.page))
	}
	const response = await fetch(`/api/media?${params}`)
	if (!response.ok) {
		throw new Error(`listing media failed with status ${response.status}`)
	}
	const page = pageSchema.parse(await response.json())
	return { items: page.items.map(toMedia), total: page.total }
}

/**
 * Stores an uploaded file in the media library.
 * @param file - The file the administrator chose.
 * @returns The stored item, or the reason it was refused.
 */
export async function uploadMedia(file: File): Promise<UploadOutcome> {
	const body = new FormData()
	body.append('file', file, file.name)
	const response = await fetch('/api/media', { method: 'POST', body })
	if (!response.ok) {
		return { kind: 'refused', reason: await messageFrom(response) }
	}
	return { kind: 'stored', item: toMedia(mediaSchema.parse(await response.json())) }
}

/**
 * Writes the given descriptions to a media item.
 * @param id - The item to write to.
 * @param descriptions - The descriptions to change.
 * @param version - The updatedAt the descriptions were prepared against.
 * @returns What the server made of the write.
 */
export async function describeMedia(
	id: number,
	descriptions: MediaDescriptions,
	version: string,
): Promise<DescribeOutcome> {
	const body: Record<string, string> = { updated_at: version }
	const wire: [keyof MediaDescriptions, string][] = [
		['title', 'title'],
		['altText', 'alt_text'],
		['caption', 'caption'],
		['description', 'description'],
	]
	for (const [field, key] of wire) {
		const value = descriptions[field]
		if (value !== undefined) {
			body[key] = value
		}
	}
	const response = await fetch(`/api/media/${id}`, {
		method: 'PATCH',
		headers: { 'content-type': 'application/json' },
		body: JSON.stringify(body),
	})
	if (response.ok) {
		return { kind: 'saved', item: toMedia(mediaSchema.parse(await response.json())) }
	}
	if (response.status === 409) {
		return { kind: 'stale' }
	}
	return { kind: 'rejected', message: await messageFrom(response) }
}

/**
 * Removes a media item and its files for good.
 * @param id - The item to delete.
 */
export async function deleteMedia(id: number): Promise<void> {
	const response = await fetch(`/api/media/${id}`, { method: 'DELETE' })
	if (!response.ok) {
		throw new Error(`deleting media failed with status ${response.status}`)
	}
}
