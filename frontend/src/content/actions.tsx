// SPDX-License-Identifier: Apache-2.0

import { Button, Notice, Stack, Text } from '@gophenberg/frontend-sdk'
import type { Action, RenderModalProps } from '@gophenberg/frontend-sdk/dataviews'
import { __, _n, _x, sprintf } from '@wordpress/i18n'
import { useMutation, useQueryClient } from '@tanstack/react-query'
import { useNavigate } from '@tanstack/react-router'
import { useCallback, useMemo } from 'react'

import { deletePost, restorePost, trashPost } from './api'
import type { Post } from './api'

export interface PostNotice {
	intent: 'error' | 'success'
	message: string
	undoIds?: string[]
}

export type ReportNotice = (notice: PostNotice | null) => void

/**
 * Returns the name a post is listed under.
 * @param post - The post to name.
 * @returns The title, or a stand in for a post that has none.
 */
function nameOf(post: Post): string {
	return post.title === '' ? __('(no title)', 'gophenberg') : post.title
}

/**
 * Returns the question asked before the given posts are trashed.
 * @param items - The posts acted on.
 * @returns The question to show.
 */
function trashQuestion(items: Post[]): string {
	if (items.length === 1) {
		return sprintf(__('Move %s to the trash?', 'gophenberg'), nameOf(items[0]))
	}
	const many = _n(
		'Move these %d post to the trash?',
		'Move these %d posts to the trash?',
		items.length,
		'gophenberg',
	)
	return sprintf(many, items.length)
}

/**
 * Returns the note carried to the screen once the posts reach the trash.
 * @param count - How many posts were trashed.
 * @returns The note to show.
 */
function trashedNote(count: number): string {
	if (count === 1) {
		return __('Moved to the trash.', 'gophenberg')
	}
	const many = _n('%d post moved to the trash.', '%d posts moved to the trash.', count, 'gophenberg')
	return sprintf(many, count)
}

/**
 * Returns the handler that reloads the listing and the status counts.
 * @returns The reload handler.
 */
export function useRefresh(): () => Promise<unknown> {
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
	done,
}: {
	question: string
	failure: string
	confirmLabel: string
	run: () => Promise<unknown>
	closeModal?: () => void
	done?: () => void
}) {
	const refresh = useRefresh()
	const action = useMutation({
		mutationFn: run,
		onSuccess: async () => {
			done?.()
			await refresh()
			closeModal?.()
		},
		onError: () => {
			void refresh()
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
					{__('Cancel', 'gophenberg')}
				</Button>
				<Button loading={action.isPending} onClick={() => action.mutate()}>
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
function TrashConfirm({
	items,
	closeModal,
	report,
}: RenderModalProps<Post> & { report: ReportNotice }) {
	const single = items.length === 1
	return (
		<Confirm
			question={trashQuestion(items)}
			failure={
				single
					? __('Could not move that post to trash.', 'gophenberg')
					: __('Could not move every post to trash.', 'gophenberg')
			}
			confirmLabel={__('Move to Trash', 'gophenberg')}
			run={async () => {
				const settled = await Promise.allSettled(items.map((post) => trashPost(post.id)))
				if (settled.some((outcome) => outcome.status === 'rejected')) {
					throw new Error('trashing did not finish')
				}
			}}
			closeModal={closeModal}
			done={() =>
				report({
					intent: 'success',
					message: trashedNote(items.length),
					undoIds: items.map((post) => post.id),
				})
			}
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
			question={sprintf(__('Delete %s for good? This cannot be undone.', 'gophenberg'), nameOf(target))}
			failure={__('Could not delete that post.', 'gophenberg')}
			confirmLabel={__('Delete Permanently', 'gophenberg')}
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
export function usePostActions(status: string, report: ReportNotice): Action<Post>[] {
	const navigate = useNavigate()
	const refresh = useRefresh()
	return useMemo(() => {
		if (status === 'trash') {
			return [
				{
					id: 'restore',
					label: _x('Restore', 'trash', 'gophenberg'),
					callback: ([post]: Post[]) => {
						restorePost(post.id)
							.then(() => {
								report(null)
								return refresh()
							})
							.catch(() =>
							report({
								intent: 'error',
								message: __('Could not restore that post.', 'gophenberg'),
							}),
						)
					},
				},
				{
					id: 'delete',
					label: __('Delete Permanently', 'gophenberg'),
					RenderModal: DeleteConfirm,
				},
			]
		}
		return [
			{
				id: 'edit',
				label: __('Edit', 'gophenberg'),
				callback: ([post]: Post[]) => {
					void navigate({ to: '/content/$typeKey/$postId/edit', params: { typeKey: post.type, postId: post.id } })
				},
			},
			{
				id: 'trash',
				label: __('Move to Trash', 'gophenberg'),
				supportsBulk: true,
				RenderModal: (props: RenderModalProps<Post>) => (
					<TrashConfirm {...props} report={report} />
				),
			},
		]
	}, [navigate, refresh, report, status])
}
