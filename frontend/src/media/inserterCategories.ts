// SPDX-License-Identifier: Apache-2.0

import type {
	InserterMediaCategory,
	InserterMediaItem,
	InserterMediaRequest,
} from '@gophenberg/frontend-sdk/editor'
import { __ } from '@wordpress/i18n'

import { listMedia, mediaSrc } from './api'
import type { MediaItem } from './api'
import { bestRendition, displayFile } from './format'

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
		url: mediaSrc(displayFile(item)),
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
		const page = await listMedia({ mimes: [`${kind}/`], search: query.search, page: query.page })
		return page.items.map(toInserterItem)
	}
}

export const MEDIA_CATEGORIES: InserterMediaCategory[] = [
	{
		name: 'gophenberg-images',
		labels: {
			get name() {
				return __('Images', 'gophenberg')
			},
			get search_items() {
				return __('Search images', 'gophenberg')
			},
		},
		mediaType: 'image',
		fetch: storedMediaOf('image'),
	},
	{
		name: 'gophenberg-videos',
		labels: {
			get name() {
				return __('Videos', 'gophenberg')
			},
			get search_items() {
				return __('Search videos', 'gophenberg')
			},
		},
		mediaType: 'video',
		fetch: storedMediaOf('video'),
	},
	{
		name: 'gophenberg-audio',
		labels: {
			get name() {
				return __('Audio', 'gophenberg')
			},
			get search_items() {
				return __('Search audio', 'gophenberg')
			},
		},
		mediaType: 'audio',
		fetch: storedMediaOf('audio'),
	},
]
