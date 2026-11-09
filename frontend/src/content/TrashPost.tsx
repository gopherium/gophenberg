// SPDX-License-Identifier: Apache-2.0

import { Button } from '@gophenberg/frontend-sdk/editor'
import { ConfirmDialog, useToaster } from '@gopherium/godmin'
import { __, sprintf } from '@wordpress/i18n'
import { useNavigate, useRouter } from '@tanstack/react-router'
import { useState } from 'react'

import { useRefresh } from './actions'
import { trashPost } from './api'

/**
 * Renders the control moving the post being edited to the trash, and the question it asks first.
 * @param props - The post to trash and the name it is known by.
 * @returns The trash control, busy while the trash runs, and its confirm while it asks.
 */
export function TrashPost({ postId, title }: { postId: string, title: string }) {
	const navigate = useNavigate()
	const router = useRouter()
	const refresh = useRefresh()
	const toaster = useToaster()
	const [asking, setAsking] = useState(false)
	const [trashing, setTrashing] = useState(false)
	const label = __('Move to trash', 'gophenberg')
	const named = title === '' ? __('(no title)', 'gophenberg') : title
	const trash = () => {
		setTrashing(true)
		const editing = router.latestLocation.pathname
		void trashPost(postId).then(
			async () => {
				if (router.latestLocation.pathname === editing) {
					await navigate({ to: '/content/$typeKey' })
				}
				await refresh([postId])
				toaster.show(sprintf(__('"%s" moved to the trash.', 'gophenberg'), named))
			},
			(failure: Error) => {
				setTrashing(false)
				toaster.show(failure.message)
			},
		)
	}
	return (
		<>
			<Button
				__next40pxDefaultSize
				className="gophenberg-editor__trash"
				isDestructive
				variant="secondary"
				isBusy={trashing}
				aria-disabled={trashing}
				onClick={trashing ? undefined : () => setAsking(true)}
			>
				{label}
			</Button>
			{asking ? (
				<ConfirmDialog
					size="small"
					confirmLabel={label}
					cancelLabel={__('Cancel', 'gophenberg')}
					onConfirm={trash}
					onClose={() => setAsking(false)}
				>
					{sprintf(__('Are you sure you want to move "%s" to the trash?', 'gophenberg'), named)}
				</ConfirmDialog>
			) : null}
		</>
	)
}
