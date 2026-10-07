// SPDX-License-Identifier: Apache-2.0

import type { BulkFailure, BulkWords } from '@gopherium/godmin'
import { formatNumber } from '@gopherium/gottext'
import { __, _n, sprintf } from '@wordpress/i18n'

import type { Post } from './api'

/** Cuts a title to the length a toast shows. */
export type NameCut = (title: string) => string

/**
 * Returns the name a post is listed under.
 * @param post - The post to name.
 * @returns The title, or a stand in for a post that has none.
 */
function nameOf(post: Post): string {
	return post.title === '' ? __('(no title)', 'gophenberg') : post.title
}

/**
 * Returns the question asked before the given posts are trashed.
 * @param items - The posts acted on.
 * @param name - Cuts a title to the length a toast shows.
 * @returns The question naming the post when only one was picked.
 */
export function trashQuestion(items: Post[], name: NameCut): string {
	if (items.length === 1) {
		return sprintf(__('Move "%(title)s" to the trash?', 'gophenberg'), { title: name(nameOf(items[0])) })
	}
	const many = _n('Move %s item to the trash?', 'Move %s items to the trash?', items.length, 'gophenberg')
	return sprintf(many, formatNumber(items.length))
}

/**
 * Returns the question asked before the given posts are deleted for good.
 * @param items - The posts acted on.
 * @param name - Cuts a title to the length a toast shows.
 * @returns The question naming the post when only one was picked.
 */
export function deleteQuestion(items: Post[], name: NameCut): string {
	if (items.length === 1) {
		const one = __('Delete "%(title)s" for good? This cannot be undone.', 'gophenberg')
		return sprintf(one, { title: name(nameOf(items[0])) })
	}
	const many = _n(
		'Delete %s item for good? This cannot be undone.',
		'Delete %s items for good? This cannot be undone.',
		items.length,
		'gophenberg',
	)
	return sprintf(many, formatNumber(items.length))
}

/**
 * Returns the words a run over posts reads as.
 * @param name - Cuts a title to the length a toast shows.
 * @param named - The toast for the one post a run finished.
 * @param finished - The toast counting the posts a run finished.
 * @param unfinished - The notice counting the posts a run could not finish.
 * @returns The words for the posts that finished and for the ones that failed.
 */
function postWords(
	name: NameCut,
	named: (title: string) => string,
	finished: (count: number) => string,
	unfinished: (count: number) => string,
): BulkWords<Post> {
	return {
		done: (count, only) => (only === undefined ? finished(count) : named(name(nameOf(only)))),
		failed: (failures, asked) => (asked === 1 ? reasonOf(failures) : unfinished(failures.length)),
	}
}

/**
 * Returns the reason the one failed call gave, already in the reader's words.
 * @param failures - The failures of a run over one post.
 * @returns The reason.
 */
function reasonOf(failures: BulkFailure<Post>[]): string {
	return (failures[0].error as Error).message
}

/**
 * Returns the words a trash over posts reads as.
 * @param name - Cuts a title to the length a toast shows.
 * @returns The words for the posts trashed and for the ones that stayed.
 */
export function trashWords(name: NameCut): BulkWords<Post> {
	return postWords(
		name,
		(title) => sprintf(__('"%s" moved to the trash.', 'gophenberg'), title),
		(count) =>
			sprintf(_n('%s item moved to the trash.', '%s items moved to the trash.', count, 'gophenberg'), formatNumber(count)),
		(count) =>
			sprintf(
				_n('%s item could not be moved to the trash.', '%s items could not be moved to the trash.', count, 'gophenberg'),
				formatNumber(count),
			),
	)
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
 * @param name - Cuts a title to the length a toast shows.
 * @param type - The content type the posts belong to.
 * @returns The words for the posts restored and for the ones that stayed in the trash.
 */
export function restoreWords(name: NameCut, type: string): BulkWords<Post> {
	return postWords(
		name,
		(title) => sprintf(__('"%s" has been restored.', 'gophenberg'), title),
		(count) => restoredCount(type, count),
		(count) =>
			sprintf(
				_n('%s item could not be restored.', '%s items could not be restored.', count, 'gophenberg'),
				formatNumber(count),
			),
	)
}

/**
 * Returns the words a permanent delete over posts reads as.
 * @param name - Cuts a title to the length a toast shows.
 * @returns The words for the posts deleted and for the ones that stayed.
 */
export function deleteWords(name: NameCut): BulkWords<Post> {
	return postWords(
		name,
		(title) => sprintf(__('"%s" permanently deleted.', 'gophenberg'), title),
		(count) =>
			sprintf(
				_n('%s item permanently deleted.', '%s items permanently deleted.', count, 'gophenberg'),
				formatNumber(count),
			),
		(count) =>
			sprintf(
				_n(
					'%s item could not be permanently deleted.',
					'%s items could not be permanently deleted.',
					count,
					'gophenberg',
				),
				formatNumber(count),
			),
	)
}
