// SPDX-License-Identifier: Apache-2.0

import { dispatch } from '@wordpress/data'
import { addFilter } from '@wordpress/hooks'
import type { ComponentType, ReactNode } from 'react'

// BLOCK_EDITOR_STORE names the store the block editor registers itself under.
export const BLOCK_EDITOR_STORE = 'core/block-editor'

// mediaLibraryFilter is the filter the block editor takes its library component from.
const mediaLibraryFilter = 'editor.MediaUpload'

export interface MediaLibraryProps {
	allowedTypes?: string[]
	multiple?: boolean | string
	gallery?: boolean
	addToGallery?: boolean
	value?: number | number[]
	mode?: string
	onSelect: (media: unknown) => void
	onClose?: () => void
	render: (props: { open: () => void }) => ReactNode
}

export interface InserterMediaItem {
	id?: number
	url: string
	alt?: string
	caption?: string
	title?: string
	previewUrl?: string
}

export interface InserterMediaRequest {
	per_page?: number
	page?: number
	search?: string
}

export interface InserterMediaCategory {
	name: string
	labels: { name: string, search_items?: string }
	mediaType: 'image' | 'video' | 'audio'
	fetch: (query: InserterMediaRequest) => Promise<InserterMediaItem[]>
}

/**
 * Serves the block editor's media library buttons from the given component.
 * @param component - The library the editor opens.
 */
export function registerMediaLibrary(component: ComponentType<MediaLibraryProps>): void {
	addFilter(mediaLibraryFilter, 'gophenberg/media-library', () => component)
}

/**
 * Offers the given categories in the inserter's media tab.
 * @param categories - The categories to offer.
 */
export function registerMediaCategories(categories: InserterMediaCategory[]): void {
	const editor = dispatch(BLOCK_EDITOR_STORE) as unknown as {
		registerInserterMediaCategory: (category: InserterMediaCategory) => void
	}
	for (const category of categories) {
		editor.registerInserterMediaCategory(category)
	}
}
