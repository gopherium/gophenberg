// SPDX-License-Identifier: Apache-2.0

import type { BulkFailure, BulkWords } from '@gopherium/godmin'
import { formatNumber } from '@gopherium/gottext'
import { __, _n, sprintf } from '@wordpress/i18n'

/** The sentences a run over items reads as. */
interface RunSentences {
	/** The toast for the one item a run finished, given its name. */
	named: (name: string) => string
	/** The toast counting the items a run finished. */
	finished: (count: number) => string
	/** The toast counting the items a run could not finish. */
	unfinished: (count: number) => string
}

/**
 * Returns the reason the one failed call gave, already in the reader's words.
 * @param failures - The failures of a run over one item.
 * @returns The reason.
 */
function reasonOf<Item>(failures: BulkFailure<Item>[]): string {
	return (failures[0].error as Error).message
}

/**
 * Returns the words a write to one item reads as.
 * @param done - The toast once the write landed.
 * @returns The words for the write that landed and the reason of the one that failed.
 */
export function oneWriteWords<Item>(done: string): BulkWords<Item> {
	return { done: () => done, failed: reasonOf }
}

/**
 * Returns the words a run over items reads as.
 * @param nameOf - Returns the name an item is listed under.
 * @param sentences - The toasts of the run, the one counting its failures included.
 * @returns The words for the items that finished and for the ones that failed, the reason itself for a run over one.
 */
export function itemWords<Item>(nameOf: (item: Item) => string, sentences: RunSentences): BulkWords<Item> {
	return {
		done: (count, only) => (only === undefined ? sentences.finished(count) : sentences.named(nameOf(only))),
		failed: (failures, asked) => (asked === 1 ? reasonOf(failures) : sentences.unfinished(failures.length)),
	}
}

/**
 * Returns the title of the confirm asked before the given items are deleted for good.
 * @param items - The items acted on.
 * @param nameOf - Returns the name an item is listed under.
 * @returns The question naming the item when only one was picked, else counting the items.
 */
export function deleteTitle<Item>(items: Item[], nameOf: (item: Item) => string): string {
	if (items.length === 1) {
		return sprintf(__('Delete "%(title)s"?', 'gophenberg'), { title: nameOf(items[0]) })
	}
	return sprintf(_n('Delete %s item?', 'Delete %s items?', items.length, 'gophenberg'), formatNumber(items.length))
}

/**
 * Returns the sentence a confirm describes an action that cannot be undone with.
 * @returns The sentence.
 */
export function irreversible(): string {
	return __('This action cannot be undone.', 'gophenberg')
}

/**
 * Returns the words a permanent delete over items reads as.
 * @param nameOf - Returns the name an item is listed under.
 * @returns The words for the items deleted and for the ones that stayed.
 */
export function deleteWords<Item>(nameOf: (item: Item) => string): BulkWords<Item> {
	return itemWords(nameOf, {
		named: (title) => sprintf(__('"%s" permanently deleted.', 'gophenberg'), title),
		finished: (count) =>
			sprintf(
				_n('%s item permanently deleted.', '%s items permanently deleted.', count, 'gophenberg'),
				formatNumber(count),
			),
		unfinished: (count) =>
			sprintf(
				_n(
					'%s item could not be permanently deleted.',
					'%s items could not be permanently deleted.',
					count,
					'gophenberg',
				),
				formatNumber(count),
			),
	})
}
