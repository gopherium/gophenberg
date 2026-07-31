// SPDX-License-Identifier: Apache-2.0

import { Button, Notice, Stack, Text } from '@gophenberg/frontend-sdk'
import type { Action, RenderModalProps } from '@gophenberg/frontend-sdk/dataviews'
import { useMutation, useQueryClient } from '@tanstack/react-query'
import { useNavigate } from '@tanstack/react-router'
import { useMemo } from 'react'

import { trashPost } from './api'
import type { Post } from './api'

/**
 * Renders the confirmation asked for before a post is trashed.
 * @param props - The posts acted on and the handler closing the modal.
 * @returns The confirmation body.
 */
function TrashConfirm({ items, closeModal }: RenderModalProps<Post>) {
	const client = useQueryClient()
	const target = items[0]
	const trash = useMutation({
		mutationFn: () => trashPost(target.id),
		onSuccess: async () => {
			await Promise.all([
				client.invalidateQueries({ queryKey: ['posts'] }),
				client.invalidateQueries({ queryKey: ['post-counts'] }),
			])
			closeModal?.()
		},
	})
	return (
		<Stack direction="column" gap="md">
			<Text>Move {target.title === '' ? '(no title)' : target.title} to the trash?</Text>
			{trash.isError && (
				<Notice.Root intent="error" role="alert">
					<Notice.Description>Could not move that post to trash.</Notice.Description>
				</Notice.Root>
			)}
			<Stack direction="row" gap="sm" justify="flex-end">
				<Button variant="outline" onClick={closeModal}>
					Cancel
				</Button>
				<Button disabled={trash.isPending} onClick={() => trash.mutate()}>
					Move to Trash
				</Button>
			</Stack>
		</Stack>
	)
}

/**
 * Returns the actions offered on each row of the posts list.
 * @returns The row actions.
 */
export function usePostActions(): Action<Post>[] {
	const navigate = useNavigate()
	return useMemo(
		() => [
			{
				id: 'edit',
				label: 'Edit',
				isPrimary: false,
				callback: ([post]: Post[]) => {
					void navigate({ to: '/posts/$postId/edit', params: { postId: post.id } })
				},
			},
			{
				id: 'trash',
				label: 'Move to Trash',
				RenderModal: TrashConfirm,
			},
		],
		[navigate],
	)
}
