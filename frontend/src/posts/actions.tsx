// SPDX-License-Identifier: Apache-2.0

import { Button, Notice, Stack, Text } from '@gophenberg/frontend-sdk'
import type { Action, RenderModalProps } from '@gophenberg/frontend-sdk/dataviews'
import { useMutation, useQueryClient } from '@tanstack/react-query'
import { useNavigate } from '@tanstack/react-router'
import { useCallback, useMemo } from 'react'

import { deletePost, restorePost, trashPost } from './api'
import type { Post } from './api'

/**
 * Returns the name a post is listed under.
 * @param post - The post to name.
 * @returns The title, or a stand in for a post that has none.
 */
function nameOf(post: Post): string {
	return post.title === '' ? '(no title)' : post.title
}

/**
 * Returns the handler that reloads the listing and the status counts.
 * @returns The reload handler.
 */
function useRefresh(): () => Promise<unknown> {
	const client = useQueryClient()
	return useCallback(
		() =>
			Promise.all([
				client.invalidateQueries({ queryKey: ['posts'] }),
				client.invalidateQueries({ queryKey: ['post-counts'] }),
			]),
		[client],
	)
}

/**
 * Renders a confirmation body over the given work.
 * @param props - The question, the failure to report, the button label and the work to run.
 * @returns The confirmation body.
 */
function Confirm({
	question,
	failure,
	confirmLabel,
	run,
	closeModal,
}: {
	question: string
	failure: string
	confirmLabel: string
	run: () => Promise<unknown>
	closeModal?: () => void
}) {
	const refresh = useRefresh()
	const action = useMutation({
		mutationFn: run,
		onSuccess: async () => {
			await refresh()
			closeModal?.()
		},
	})
	return (
		<Stack direction="column" gap="md">
			<Text>{question}</Text>
			{action.isError && (
				<Notice.Root intent="error" role="alert">
					<Notice.Description>{failure}</Notice.Description>
				</Notice.Root>
			)}
			<Stack direction="row" gap="sm" justify="flex-end">
				<Button variant="outline" onClick={closeModal}>
					Cancel
				</Button>
				<Button disabled={action.isPending} onClick={() => action.mutate()}>
					{confirmLabel}
				</Button>
			</Stack>
		</Stack>
	)
}

/**
 * Renders the confirmation asked for before a post is trashed.
 * @param props - The posts acted on and the handler closing the modal.
 * @returns The confirmation body.
 */
function TrashConfirm({ items, closeModal }: RenderModalProps<Post>) {
	const target = items[0]
	return (
		<Confirm
			question={`Move ${nameOf(target)} to the trash?`}
			failure="Could not move that post to trash."
			confirmLabel="Move to Trash"
			run={() => trashPost(target.id)}
			closeModal={closeModal}
		/>
	)
}

/**
 * Renders the confirmation asked for before a post is deleted for good.
 * @param props - The posts acted on and the handler closing the modal.
 * @returns The confirmation body.
 */
function DeleteConfirm({ items, closeModal }: RenderModalProps<Post>) {
	const target = items[0]
	return (
		<Confirm
			question={`Delete ${nameOf(target)} for good? This cannot be undone.`}
			failure="Could not delete that post."
			confirmLabel="Delete Permanently"
			run={() => deletePost(target.id)}
			closeModal={closeModal}
		/>
	)
}

/**
 * Returns the actions offered on each row of the posts list.
 * @param status - The status the list is filtered by, empty for every status.
 * @param report - The handler carrying a failure to the screen.
 * @returns The row actions.
 */
export function usePostActions(
	status: string,
	report: (failure: string | null) => void,
): Action<Post>[] {
	const navigate = useNavigate()
	const refresh = useRefresh()
	return useMemo(() => {
		if (status === 'trash') {
			return [
				{
					id: 'restore',
					label: 'Restore',
					callback: ([post]: Post[]) => {
						restorePost(post.id)
							.then(() => {
								report(null)
								return refresh()
							})
							.catch(() => report('Could not restore that post.'))
					},
				},
				{
					id: 'delete',
					label: 'Delete Permanently',
					RenderModal: DeleteConfirm,
				},
			]
		}
		return [
			{
				id: 'edit',
				label: 'Edit',
				callback: ([post]: Post[]) => {
					void navigate({ to: '/posts/$postId/edit', params: { postId: post.id } })
				},
			},
			{
				id: 'trash',
				label: 'Move to Trash',
				RenderModal: TrashConfirm,
			},
		]
	}, [navigate, refresh, report, status])
}
