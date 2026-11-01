// SPDX-License-Identifier: Apache-2.0

import { backupIcon, sessionMayChange, trashIcon } from '@gophenberg/frontend-sdk'
import type { Action, RenderModalProps } from '@gophenberg/frontend-sdk/dataviews'
import { RunConfirm, useBulkRun, useUnsavedText } from '@gopherium/godmin'
import type { BulkRun, ListRun, UnsavedText } from '@gopherium/godmin'
import { useSession } from '@gopherium/react-auth'
import { __, _x } from '@wordpress/i18n'
import { useQueryClient } from '@tanstack/react-query'
import { useCallback, useMemo } from 'react'

import { deleteQuestion, deleteWords } from '../lists/itemWords'
import { postName, restoreWords, trashQuestion, trashWords } from './actionWords'
import { deletePost, restorePost, trashPost } from './api'
import type { Post } from './api'
import { DuplicateModal, RenamePostModal } from './NameModals'
import type { ContentType } from './types'

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
				...moved.map((id) => client.resetQueries({ queryKey: ['post', id], exact: true })),
			]),
		[client],
	)
}

/**
 * Returns the handler running one call over each post, reloading the listing and forgetting each post it acted on.
 * @param list - What the list does around a run.
 * @returns The run handler.
 */
function useRunOver(list: ListRun): BulkRun<Post> {
	const refresh = useRefresh()
	const refreshPosts = useCallback((posts: Post[]) => refresh(posts.map((post) => post.id)), [refresh])
	return useBulkRun(list, refreshPosts)
}

/** What a confirm of the posts list takes beside the props DataViews hands a modal. */
interface PostConfirmProps extends RenderModalProps<Post> {
	/** Runs one call over the picked posts. */
	run: BulkRun<Post>
}

/**
 * Renders the confirm moving the picked posts to the trash, closing only itself once the run ends.
 * @param props - The posts, the handler closing the modal and the run.
 * @returns The confirm.
 */
function TrashConfirm({ items, closeModal, run }: PostConfirmProps) {
	return (
		<RunConfirm
			items={items}
			closeModal={closeModal}
			confirmLabel={__('Move to trash', 'gophenberg')}
			cancelLabel={__('Cancel', 'gophenberg')}
			run={(picked) => run(picked, (post) => trashPost(post.id), trashWords())}
		>
			{trashQuestion(items)}
		</RunConfirm>
	)
}

/**
 * Renders the confirm deleting the picked posts for good, closing only itself once the run ends.
 * @param props - The posts, the handler closing the modal and the run.
 * @returns The confirm.
 */
function DeleteConfirm({ items, closeModal, run }: PostConfirmProps) {
	return (
		<RunConfirm
			items={items}
			closeModal={closeModal}
			confirmLabel={__('Delete permanently', 'gophenberg')}
			cancelLabel={__('Cancel', 'gophenberg')}
			run={(picked) => run(picked, (post) => deletePost(post.id), deleteWords(postName))}
		>
			{deleteQuestion(items, postName)}
		</RunConfirm>
	)
}

/**
 * Returns the actions offered on the rows of a trash view.
 * @param mine - Whether the session may change a post.
 * @param run - Runs one call over the picked posts.
 * @param type - The content type the list shows.
 * @returns The restore first, a primary action DataViews also draws on the row, then the permanent delete.
 */
function trashViewActions(mine: (post: Post) => boolean, run: BulkRun<Post>, type: string): Action<Post>[] {
	return [
		{
			id: 'restore',
			label: _x('Restore', 'trash', 'gophenberg'),
			icon: backupIcon,
			isPrimary: true,
			supportsBulk: true,
			isEligible: mine,
			callback: (items) => run(items, (post) => restorePost(post.id), restoreWords(type)),
		},
		{
			id: 'delete',
			label: __('Permanently delete…', 'gophenberg'),
			icon: trashIcon,
			supportsBulk: true,
			isEligible: mine,
			hideModalHeader: true,
			modalFocusOnMount: 'firstContentElement',
			RenderModal: (props) => <DeleteConfirm {...props} run={run} />,
		},
	]
}

/** The text failed row forms left unsaved, one store per form. */
interface UnsavedForms {
	/** The titles failed copies left unsaved. */
	copies: UnsavedText
	/** The names failed renames left unsaved. */
	renames: UnsavedText
}

/**
 * Returns the actions offered on the rows of a list outside the trash, the view first and the trash last.
 * @param mine - Whether the session may change a post.
 * @param run - Runs one call over the picked posts.
 * @param served - Whether the site serves the type the list shows.
 * @param unsaved - The text failed row forms left unsaved.
 * @returns The view, a primary action DataViews also draws on the row, then the duplicate, offered on every post not
 *   in the trash since every role may create, then the rename and the trash, offered on a post the session may change
 *   that is not in the trash yet.
 */
function listActions(
	mine: (post: Post) => boolean,
	run: BulkRun<Post>,
	served: boolean,
	unsaved: UnsavedForms,
): Action<Post>[] {
	const changeable = (post: Post) => post.status !== 'trash' && mine(post)
	return [
		viewAction(served),
		{
			id: 'duplicate',
			label: _x('Duplicate…', 'action label', 'gophenberg'),
			isEligible: (post) => post.status !== 'trash',
			modalHeader: _x('Duplicate', 'action label', 'gophenberg'),
			modalFocusOnMount: 'firstContentElement',
			RenderModal: (props) => <DuplicateModal {...props} unsaved={unsaved.copies} run={run} />,
		},
		{
			id: 'rename',
			label: __('Rename…', 'gophenberg'),
			isEligible: changeable,
			modalHeader: __('Rename', 'gophenberg'),
			modalFocusOnMount: 'firstContentElement',
			RenderModal: (props) => <RenamePostModal {...props} unsaved={unsaved.renames} run={run} />,
		},
		{
			id: 'trash',
			label: _x('Trash…', 'verb', 'gophenberg'),
			icon: trashIcon,
			supportsBulk: true,
			isEligible: changeable,
			hideModalHeader: true,
			modalFocusOnMount: 'firstContentElement',
			RenderModal: (props) => <TrashConfirm {...props} run={run} />,
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
	const session = useSession().data
	const copies = useUnsavedText()
	const renames = useUnsavedText()
	const { key, active } = listed
	return useMemo(() => {
		const mine = (post: Post) => sessionMayChange(session, post.authorId)
		if (status === 'trash') {
			return trashViewActions(mine, run, key)
		}
		return listActions(mine, run, active, { copies, renames })
	}, [active, copies, key, renames, run, session, status])
}
