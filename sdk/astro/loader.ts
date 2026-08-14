// SPDX-License-Identifier: Apache-2.0

import type { LiveLoader } from 'astro/loaders'

import { GophenbergClient } from './client.ts'
import type { ClientOptions } from './client.ts'
import { kitName } from './kit.ts'
import type { PostSummary } from './content.ts'

/** A post as a live collection carries it, with content only on a single entry. */
export type LivePost = PostSummary & { content?: string }

/** What a page may narrow a live listing by. */
export interface PostCollectionFilter {
	type?: string
	page?: number
	perPage?: number
}

/** How a page addresses one live post. */
export interface PostEntryFilter {
	path: string
}

/** How a loader reaches its instance, either by client options or with a client already built. */
export type LoaderOptions = ClientOptions & { client?: GophenbergClient }

/**
 * Returns the live loader serving published posts to a theme.
 * @param options - How the loader reaches its instance.
 * @returns The loader an Astro live collection is defined with.
 */
export function gophenbergLoader(
	options: LoaderOptions = {},
): LiveLoader<LivePost, PostEntryFilter, PostCollectionFilter> {
	const client = options.client ?? new GophenbergClient(options)
	return {
		name: kitName,
		loadCollection: async ({ filter }) => {
			try {
				const page = await client.listPosts(filter)
				return { entries: page.items.map((summary) => ({ id: entryId(summary), data: summary })) }
			} catch (cause) {
				return { error: asError(cause) }
			}
		},
		loadEntry: async ({ filter }) => {
			try {
				const held = await client.resolve(filter.path)
				return held?.item && { id: entryId(held.item), data: held.item }
			} catch (cause) {
				return { error: asError(cause) }
			}
		},
	}
}

/**
 * Returns the id a post is unique under within its collection.
 * @param post - The post the id stands for.
 * @returns The address the post answers at.
 */
function entryId(post: PostSummary): string {
	return post.path
}

/**
 * Returns a thrown value as an error.
 * @param cause - What was thrown.
 * @returns The error to report.
 */
function asError(cause: unknown): Error {
	return cause instanceof Error ? cause : new Error(String(cause))
}
