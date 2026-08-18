// SPDX-License-Identifier: Apache-2.0

import { __ } from '@wordpress/i18n'

import {
	ALLOWED_MIME_TYPES,
	IMAGE_SIZES,
	MAX_UPLOAD_BYTES,
	editorMediaUpload,
} from '../media/editorMedia'

export const EDITOR_SETTINGS = {
	get bodyPlaceholder() {
		return __('Start writing.', 'gophenberg')
	},
	__experimentalBlockPatterns: [],
	mediaUpload: editorMediaUpload,
	allowedMimeTypes: ALLOWED_MIME_TYPES,
	imageSizes: IMAGE_SIZES,
	imageDefaultSize: 'large',
	imageEditing: false,
	maxUploadFileSize: MAX_UPLOAD_BYTES,
}

const PREVIEW_ITEMS = {
	desktop: {
		get label() {
			return __('Desktop', 'gophenberg')
		},
		value: 'desktop',
	},
	tablet: {
		get label() {
			return __('Tablet', 'gophenberg')
		},
		value: 'tablet',
	},
	mobile: {
		get label() {
			return __('Mobile', 'gophenberg')
		},
		value: 'mobile',
	},
}

export type PreviewWidth = keyof typeof PREVIEW_ITEMS

export const PREVIEW_WIDTHS = Object.values(PREVIEW_ITEMS)

/**
 * Returns the item the preview select holds for a width.
 * @param width - The width the canvas renders at.
 * @returns The item standing for that width.
 */
export function previewItem(width: PreviewWidth) {
	return PREVIEW_ITEMS[width]
}

/**
 * Returns the classes the canvas carries at a preview width.
 * @param width - The width the canvas renders at.
 * @returns The class attribute of the canvas.
 */
export function canvasClass(width: PreviewWidth): string {
	const base = 'gophenberg-editor__canvas'
	if (width === 'desktop') {
		return base
	}
	return `${base} ${base}--${width}`
}
