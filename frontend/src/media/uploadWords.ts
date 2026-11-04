// SPDX-License-Identifier: Apache-2.0

import type { BulkFailure, BulkWords } from '@gopherium/godmin'
import { formatNumber } from '@gopherium/gottext'
import { __, _n, sprintf } from '@wordpress/i18n'

/**
 * Returns the reason a file was not stored, already in the reader's words.
 * @param failure - The file and what its upload failed with.
 * @returns The reason.
 */
function reasonOf(failure: BulkFailure<File>): string {
	return (failure.error as Error).message
}

/**
 * Returns the line naming a file with the reason it was not stored.
 * @param file - The name of the file.
 * @param reason - The reason, already in the reader's words.
 * @returns The line.
 */
export function fileReason(file: string, reason: string): string {
	return sprintf(__('%(file)s: %(reason)s', 'gophenberg'), { file, reason })
}

/**
 * Returns the notice naming the files an upload could not store, one line per file when several were sent.
 * @param failures - The files that failed and what each failed with.
 * @param asked - How many files the upload was given.
 * @returns The reason alone for one file sent, else a line naming each failed file with its reason.
 */
export function refusedLines(failures: BulkFailure<File>[], asked: number): string {
	if (asked === 1) {
		return reasonOf(failures[0])
	}
	return failures.map((failure) => fileReason(failure.item.name, reasonOf(failure))).join('\n')
}

/**
 * Returns the words an upload to the library reads as.
 * @returns The toast naming the one file stored or counting the files stored, and the notice for the refused ones.
 */
export function uploadWords(): BulkWords<File> {
	return {
		done: (count, only) =>
			only === undefined
				? sprintf(_n('Uploaded %s file.', 'Uploaded %s files.', count, 'gophenberg'), formatNumber(count))
				: sprintf(__('"%s" uploaded.', 'gophenberg'), only.name),
		failed: refusedLines,
	}
}
