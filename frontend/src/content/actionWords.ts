// SPDX-License-Identifier: Apache-2.0

import type { BulkWords } from '@gopherium/godmin'
import { formatNumber } from '@gopherium/gottext'
import { __, _n, sprintf } from '@wordpress/i18n'

import { irreversible, itemWords } from '../lists/itemWords'
import type { Post } from './api'

/**
 * Returns the name a post is listed under.
 * @param post - The post to name.
 * @returns The title, or a stand in for a post that has none.
 */
export function postName(post: Post): string {
	return post.title === '' ? __('(no title)', 'gophenberg') : post.title
}

/**
 * Returns the words a trash over posts reads as.
 * @returns The words for the posts trashed and for the ones that stayed.
 */
export function trashWords(): BulkWords<Post> {
	return itemWords(postName, {
		named: (title) => sprintf(__('"%s" moved to the trash.', 'gophenberg'), title),
		finished: (count) =>
			sprintf(_n('%s item moved to the trash.', '%s items moved to the trash.', count, 'gophenberg'), formatNumber(count)),
		unfinished: (count) =>
			sprintf(
				_n('%s item could not be moved to the trash.', '%s items could not be moved to the trash.', count, 'gophenberg'),
				formatNumber(count),
			),
	})
}

/**
 * Returns the description of the confirm asked before posts are deleted for good.
 * @param count - How many posts the confirm deletes.
 * @returns What goes with the posts, counting them when there are several, then the warning.
 */
export function deleteDescription(count: number): string {
	const what =
		count === 1
			? __('The item is deleted for good, with its revisions and autosaves.', 'gophenberg')
			: sprintf(
					_n(
						'The %s item is deleted for good, with its revisions and autosaves.',
						'The %s items are deleted for good, with their revisions and autosaves.',
						count,
						'gophenberg',
					),
					formatNumber(count),
				)
	return `${what} ${irreversible()}`
}

/**
 * Returns the toast counting the posts a restore brought back, worded on pages for the page type.
 * @param type - The content type the posts belong to.
 * @param count - How many posts came back.
 * @returns The toast.
 */
function restoredCount(type: string, count: number): string {
	const many =
		type === 'page'
			? _n('%s page has been restored.', '%s pages have been restored.', count, 'gophenberg')
			: _n('%s post has been restored.', '%s posts have been restored.', count, 'gophenberg')
	return sprintf(many, formatNumber(count))
}

/**
 * Returns the words a restore over posts reads as.
 * @param type - The content type the posts belong to.
 * @returns The words for the posts restored and for the ones that stayed in the trash.
 */
export function restoreWords(type: string): BulkWords<Post> {
	return itemWords(postName, {
		named: (title) => sprintf(__('"%s" has been restored.', 'gophenberg'), title),
		finished: (count) => restoredCount(type, count),
		unfinished: (count) =>
			sprintf(
				_n('%s item could not be restored.', '%s items could not be restored.', count, 'gophenberg'),
				formatNumber(count),
			),
	})
}
