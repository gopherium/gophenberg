// SPDX-License-Identifier: Apache-2.0

import { AlertDialog, useSnackbar } from '@gophenberg/frontend-sdk'
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
	const snackbar = useSnackbar()
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
			return { close: false, error: 'Could not move that post to trash.' }
		}
		await navigate({ to: '/posts' })
		await reload()
		snackbar.show('Moved to the trash.', {
			label: 'Undo',
			onAct: () => {
				restorePost(postId)
					.then(reload)
					.catch(() => snackbar.show('Could not restore that post.'))
			},
		})
	}
	return (
		<AlertDialog.Root onConfirm={trash}>
			<AlertDialog.Trigger>Move to trash</AlertDialog.Trigger>
			<AlertDialog.Popup
				intent="irreversible"
				title="Move to trash"
				description={`${title === '' ? 'This post' : title} goes to the trash. You can restore it from there.`}
				confirmButtonText="Move to trash"
			/>
		</AlertDialog.Root>
	)
}
