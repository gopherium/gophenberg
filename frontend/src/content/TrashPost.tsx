// SPDX-License-Identifier: Apache-2.0

import { AlertDialog, Button } from '@gophenberg/frontend-sdk'
import { useToaster } from '@gopherium/godmin'
import { __, sprintf } from '@wordpress/i18n'
import { useNavigate } from '@tanstack/react-router'

import { useRefresh } from './actions'
import { trashPost } from './api'

/**
 * Renders the control moving the post being edited to the trash, and its confirm.
 * @param props - The post to trash and the name it is known by.
 * @returns The trash control and its confirm.
 */
export function TrashPost({ postId, title }: { postId: string, title: string }) {
	const navigate = useNavigate()
	const refresh = useRefresh()
	const toaster = useToaster()
	const label = __('Move to trash', 'gophenberg')
	const named = toaster.name(title === '' ? __('(no title)', 'gophenberg') : title)
	const confirm = async () => {
		try {
			await trashPost(postId)
		} catch (failure) {
			return { close: false, error: (failure as Error).message }
		}
		await navigate({ to: '/content/$typeKey' })
		await refresh([postId])
		toaster.show(sprintf(__('"%s" moved to the trash.', 'gophenberg'), named))
	}
	return (
		<AlertDialog.Root onConfirm={confirm}>
			<AlertDialog.Trigger render={<Button variant="outline" />}>{label}</AlertDialog.Trigger>
			<AlertDialog.Popup
				title={__('Move to trash?', 'gophenberg')}
				description={sprintf(
					__('"%(title)s" goes to the trash. You can restore it from the Trash tab.', 'gophenberg'),
					{ title: named },
				)}
				confirmButtonText={label}
				cancelButtonText={__('Cancel', 'gophenberg')}
			/>
		</AlertDialog.Root>
	)
}
