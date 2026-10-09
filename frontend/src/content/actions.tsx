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
import { DuplicateModal, RenameModal } from './NameModals'
import type { ContentType } from './types'

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
 * Returns the handler that reloads the listing and forgets the cached copy of each post moved or renamed.
 * @returns The reload handler, taking the ids of the posts that moved into or out of the trash or took a new name.
 */
export function useRefresh(): (moved?: string[]) => Promise<unknown> {
	const client = useQueryClient()
	return useCallback(
		(moved: string[] = []) =>
			Promise.all([
				client.invalidateQueries({ queryKey: ['posts'] }),
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
 * @returns The restore first, a primary action DataViews also draws on the row, then the permanent delete.
 */
function trashViewActions(mine: (post: Post) => boolean, run: RunOver, name: NameCut, type: string): Action<Post>[] {
	const confirmLabel = __('Permanently delete', 'gophenberg')
	return [
		{
			id: 'restore',
			label: _x('Restore', 'trash', 'gophenberg'),
			icon: backupIcon,
			isPrimary: true,
			supportsBulk: true,
			isEligible: mine,
			callback: (items) => run(items, (post) => restorePost(post.id), restoreWords(name, type)),
		},
		{
			id: 'delete',
			label: __('Permanently delete…', 'gophenberg'),
			icon: trashIcon,
			supportsBulk: true,
			isEligible: mine,
			hideModalHeader: true,
			modalFocusOnMount: 'firstContentElement',
			RenderModal: (props) => (
				<RunConfirm
					{...props}
					question={deleteQuestion(props.items, name)}
					confirmLabel={confirmLabel}
					run={(items) => run(items, (post) => deletePost(post.id), deleteWords(name))}
				/>
			),
		},
	]
}

/**
 * Returns the actions offered on the rows of a list outside the trash, the view first and the trash last.
 * @param mine - Whether the session may change a post.
 * @param run - Runs one call over the picked posts.
 * @param name - Cuts a title to the length a toast shows.
 * @param served - Whether the site serves the type the list shows.
 * @returns The view, a primary action DataViews also draws on the row, then the duplicate, offered on every post not
 *   in the trash since every role may create, then the rename and the trash, offered on a post the session may change
 *   that is not in the trash yet.
 */
function listActions(mine: (post: Post) => boolean, run: RunOver, name: NameCut, served: boolean): Action<Post>[] {
	const confirmLabel = _x('Trash', 'verb', 'gophenberg')
	const changeable = (post: Post) => post.status !== 'trash' && mine(post)
	return [
		viewAction(served),
		{
			id: 'duplicate',
			label: _x('Duplicate…', 'action label', 'gophenberg'),
			isEligible: (post) => post.status !== 'trash',
			modalHeader: _x('Duplicate', 'action label', 'gophenberg'),
			modalSize: 'small',
			modalFocusOnMount: 'firstContentElement',
			RenderModal: DuplicateModal,
		},
		{
			id: 'rename',
			label: __('Rename…', 'gophenberg'),
			isEligible: changeable,
			modalHeader: __('Rename', 'gophenberg'),
			modalSize: 'small',
			modalFocusOnMount: 'firstContentElement',
			RenderModal: RenameModal,
		},
		{
			id: 'trash',
			label: _x('Trash…', 'verb', 'gophenberg'),
			icon: trashIcon,
			supportsBulk: true,
			isEligible: changeable,
			hideModalHeader: true,
			modalFocusOnMount: 'firstContentElement',
			RenderModal: (props) => (
				<RunConfirm
					{...props}
					question={trashQuestion(props.items, name)}
					confirmLabel={confirmLabel}
					run={(items) => run(items, (post) => trashPost(post.id), trashWords(name))}
				/>
			),
		},
	]
}

/**
 * Returns the action opening the public address of an item in a new tab, a primary one click action.
 * @param served - Whether the site serves the type the list shows.
 * @returns The view, offered on a published item of a served type, the only kind with a public address.
 */
function viewAction(served: boolean): Action<Post> {
	return {
		id: 'view',
		label: _x('View', 'verb', 'gophenberg'),
		isPrimary: true,
		isEligible: (post) => served && post.status === 'published',
		callback: ([post]) => {
			window.open(`/${post.path}`, '_blank')
		},
	}
}

/**
 * Returns the actions offered on each row of the posts list.
 * @param status - The status the list is filtered by, empty for every status.
 * @param listed - The content type the list shows.
 * @param list - What the list does around a run.
 * @returns The row actions.
 */
export function usePostActions(status: string, listed: ContentType, list: ListRun): Action<Post>[] {
	const run = useRunOver(list)
	const name = useToaster().name
	const session = useSession().data
	const { key, active } = listed
	return useMemo(() => {
		const mine = (post: Post) => sessionMayChange(session, post.authorId)
		if (status === 'trash') {
			return trashViewActions(mine, run, name, key)
		}
		return listActions(mine, run, name, active)
	}, [active, key, name, run, session, status])
}
