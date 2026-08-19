// SPDX-License-Identifier: Apache-2.0

import { AlertDialog } from '@gophenberg/frontend-sdk'
import { __, sprintf } from '@wordpress/i18n'
import { useToaster } from '@gopherium/godmin'
import { useQueryClient } from '@tanstack/react-query'
import { useNavigate } from '@tanstack/react-router'

import { restorePost, trashPost } from './api'

/**
 * Renders the control moving the post being edited to the trash.
 * @param props - The post to trash and the name it is known by.
 * @returns The trash control.
 */
export function TrashPost({ postId, title }: { postId: string, title: string }) {
	const navigate = useNavigate()
	const client = useQueryClient()
	const toaster = useToaster()
	/**
	 * Reloads the listing and the status counts.
	 */
	async function reload() {
		await Promise.all([
			client.invalidateQueries({ queryKey: ['posts'] }),
			client.invalidateQueries({ queryKey: ['post-counts'] }),
		])
	}
	/**
	 * Trashes the post, leaves for the listing and offers to take it back.
	 * @returns The refusal to report, or nothing once the post is trashed.
	 */
	async function trash() {
		try {
			await trashPost(postId)
		} catch {
			return { close: false, error: __('Could not move that post to trash.', 'gophenberg') }
		}
		await navigate({ to: '/content/$typeKey' })
		await reload()
		toaster.show(__('Moved to the trash.', 'gophenberg'), {
			label: __('Undo', 'gophenberg'),
			onAct: () => {
				restorePost(postId)
					.then(reload)
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
