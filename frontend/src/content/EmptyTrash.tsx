// SPDX-License-Identifier: Apache-2.0

import { Button, Dialog } from '@gophenberg/frontend-sdk'
import { ConfirmBody, useToaster } from '@gopherium/godmin'
import { formatNumber } from '@gopherium/gottext'
import { __, _n, sprintf } from '@wordpress/i18n'
import { useMutation, useQueryClient } from '@tanstack/react-query'
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
 * Renders the control removing every trashed item of a type for good, and its confirm.
 * @param props - The type whose trash to empty, its plural label, and what the list does around the run.
 * @returns The empty trash control and its confirm.
 */
export function EmptyTrash({ type, label, list }: { type: string, label: string, list: ListRun }) {
	const client = useQueryClient()
	const refresh = useRefresh()
	const toaster = useToaster()
	const [open, setOpen] = useState(false)
	const action = __('Empty Trash', 'gophenberg')
	const empty = useMutation({
		mutationFn: () => emptyTrash(type),
		onMutate: () => list.onStart(),
		onSuccess: async (emptied, _asked, settle) => {
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
		},
	})
	return (
		<Dialog.Root
			open={open}
			onOpenChange={(next) => {
				empty.reset()
				setOpen(next)
			}}
		>
			<Dialog.Trigger render={<Button variant="outline" size="compact" />}>{action}</Dialog.Trigger>
			<Dialog.Popup size="small">
				<Dialog.Header>
					<Dialog.Title>{action}</Dialog.Title>
					<Dialog.CloseIcon />
				</Dialog.Header>
				<Dialog.Content>
					<ConfirmBody
						confirmLabel={action}
						cancelLabel={__('Cancel', 'gophenberg')}
						busy={empty.isPending}
						failure={empty.error?.message}
						onConfirm={() => empty.mutate()}
						onCancel={empty.isPending ? undefined : () => setOpen(false)}
					>
						{sprintf(
							__('Every item in the %(type)s trash is removed for good. This cannot be undone.', 'gophenberg'),
							{ type: label.toLowerCase() },
						)}
					</ConfirmBody>
				</Dialog.Content>
			</Dialog.Popup>
		</Dialog.Root>
	)
}
