// SPDX-License-Identifier: Apache-2.0

import { Button } from '@gophenberg/frontend-sdk/editor'
import { useToaster } from '@gopherium/godmin'
import { __, sprintf } from '@wordpress/i18n'
import { useNavigate, useRouter } from '@tanstack/react-router'
import { useState } from 'react'

import { useRefresh } from './actions'
import { restorePost, trashPost } from './api'

/**
 * Renders the control moving the post being edited to the trash at once, its toast offering Undo.
 * @param props - The post to trash and the name it is known by.
 * @returns The trash control, busy while the trash runs.
 */
export function TrashPost({ postId, title }: { postId: string, title: string }) {
	const navigate = useNavigate()
	const router = useRouter()
	const refresh = useRefresh()
	const toaster = useToaster()
	const [trashing, setTrashing] = useState(false)
	const named = title === '' ? __('(no title)', 'gophenberg') : title
	const restore = () => {
		void restorePost(postId).then(
			async () => {
				await refresh([postId])
				toaster.show(sprintf(__('"%s" has been restored.', 'gophenberg'), named))
			},
			(failure: Error) => toaster.show(failure.message),
		)
	}
	const trash = () => {
		setTrashing(true)
		const editing = router.latestLocation.pathname
		void trashPost(postId).then(
			async () => {
				if (router.latestLocation.pathname === editing) {
					await navigate({ to: '/content/$typeKey' })
				}
				await refresh([postId])
				toaster.show(sprintf(__('"%s" moved to the trash.', 'gophenberg'), named), {
					label: __('Undo', 'gophenberg'),
					onAct: restore,
				})
			},
			(failure: Error) => {
				setTrashing(false)
				toaster.show(failure.message)
			},
		)
	}
	return (
		<Button
			__next40pxDefaultSize
			className="gophenberg-editor__trash"
			isDestructive
			variant="secondary"
			isBusy={trashing}
			aria-disabled={trashing}
			onClick={trashing ? undefined : trash}
		>
			{__('Move to trash', 'gophenberg')}
		</Button>
	)
}
