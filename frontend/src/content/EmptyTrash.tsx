// SPDX-License-Identifier: Apache-2.0

import { Button } from '@gophenberg/frontend-sdk'
import { useToaster } from '@gopherium/godmin'
import type { ListRun } from '@gopherium/godmin'
import { ConfirmDialog } from '@gopherium/godmin/dialogs'
import { formatNumber } from '@gopherium/gottext'
import { __, _n, sprintf } from '@wordpress/i18n'
import { useQueryClient } from '@tanstack/react-query'
import { useState } from 'react'

import { useRefresh } from './actions'
import { emptyTrash } from './api'
import type { Emptied, Post } from './api'

/**
 * Returns the toast counting the items emptying the trash deleted for good and the items left there.
 * @param emptied - How many items went and how many stayed.
 * @returns The toast, empty when no item went and none stayed.
 */
function emptiedToast({ deleted, kept }: Emptied): string {
	const said: string[] = []
	if (deleted > 0) {
		const went = _n('%s item permanently deleted.', '%s items permanently deleted.', deleted, 'gophenberg')
		said.push(sprintf(went, formatNumber(deleted)))
	}
	if (kept > 0) {
		const stayed = _n('%s item stays in the trash.', '%s items stay in the trash.', kept, 'gophenberg')
		said.push(sprintf(stayed, formatNumber(kept)))
	}
	return said.join(' ')
}

/**
 * Returns the warning counting the items emptying the trash deletes for good.
 * @param count - How many items the trash holds.
 * @returns The warning.
 */
function warningOf(count: number): string {
	const warning = _n(
		'The %(count)s item in the trash is deleted for good. This cannot be undone.',
		'The %(count)s items in the trash are deleted for good. This cannot be undone.',
		count,
		'gophenberg',
	)
	return sprintf(warning, { count: formatNumber(count) })
}

/**
 * Returns the handler emptying the trash of a type, reporting how it went in a toast and settling the run on the list.
 * @param type - The type whose trash to empty.
 * @param list - What the list does around the run.
 * @param setEmptying - Marks the control busy while the trash empties.
 * @returns The handler.
 */
function useEmptying(type: string, list: ListRun, setEmptying: (emptying: boolean) => void): () => Promise<void> {
	const client = useQueryClient()
	const refresh = useRefresh()
	const toaster = useToaster()
	return async () => {
		setEmptying(true)
		const settle = list.onStart()
		let emptied: Emptied
		try {
			emptied = await emptyTrash(type)
		} catch (failure) {
			setEmptying(false)
			toaster.show((failure as Error).message)
			settle(false)
			return
		}
		client.removeQueries({
			queryKey: ['post'],
			predicate: (query) => (query.state.data as Post | undefined)?.type === type,
		})
		setEmptying(false)
		const toast = emptiedToast(emptied)
		if (toast !== '') {
			toaster.show(toast)
		}
		settle(emptied.deleted > 0)
		const ended = emptied.deleted > 0 ? list.onRefresh() : undefined
		await refresh().finally(ended)
	}
}

/**
 * Renders the control removing every trashed item of a type for good, and its confirm.
 * @param props - The type whose trash to empty, how many items it holds, and what the list does around the run.
 * @returns The empty trash control, busy while the trash empties, and its confirm while it asks.
 */
export function EmptyTrash({ type, count, list }: { type: string, count: number, list: ListRun }) {
	const [asking, setAsking] = useState(false)
	const [emptying, setEmptying] = useState(false)
	const empty = useEmptying(type, list, setEmptying)
	return (
		<>
			<Button variant="outline" size="compact" loading={emptying} onClick={() => setAsking(true)}>
				{__('Empty Trash', 'gophenberg')}
			</Button>
			{asking ? (
				<ConfirmDialog
					confirmLabel={__('Empty trash', 'gophenberg')}
					cancelLabel={__('Cancel', 'gophenberg')}
					onConfirm={() => {
						void empty()
					}}
					onClose={() => setAsking(false)}
				>
					{warningOf(count)}
				</ConfirmDialog>
			) : null}
		</>
	)
}
