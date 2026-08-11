// SPDX-License-Identifier: Apache-2.0

import type {
	InserterMediaCategory,
	InserterMediaItem,
	InserterMediaRequest,
} from '@gophenberg/frontend-sdk/editor'

import { listMedia, mediaSrc } from './api'
import type { MediaItem } from './api'
import { bestRendition } from './format'

// previewWidth is the width the inserter draws a media preview at.
const previewWidth = 300

/**
 * Returns a stored media item in the shape the inserter reads.
 * @param item - The item the library holds.
 * @returns The item the inserter previews and places.
 */
export function toInserterItem(item: MediaItem): InserterMediaItem {
	return {
		id: item.id,
		url: mediaSrc(item.file),
		alt: item.altText,
		caption: item.caption,
		title: item.title,
		previewUrl: mediaSrc(bestRendition(item, previewWidth)),
	}
}

/**
 * Returns the handler listing one kind of stored media for the inserter.
 * @param kind - The content type family the category offers.
 * @returns The fetch the inserter calls.
 */
function storedMediaOf(kind: string) {
	return async (query: InserterMediaRequest): Promise<InserterMediaItem[]> => {
		const page = await listMedia({ mime: kind, search: query.search, page: query.page })
		return page.items.map(toInserterItem)
	}
}

export const MEDIA_CATEGORIES: InserterMediaCategory[] = [
	{
		name: 'gophenberg-images',
		labels: { name: 'Images', search_items: 'Search images' },
		mediaType: 'image',
		fetch: storedMediaOf('image'),
	},
	{
		name: 'gophenberg-videos',
		labels: { name: 'Videos', search_items: 'Search videos' },
		mediaType: 'video',
		fetch: storedMediaOf('video'),
	},
	{
		name: 'gophenberg-audio',
		labels: { name: 'Audio', search_items: 'Search audio' },
		mediaType: 'audio',
		fetch: storedMediaOf('audio'),
	},
]
