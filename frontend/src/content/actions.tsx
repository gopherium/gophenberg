// SPDX-License-Identifier: Apache-2.0

import { backupIcon, sessionMayChange, trashIcon } from '@gophenberg/frontend-sdk'
import type { Action, RenderModalProps } from '@gophenberg/frontend-sdk/dataviews'
import { ConfirmBody, bulkNotes, runEach, useToaster } from '@gopherium/godmin'
import type { BulkWords } from '@gopherium/godmin'
import { useSession } from '@gopherium/react-auth'
import { __, _x } from '@wordpress/i18n'
import { useQueryClient } from '@tanstack/react-query'
import { useCallback, useMemo, useState } from 'react'

import { deleteQuestion, deleteWords, restoreWords, trashQuestion, trashWords } from './actionWords'
import type { NameCut } from './actionWords'
import { deletePost, restorePost, trashPost } from './api'
import type { Post } from './api'

/** Shows the failure of a run above the list, none when every post finished, and focuses the list once a post moved. */
type RunSettled = (failure: string | undefined, moved: boolean) => void

/** What the list does around a run over its posts. */
export interface ListRun {
	/** Clears the failure the last run left above the list and returns what settles this run on the list. */
	onStart: () => RunSettled
}

/** Runs one call over each post and reports what the run reached in the given words. */
type RunOver = (items: Post[], call: (post: Post) => Promise<unknown>, words: BulkWords<Post>) => Promise<void>

/**
 * Returns the handler that reloads the listing and the status counts and forgets the cached copy of each post moved.
 * @returns The reload handler, taking the ids of the posts that moved into or out of the trash.
 */
export function useRefresh(): (moved?: string[]) => Promise<unknown> {
	const client = useQueryClient()
	return useCallback(
		(moved: string[] = []) =>
			Promise.all([
				client.invalidateQueries({ queryKey: ['posts'] }),
				client.invalidateQueries({ queryKey: ['post-counts'] }),
				...moved.map((id) => client.removeQueries({ queryKey: ['post', id], exact: true })),
			]),
		[client],
	)
}

/**
 * Returns the handler running one call over each post and reporting what the run reached.
 * @param list - What the list does around a run.
 * @returns The run handler.
 */
function useRunOver(list: ListRun): RunOver {
	const refresh = useRefresh()
	const toaster = useToaster()
	return useCallback(
		async (items, call, words) => {
			const settle = list.onStart()
			const outcome = await runEach(items, call)
			await refresh(items.map((post) => post.id))
			const notes = bulkNotes(items, outcome, words)
			if (notes.toast !== undefined) {
				toaster.show(notes.toast)
			}
			settle(notes.notice, outcome.done > 0)
		},
		[list, refresh, toaster],
	)
}

/**
 * Renders the confirmation asked for before a run over the picked posts, closing once the run settles.
 * @param props - The posts, the handler closing the modal, the question, the confirm label and the run.
 * @returns The confirmation body.
 */
function RunConfirm({
	items,
	closeModal,
	question,
	confirmLabel,
	run,
}: RenderModalProps<Post> & { question: string, confirmLabel: string, run: (items: Post[]) => Promise<void> }) {
	const [busy, setBusy] = useState(false)
	return (
		<ConfirmBody
			confirmLabel={confirmLabel}
			cancelLabel={__('Cancel', 'gophenberg')}
			busy={busy}
			onCancel={busy ? undefined : closeModal}
			onConfirm={() => {
				setBusy(true)
				void run(items).then(() => closeModal?.())
			}}
		>
			{question}
		</ConfirmBody>
	)
}

/**
 * Returns the actions offered on the rows of a trash view.
 * @param mine - Whether the session may change a post.
 * @param run - Runs one call over the picked posts.
 * @param name - Cuts a title to the length a toast shows.
 * @param type - The content type the list shows.
 * @returns The restore first, then the permanent delete.
 */
function trashViewActions(mine: (post: Post) => boolean, run: RunOver, name: NameCut, type: string): Action<Post>[] {
	const label = __('Permanently delete', 'gophenberg')
	return [
		{
			id: 'restore',
			label: _x('Restore', 'trash', 'gophenberg'),
			icon: backupIcon,
			supportsBulk: true,
			isEligible: mine,
			callback: (items) => run(items, (post) => restorePost(post.id), restoreWords(name, type)),
		},
		{
			id: 'delete',
			label,
			icon: trashIcon,
			supportsBulk: true,
			isEligible: mine,
			modalHeader: label,
			modalSize: 'small',
			RenderModal: (props) => (
				<RunConfirm
					{...props}
					question={deleteQuestion(props.items, name)}
					confirmLabel={label}
					run={(items) => run(items, (post) => deletePost(post.id), deleteWords(name))}
				/>
			),
		},
	]
}

/**
 * Returns the actions offered on the rows of a list outside the trash.
 * @param mine - Whether the session may change a post.
 * @param run - Runs one call over the picked posts.
 * @param name - Cuts a title to the length a toast shows.
 * @returns The trash, offered on a post not in the trash yet.
 */
function listActions(mine: (post: Post) => boolean, run: RunOver, name: NameCut): Action<Post>[] {
	const label = _x('Trash', 'verb', 'gophenberg')
	return [
		{
			id: 'trash',
			label,
			icon: trashIcon,
			supportsBulk: true,
			isEligible: (post) => post.status !== 'trash' && mine(post),
			modalHeader: label,
			modalSize: 'small',
			RenderModal: (props) => (
				<RunConfirm
					{...props}
					question={trashQuestion(props.items, name)}
					confirmLabel={label}
					run={(items) => run(items, (post) => trashPost(post.id), trashWords(name))}
				/>
			),
		},
	]
}

/**
 * Returns the actions offered on each row of the posts list.
 * @param status - The status the list is filtered by, empty for every status.
 * @param type - The content type the list shows.
 * @param list - What the list does around a run.
 * @returns The row actions.
 */
export function usePostActions(status: string, type: string, list: ListRun): Action<Post>[] {
	const run = useRunOver(list)
	const name = useToaster().name
	const session = useSession().data
	return useMemo(() => {
		const mine = (post: Post) => sessionMayChange(session, post.authorId)
		if (status === 'trash') {
			return trashViewActions(mine, run, name, type)
		}
		return listActions(mine, run, name)
	}, [name, run, session, status, type])
}
