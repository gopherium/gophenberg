// SPDX-License-Identifier: Apache-2.0

import { AlertDialog } from '@gophenberg/frontend-sdk'
import { __, sprintf } from '@wordpress/i18n'
import { useToaster } from '@gopherium/godmin'
import { useNavigate } from '@tanstack/react-router'

import { useRefresh } from './actions'
import { restorePost, trashPost } from './api'

/**
 * Renders the control moving the post being edited to the trash.
 * @param props - The post to trash and the name it is known by.
 * @returns The trash control.
 */
export function TrashPost({ postId, title }: { postId: string, title: string }) {
	const navigate = useNavigate()
	const refresh = useRefresh()
	const toaster = useToaster()
	/**
	 * Trashes the post, leaves for the listing and offers to take it back.
	 * @returns The error to report, or nothing once the post is trashed.
	 */
	async function trash() {
		try {
			await trashPost(postId)
		} catch {
			return { close: false, error: __('Could not move that post to trash.', 'gophenberg') }
		}
		await navigate({ to: '/content/$typeKey' })
		await refresh([postId])
		toaster.show(__('Moved to the trash.', 'gophenberg'), {
			label: __('Undo', 'gophenberg'),
			onAct: () => {
				restorePost(postId)
					.then(() => refresh([postId]))
					.catch(() => toaster.show(__('Could not restore that post.', 'gophenberg')))
			},
		})
	}
	const description =
		title === ''
			? __('This post goes to the trash. You can restore it from there.', 'gophenberg')
			: sprintf(__('%(title)s goes to the trash. You can restore it from there.', 'gophenberg'), { title })
	return (
		<AlertDialog.Root onConfirm={trash}>
			<AlertDialog.Trigger>{__('Move to trash', 'gophenberg')}</AlertDialog.Trigger>
			<AlertDialog.Popup
				intent="irreversible"
				title={__('Move to trash', 'gophenberg')}
				description={description}
				confirmButtonText={__('Move to trash', 'gophenberg')}
			/>
		</AlertDialog.Root>
	)
}
