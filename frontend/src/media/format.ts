// SPDX-License-Identifier: Apache-2.0

import { formatNumber } from '@gopherium/gottext'
import { __, _n, _x, sprintf } from '@wordpress/i18n'

import { irreversible } from '../lists/itemWords'
import type { MediaItem } from './api'

/** The content type prefix of each WordPress file type group a stored file falls in, keyed by the group. */
const KIND_PREFIXES: Record<string, string> = {
	image: 'image/',
	audio: 'audio/',
	video: 'video/',
	document: 'application/pdf',
	archive: 'application/zip',
}

/**
 * Returns the binary steps a file size is reported in, largest first, read fresh so the loaded catalogue answers.
 * @returns Each step in bytes beside its unit symbol.
 */
function sizeUnits(): { mag: number, unit: string }[] {
	return [
		{ mag: 1024 ** 8, unit: _x('YB', 'unit symbol', 'gophenberg') },
		{ mag: 1024 ** 7, unit: _x('ZB', 'unit symbol', 'gophenberg') },
		{ mag: 1024 ** 6, unit: _x('EB', 'unit symbol', 'gophenberg') },
		{ mag: 1024 ** 5, unit: _x('PB', 'unit symbol', 'gophenberg') },
		{ mag: 1024 ** 4, unit: _x('TB', 'unit symbol', 'gophenberg') },
		{ mag: 1024 ** 3, unit: _x('GB', 'unit symbol', 'gophenberg') },
		{ mag: 1024 ** 2, unit: _x('MB', 'unit symbol', 'gophenberg') },
		{ mag: 1024, unit: _x('KB', 'unit symbol', 'gophenberg') },
		{ mag: 1, unit: _x('B', 'unit symbol', 'gophenberg') },
	]
}

/**
 * Returns a byte count in the largest unit that leaves it above one, as WordPress writes a file size.
 * @param bytes - The size of the file.
 * @returns The size to show, empty when there is nothing to report.
 */
export function fileSize(bytes: number): string {
	const step = sizeUnits().find(({ mag }) => bytes >= mag)
	if (step === undefined) {
		return ''
	}
	const amount = formatNumber(bytes / step.mag, { maximumFractionDigits: 2 })
	return sprintf(_x('%1$s %2$s', 'file size', 'gophenberg'), amount, step.unit)
}

/**
 * Returns the pixel dimensions of a picture, as WordPress writes them.
 * @param item - The media item to measure.
 * @returns The dimensions to show, empty for a file that carries none.
 */
export function imageDimensions(item: MediaItem): string {
	if (item.width <= 0 || item.height <= 0) {
		return ''
	}
	return sprintf(_x('%1$s × %2$s', 'image dimensions', 'gophenberg'), String(item.width), String(item.height))
}

/**
 * Returns the WordPress file type group a content type falls in, read from its prefix as WordPress reads it.
 * @param mimeType - The content type of the file.
 * @returns The group, file for a type no group takes.
 */
export function kindOf(mimeType: string): string {
	return Object.keys(KIND_PREFIXES).find((kind) => mimeType.startsWith(KIND_PREFIXES[kind])) ?? 'file'
}

/**
 * Returns the content type prefix the server narrows a listing to for a file type group.
 * @param kind - The group.
 * @returns The prefix.
 */
export function kindPrefix(kind: string): string {
	return KIND_PREFIXES[kind]
}

/**
 * Reports whether a media item is a picture, told from its content type as WordPress tells one.
 * @param item - The media item.
 * @returns Whether its content type is an image type.
 */
export function isPicture(item: MediaItem): boolean {
	return kindOf(item.mimeType) === 'image'
}

/**
 * Returns the name a media item is shown under, as WordPress names an untitled item.
 * @param item - The media item.
 * @returns The title, or (no title) when the item has none.
 */
export function titleOf(item: MediaItem): string {
	return item.title === '' ? __('(no title)', 'gophenberg') : item.title
}

/**
 * Returns the sentence a failed edit of a file reads as, WordPress's words for a picture or for any other file.
 * @param item - The file edited.
 * @param reason - Why the edit failed, in the reader's words.
 * @returns The sentence ending on the reason.
 */
export function saveFailure(item: MediaItem, reason: string): string {
	if (isPicture(item)) {
		return sprintf(__('Could not save image. %s', 'gophenberg'), reason)
	}
	return sprintf(__('Could not save media. %s', 'gophenberg'), reason)
}

/**
 * Returns the description of the confirm asked before files are deleted for good.
 * @param count - How many files the confirm deletes.
 * @returns What goes with the files, counting them when there are several, then the warning.
 */
export function fileDeleteDescription(count: number): string {
	const what =
		count === 1
			? __('The file is deleted from your site, with every size made from it.', 'gophenberg')
			: sprintf(
					_n(
						'The %s file is deleted from your site, with every size made from it.',
						'The %s files are deleted from your site, with every size made from them.',
						count,
						'gophenberg',
					),
					formatNumber(count),
				)
	return `${what} ${irreversible()}`
}

/**
 * Returns the file a placed media item displays, preferring the display copy.
 * @param item - The media item to place.
 * @returns The file of the full sub-size, or the stored file without one.
 */
export function displayFile(item: MediaItem): string {
	return item.sizes.full?.file ?? item.file
}

/**
 * Returns the file of the smallest sub-size at least as wide as target.
 * @param item - The media item to draw.
 * @param target - The width the caller draws at.
 * @returns The file to load, the stored file when the item carries no sub-sizes.
 */
export function bestSubSizeFile(item: MediaItem, target: number): string {
	const widths = Object.values(item.sizes).sort((left, right) => left.width - right.width)
	if (widths.length === 0) {
		return item.file
	}
	const fitting = widths.find((subSize) => subSize.width >= target)
	return (fitting ?? widths[widths.length - 1]).file
}
