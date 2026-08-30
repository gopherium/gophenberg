// SPDX-License-Identifier: Apache-2.0

import { env } from 'node:process'

import type { MediaValue, Post } from './content.ts'

/** One media field of an item and the files it names. */
export interface MediaField {
	key: string
	items: MediaValue[]
}

/**
 * Returns the files a value names, or nothing when it names no file.
 * @param value - The value an item holds under a field key.
 * @returns The files named, or nothing.
 */
export function mediaItems(value: unknown): MediaValue[] | undefined {
	if (isMediaValue(value)) {
		return [value]
	}
	if (!Array.isArray(value) || value.length === 0) {
		return undefined
	}
	if (value.every(isMediaValue)) {
		return value
	}
	return undefined
}

/**
 * Reports whether a value names one library file.
 * @param held - A field value or one entry of it.
 * @returns True when the value names a file.
 */
function isMediaValue(held: unknown): held is MediaValue {
	if (typeof held !== 'object' || held === null) {
		return false
	}
	const named = held as Record<string, unknown>
	return (
		typeof named.id === 'number' &&
		typeof named.src === 'string' &&
		typeof named.sizes === 'object' &&
		named.sizes !== null
	)
}

/**
 * Returns the media fields an item carries, keyed in the order their keys read.
 * @param post - The item to read.
 * @returns Each media field and the files it names.
 */
export function mediaFields(post: Post): MediaField[] {
	const held: MediaField[] = []
	for (const key of Object.keys(post.fields).sort()) {
		const items = mediaItems(post.fields[key])
		if (items !== undefined) {
			held.push({ key, items })
		}
	}
	return held
}

/**
 * Returns the address a themed page loads a library file from.
 * @param src - The address the content API served.
 * @returns The address to put in the markup.
 */
export function mediaUrl(src: string): string {
	if (/^[a-z]+:\/\//i.test(src)) {
		return src
	}
	const origin = (env.GOPHENBERG_ASSET_ORIGIN ?? '').replace(/\/+$/, '')
	return `${origin}${src}`
}
