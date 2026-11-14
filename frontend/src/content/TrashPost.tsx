// SPDX-License-Identifier: Apache-2.0

import { Button } from '@gophenberg/frontend-sdk/editor'
import { useToaster } from '@gopherium/godmin'
import { __, sprintf } from '@wordpress/i18n'
import { useNavigate, useRouter } from '@tanstack/react-router'
import { useState } from 'react'

import { useRefresh } from './actions'
import { restorePost, trashPost } from './api'
import { useListOnShow } from './listOnShow'

/**
 * Renders the control parking the unsaved words, then moving the post to the trash at once, its toast offering Undo.
 * @param props - The post to trash, the name it is known by and what parks its unsaved words at once.
 * @returns The trash control, busy while the trash runs.
 */
export function TrashPost(
	{ postId, title, parkNow }: { postId: string, title: string, parkNow: () => Promise<() => void> },
) {
	const navigate = useNavigate()
	const router = useRouter()
	const refresh = useRefresh()
	const toaster = useToaster()
	const listOnShow = useListOnShow()
	const [trashing, setTrashing] = useState(false)
	const named = title === '' ? __('(no title)', 'gophenberg') : title
	const restore = () => {
		const list = listOnShow()
		const settle = list?.onStart()
		void restorePost(postId).then(
			async () => {
				toaster.show(sprintf(__('"%s" has been restored.', 'gophenberg'), named))
				settle?.(true)
				const ended = list?.onRefresh()
				await refresh([postId]).finally(ended)
			},
			(failure: Error) => {
				toaster.show(failure.message)
				settle?.(false)
			},
		)
	}
	const unsaved = () => {
		const words = __('"%s" was not moved to the trash, because its changes could not be saved.', 'gophenberg')
		throw new Error(sprintf(words, named))
	}
	const trash = () => {
		setTrashing(true)
		const editing = router.latestLocation.pathname
		void parkNow().then((trashed) => trashPost(postId).then(trashed), unsaved).then(
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
