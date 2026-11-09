// SPDX-License-Identifier: Apache-2.0

import type { BulkWords } from '@gopherium/godmin'
import { formatNumber } from '@gopherium/gottext'
import { __, _n, _x, sprintf } from '@wordpress/i18n'

import { deleteTitle, itemWords } from '../lists/itemWords'
import { pluralOf } from './typeText'
import type { ContentType } from './types'

/**
 * Returns the words a switch on or off over types reads as.
 * @param on - Whether the switch turns the types on.
 * @returns The words for the types switched and for the ones that stayed.
 */
export function switchWords(on: boolean): BulkWords<ContentType> {
	return itemWords(pluralOf, {
		named: (type) =>
			sprintf(on ? __('"%s" activated.', 'gophenberg') : __('"%s" deactivated.', 'gophenberg'), type),
		finished: (count) =>
			sprintf(
				on
					? _n('%s content type activated.', '%s content types activated.', count, 'gophenberg')
					: _n('%s content type deactivated.', '%s content types deactivated.', count, 'gophenberg'),
				formatNumber(count),
			),
		unfinished: (count) =>
			sprintf(
				on
					? _n('%s content type could not be activated.', '%s content types could not be activated.', count, 'gophenberg')
					: _n(
							'%s content type could not be deactivated.',
							'%s content types could not be deactivated.',
							count,
							'gophenberg',
						),
				formatNumber(count),
			),
	})
}

/**
 * Returns the words deleting types for good reads as, counting content types where the posts list counts items.
 * @returns The words for the types deleted and for the ones that stayed.
 */
export function removalWords(): BulkWords<ContentType> {
	return itemWords(pluralOf, {
		named: (type) => sprintf(_x('"%s" permanently deleted.', 'content type', 'gophenberg'), type),
		finished: (count) =>
			sprintf(
				_n('%s content type permanently deleted.', '%s content types permanently deleted.', count, 'gophenberg'),
				formatNumber(count),
			),
		unfinished: (count) =>
			sprintf(
				_n(
					'%s content type could not be permanently deleted.',
					'%s content types could not be permanently deleted.',
					count,
					'gophenberg',
				),
				formatNumber(count),
			),
	})
}

/**
 * Returns the title of the confirm asked before types are deleted for good.
 * @param types - The types picked.
 * @returns The question naming the type when only one was picked, else counting content types.
 */
export function removalTitle(types: ContentType[]): string {
	if (types.length === 1) {
		return deleteTitle(types, pluralOf)
	}
	const many = _n('Delete %s content type?', 'Delete %s content types?', types.length, 'gophenberg')
	return sprintf(many, formatNumber(types.length))
}
