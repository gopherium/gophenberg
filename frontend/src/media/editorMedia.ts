// SPDX-License-Identifier: Apache-2.0

import { mediaSrc, uploadMedia } from './api'
import type { MediaItem } from './api'
import { displayFile } from './format'

// MAX_UPLOAD_BYTES mirrors the cap the server enforces on one upload.
export const MAX_UPLOAD_BYTES = 128 * 1024 * 1024

// ALLOWED_MIME_TYPES mirrors the upload types the server accepts, keyed by extension.
export const ALLOWED_MIME_TYPES: Record<string, string> = {
	'jpg|jpeg': 'image/jpeg',
	png: 'image/png',
	gif: 'image/gif',
	webp: 'image/webp',
	mp4: 'video/mp4',
	webm: 'video/webm',
	mp3: 'audio/mpeg',
	m4a: 'audio/mp4',
	wav: 'audio/wav',
	ogg: 'audio/ogg',
	flac: 'audio/flac',
	pdf: 'application/pdf',
	zip: 'application/zip',
}

// IMAGE_SIZES names the renditions the editor offers, in the server's slugs.
export const IMAGE_SIZES = [
	{ slug: 'thumbnail', name: 'Thumbnail' },
	{ slug: 'medium', name: 'Medium' },
	{ slug: 'large', name: 'Large' },
	{ slug: 'full', name: 'Full Size' },
]

export interface EditorAttachment {
	id?: number
	url: string
	alt?: string
	caption?: string
	title?: string
	link?: string
	mime_type?: string
	media_type?: string
	media_details?: {
		width: number
		height: number
		sizes: Record<string, { source_url: string, width: number, height: number }>
	}
}

export interface EditorMediaUploadArgs {
	filesList: File[] | FileList
	allowedTypes?: string[]
	additionalData?: Record<string, unknown>
	maxUploadFileSize?: number
	multiple?: boolean
	onFileChange?: (attachments: EditorAttachment[]) => void
	onError?: (error: Error) => void
}

/**
 * Returns a stored media item in the shape blocks read.
 * @param item - The item the API answered.
 * @returns The attachment the block editor places.
 */
export function toAttachment(item: MediaItem): EditorAttachment {
	const sizes: Record<string, { source_url: string, width: number, height: number }> = {}
	for (const [slug, rendition] of Object.entries(item.sizes)) {
		sizes[slug] = {
			source_url: mediaSrc(rendition.file),
			width: rendition.width,
			height: rendition.height,
		}
	}
	return {
		id: item.id,
		url: mediaSrc(displayFile(item)),
		alt: item.altText,
		caption: item.caption,
		title: item.title,
		link: mediaSrc(displayFile(item)),
		mime_type: item.mimeType,
		media_type: item.type,
		media_details: { width: item.width, height: item.height, sizes },
	}
}

/**
 * Returns why a file may not be uploaded, empty when it may.
 * @param file - The file the block was handed.
 * @param args - The rules the block passed along.
 * @returns The refusal to report, empty for an acceptable file.
 */
function refusalOf(file: File, args: EditorMediaUploadArgs): string {
	if (args.allowedTypes !== undefined && !typeAllowed(file.type, args.allowedTypes)) {
		return `${file.name} is not a file this block accepts.`
	}
	if (args.maxUploadFileSize !== undefined && file.size > args.maxUploadFileSize) {
		return `${file.name} is larger than one upload may be.`
	}
	return ''
}

/**
 * Reports whether a content type satisfies one of the allowed entries.
 * @param mime - The content type of the file.
 * @param allowed - Full types or type families a block accepts.
 * @returns Whether the file may land in the block.
 */
function typeAllowed(mime: string, allowed: string[]): boolean {
	return allowed.some((entry) =>
		entry.includes('/') ? entry === mime : mime.startsWith(`${entry}/`),
	)
}

/**
 * Uploads one file and returns the placed attachment.
 * @param file - The file to upload.
 * @param args - The handlers the block passed along.
 * @returns The stored attachment, or null when the upload failed.
 */
async function storedAttachment(
	file: File,
	args: EditorMediaUploadArgs,
): Promise<EditorAttachment | null> {
	try {
		const outcome = await uploadMedia(file)
		if (outcome.kind === 'stored') {
			return toAttachment(outcome.item)
		}
		args.onError?.(new Error(outcome.reason))
	} catch {
		args.onError?.(new Error('The server could not be reached, so nothing was uploaded.'))
	}
	return null
}

/**
 * Reports the attachments placed so far to the block.
 * @param slots - One slot per accepted file, null where an upload failed.
 * @param args - The handlers the block passed along.
 */
function report(slots: (EditorAttachment | null)[], args: EditorMediaUploadArgs) {
	args.onFileChange?.(slots.filter((slot): slot is EditorAttachment => slot !== null))
}

/**
 * Uploads the block editor's files, previewing each before it lands.
 * @param args - The files, rules and handlers the block editor passes.
 */
export async function editorMediaUpload(args: EditorMediaUploadArgs): Promise<void> {
	const files = Array.from(args.filesList)
	if (args.multiple === false && files.length > 1) {
		args.onError?.(new Error('Only one file may be placed here.'))
		return
	}
	const accepted: File[] = []
	for (const file of files) {
		const refusal = refusalOf(file, args)
		if (refusal === '') {
			accepted.push(file)
		} else {
			args.onError?.(new Error(refusal))
		}
	}
	const slots: (EditorAttachment | null)[] = accepted.map((file) => ({
		url: URL.createObjectURL(file),
	}))
	report(slots, args)
	await Promise.all(
		accepted.map(async (file, index) => {
			const placed = await storedAttachment(file, args)
			URL.revokeObjectURL((slots[index] as EditorAttachment).url)
			slots[index] = placed
			report(slots, args)
		}),
	)
}
