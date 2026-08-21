// SPDX-License-Identifier: Apache-2.0

import { displayLocale } from '@gopherium/gottext'
import type { MediaItem } from './api'

/** The binary steps a file size is reported in. */
const sizeUnits = [
	{ mag: 1024 ** 4, unit: 'TB' },
	{ mag: 1024 ** 3, unit: 'GB' },
	{ mag: 1024 ** 2, unit: 'MB' },
	{ mag: 1024, unit: 'KB' },
	{ mag: 1, unit: 'B' },
]

/**
 * Returns a byte count in the largest unit that leaves it above one.
 * @param bytes - The size of the file.
 * @returns The size to show, empty when there is nothing to report.
 */
export function fileSize(bytes: number): string {
	const step = sizeUnits.find(({ mag }) => bytes >= mag)
	if (step === undefined) {
		return ''
	}
	const scaled = (bytes / step.mag).toLocaleString(displayLocale(), {
		minimumFractionDigits: 0,
		maximumFractionDigits: 2,
	})
	return `${scaled} ${step.unit}`
}

/**
 * Returns the pixel dimensions of a picture.
 * @param item - The media item to measure.
 * @returns The dimensions to show, empty for a file that carries none.
 */
export function imageDimensions(item: MediaItem): string {
	if (item.width <= 0 || item.height <= 0) {
		return ''
	}
	return `${item.width} × ${item.height}`
}

/**
 * Returns the name a media item is listed under.
 * @param item - The media item to name.
 * @returns The title, or the file name when the item has no title.
 */
export function mediaName(item: MediaItem): string {
	return item.title === '' ? item.file.slice(item.file.lastIndexOf('/') + 1) : item.title
}

/**
 * Returns the file a placed media item displays, preferring the display copy.
 * @param item - The media item to place.
 * @returns The full rendition's file, or the stored file without one.
 */
export function displayFile(item: MediaItem): string {
	return item.sizes.full?.file ?? item.file
}

/**
 * Returns the file of the smallest rendition at least as wide as target.
 * @param item - The media item to draw.
 * @param target - The width the caller draws at.
 * @returns The file to load, the stored file when the item carries no renditions.
 */
export function bestRendition(item: MediaItem, target: number): string {
	const widths = Object.values(item.sizes).sort((left, right) => left.width - right.width)
	if (widths.length === 0) {
		return item.file
	}
	const fitting = widths.find((rendition) => rendition.width >= target)
	return (fitting ?? widths[widths.length - 1]).file
}
