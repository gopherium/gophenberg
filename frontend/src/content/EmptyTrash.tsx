// SPDX-License-Identifier: Apache-2.0

import { AlertDialog, Button } from '@gophenberg/frontend-sdk'
import { useToaster } from '@gopherium/godmin'
import { formatNumber } from '@gopherium/gottext'
import { __, _n, sprintf } from '@wordpress/i18n'
import { useQueryClient } from '@tanstack/react-query'
import { useState } from 'react'

import { useRefresh } from './actions'
import type { ListRun } from './actions'
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
 * Renders the control removing every trashed item of a type for good, and its confirm.
 * @param props - The type whose trash to empty, how many items it holds, and what the list does around the run.
 * @returns The empty trash control and its confirm.
 */
export function EmptyTrash({ type, count, list }: { type: string, count: number, list: ListRun }) {
	const client = useQueryClient()
	const refresh = useRefresh()
	const toaster = useToaster()
	const [open, setOpen] = useState(false)
	const confirm = async () => {
		const settle = list.onStart()
		let emptied: Emptied
		try {
			emptied = await emptyTrash(type)
		} catch (failure) {
			return { close: false, error: (failure as Error).message }
		}
		client.removeQueries({
			queryKey: ['post'],
			predicate: (query) => (query.state.data as Post | undefined)?.type === type,
		})
		await refresh()
		setOpen(false)
		const toast = emptiedToast(emptied)
		if (toast !== '') {
			toaster.show(toast)
		}
		settle(undefined, emptied.deleted > 0)
	}
	return (
		<AlertDialog.Root open={open} onOpenChange={setOpen} onConfirm={confirm}>
			<AlertDialog.Trigger render={<Button variant="outline" size="compact" />}>
				{__('Empty Trash', 'gophenberg')}
			</AlertDialog.Trigger>
			<AlertDialog.Popup
				intent="irreversible"
				title={__('Empty the trash?', 'gophenberg')}
				description={warningOf(count)}
				confirmButtonText={__('Empty trash', 'gophenberg')}
				cancelButtonText={__('Cancel', 'gophenberg')}
			/>
		</AlertDialog.Root>
	)
}
