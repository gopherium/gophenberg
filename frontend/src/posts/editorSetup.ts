// SPDX-License-Identifier: Apache-2.0

export const EDITOR_SETTINGS = {
	bodyPlaceholder: 'Start writing.',
	__experimentalBlockPatterns: [],
}

const PREVIEW_ITEMS = {
	desktop: { label: 'Desktop', value: 'desktop' },
	tablet: { label: 'Tablet', value: 'tablet' },
	mobile: { label: 'Mobile', value: 'mobile' },
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
