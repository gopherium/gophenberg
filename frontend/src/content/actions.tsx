// SPDX-License-Identifier: Apache-2.0

import { backupIcon, sessionMayChange, trashIcon } from '@gophenberg/frontend-sdk'
import type { Action } from '@gophenberg/frontend-sdk/dataviews'
import { ConfirmDialog, focusedTrigger, useBulkRun, useUnsavedText } from '@gopherium/godmin'
import type { BulkRun, ListRun, Trigger, UnsavedText } from '@gopherium/godmin'
import { useSession } from '@gopherium/react-auth'
import { __, _x } from '@wordpress/i18n'
import { useQueryClient } from '@tanstack/react-query'
import { useCallback, useMemo } from 'react'

import { deleteTitle, deleteWords, irreversible } from '../lists/itemWords'
import { postName, restoreWords, trashWords } from './actionWords'
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

/** What the actions and the dialogs of the posts list need from the screen around them. */
interface PostTools {
	/** Runs one call over posts, the list reloading once the last call ended. */
	run: BulkRun<Post>
}

/** A confirm the screen holds open on the picked posts. */
export interface OpenedPosts {
	/** The action the confirm belongs to. */
	action: 'delete'
	/** The posts the confirm is open on. */
	items: Post[]
	/** The trigger the confirm opened from, which takes the focus back once the confirm is done. */
	trigger: Trigger
}

/**
 * Returns what the actions and the dialogs of the posts list need, the same for as long as the screen stays.
 * @param list - What the list does around a run.
 * @returns The tools, whose run reloads the listing and forgets each post it acted on.
 */
export function usePostTools(list: ListRun): PostTools {
	const refresh = useRefresh()
	const refreshPosts = useCallback((posts: Post[]) => refresh(posts.map((post) => post.id)), [refresh])
	const run = useBulkRun(list, refreshPosts)
	return useMemo(() => ({ run }), [run])
}

/**
 * Renders the confirm the screen holds open on the picked posts, none while no confirm is open.
 * @param props - The confirm, its posts and the trigger it opened from, what it needs from the screen and what takes
 *   it away.
 * @returns The confirm deleting the posts for good, busy until the run ends.
 */
export function OpenPostDialog({ opened, tools, onClose }: {
	opened?: OpenedPosts
	tools: PostTools
	onClose: () => void
}) {
	if (opened === undefined) {
		return null
	}
	return (
		<ConfirmDialog
			intent="irreversible"
			title={deleteTitle(opened.items, postName)}
			description={irreversible()}
			confirmLabel={__('Delete permanently', 'gophenberg')}
			cancelLabel={__('Cancel', 'gophenberg')}
			onConfirm={() => tools.run(opened.items, (post) => deletePost(post.id), deleteWords(postName))}
			onClose={onClose}
		/>
	)
}

/**
 * Returns the actions offered on the rows of a trash view.
 * @param mine - Whether the session may change a post.
 * @param run - Runs one call over the picked posts.
 * @param type - The content type the list shows.
 * @param open - Opens the permanent delete on the picked posts in a confirm the screen holds.
 * @returns The restore first, a primary action DataViews also draws on the row, then the permanent delete.
 */
function trashViewActions(
	mine: (post: Post) => boolean,
	run: BulkRun<Post>,
	type: string,
	open: (opened: OpenedPosts) => void,
): Action<Post>[] {
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
			callback: (items) => open({ action: 'delete', items, trigger: focusedTrigger() }),
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
 * @param type - The content type the list shows.
 * @param served - Whether the site serves the type the list shows.
 * @param unsaved - The text failed row forms left unsaved.
 * @returns The view, a primary action DataViews also draws on the row, then the duplicate, offered on every post not
 *   in the trash since every role may create, then the rename and the trash, offered on a post the session may change
 *   that is not in the trash yet.
 */
function listActions(
	mine: (post: Post) => boolean,
	run: BulkRun<Post>,
	type: string,
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
			label: _x('Trash', 'verb', 'gophenberg'),
			icon: trashIcon,
			supportsBulk: true,
			isEligible: changeable,
			callback: (posts) =>
				run(posts, (post) => trashPost(post.id), {
					...trashWords(),
					undo: (done) => ({
						label: __('Undo', 'gophenberg'),
						onAct: () => void run(done, (post) => restorePost(post.id), restoreWords(type)),
					}),
				}),
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
 * @param tools - What the actions need from the screen.
 * @param open - Opens a confirm on the picked posts that the screen holds.
 * @returns The row actions.
 */
export function usePostActions(
	status: string,
	listed: ContentType,
	tools: PostTools,
	open: (opened: OpenedPosts) => void,
): Action<Post>[] {
	const { run } = tools
	const session = useSession().data
	const copies = useUnsavedText()
	const renames = useUnsavedText()
	const { key, active } = listed
	return useMemo(() => {
		const mine = (post: Post) => sessionMayChange(session, post.authorId)
		if (status === 'trash') {
			return trashViewActions(mine, run, key, open)
		}
		return listActions(mine, run, key, active, { copies, renames })
	}, [active, copies, key, open, renames, run, session, status])
}
