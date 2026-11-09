// SPDX-License-Identifier: Apache-2.0

import { backupIcon, sessionMayChange, trashIcon } from '@gophenberg/frontend-sdk'
import type { Action } from '@gophenberg/frontend-sdk/dataviews'
import { ConfirmDialog, focusedTrigger, useBulkRun, useDialogRun } from '@gopherium/godmin'
import type { BulkRun, DialogRun, ListRun, Trigger } from '@gopherium/godmin'
import { useSession } from '@gopherium/react-auth'
import { __, _x } from '@wordpress/i18n'
import { useQueryClient } from '@tanstack/react-query'
import { useCallback, useMemo } from 'react'

import { deleteTitle, deleteWords } from '../lists/itemWords'
import { deleteDescription, postName, restoreWords, trashWords } from './actionWords'
import { deletePost, restorePost, trashPost } from './api'
import type { Post } from './api'
import { DuplicateDialog, RenamePostDialog } from './NameModals'
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
	/** Runs the write of a row dialog and answers the dialog, the list reloading once the write ended. */
	write: DialogRun<Post>
	/** Takes the posts with the given ids off the ticked rows, keeping every other tick. */
	unpick: (ids: string[]) => void
}

/** A dialog the screen holds open on the picked posts. */
export interface OpenedPosts {
	/** The action the dialog belongs to. */
	action: 'delete' | 'duplicate' | 'rename'
	/** The posts the dialog is open on. */
	items: Post[]
	/** The trigger the dialog opened from, which takes the focus back once the dialog is done. */
	trigger: Trigger
}

/**
 * Returns what the actions and the dialogs of the posts list need, the same for as long as the screen stays.
 * @param list - What the list does around a run.
 * @param unpick - Takes the posts with the given ids off the ticked rows.
 * @returns The tools, whose run and write reload the listing and forget each post they acted on.
 */
export function usePostTools(list: ListRun, unpick: (ids: string[]) => void): PostTools {
	const refresh = useRefresh()
	const refreshPosts = useCallback((posts: Post[]) => refresh(posts.map((post) => post.id)), [refresh])
	const run = useBulkRun(list, refreshPosts)
	const write = useDialogRun(list, refreshPosts)
	return useMemo(() => ({ run, write, unpick }), [run, write, unpick])
}

/** The row forms the screen holds open on one post, by the action they belong to. */
const ROW_FORMS = { duplicate: DuplicateDialog, rename: RenamePostDialog }

/**
 * Renders the dialog the screen holds open on the picked posts, none while no dialog is open.
 * @param props - The dialog, its posts and the trigger it opened from, what it needs from the screen, what takes it
 *   away and what puts a lost focus back on the trigger it opened from.
 * @returns The duplicate or the rename of one post, or the confirm deleting the posts for good, each busy until its
 *   write ends and giving the focus back to the trigger it opened from once it closed.
 */
export function OpenPostDialog({ opened, tools, onClose, onBack }: {
	opened?: OpenedPosts
	tools: PostTools
	onClose: () => void
	onBack: (id: string, trigger: Trigger) => void
}) {
	if (opened === undefined) {
		return null
	}
	const [post] = opened.items
	const close = () => {
		onClose()
		onBack(post.id, opened.trigger)
	}
	if (opened.action !== 'delete') {
		const Form = ROW_FORMS[opened.action]
		return <Form key={`${opened.action} ${post.id}`} post={post} write={tools.write} onClose={close} />
	}
	return (
		<ConfirmDialog
			intent="irreversible"
			title={deleteTitle(opened.items, postName)}
			description={deleteDescription(opened.items.length)}
			confirmLabel={__('Delete permanently', 'gophenberg')}
			cancelLabel={__('Cancel', 'gophenberg')}
			onConfirm={() => tools.run(opened.items, (item) => deletePost(item.id), deleteWords(postName))}
			onClose={close}
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

/**
 * Returns the trash over the picked posts, run at once, its toast offering Undo, every post it moved left unticked.
 * @param tools - Runs one call over the picked posts and takes the posts it moved off the ticked rows.
 * @param type - The content type the list shows.
 * @returns The handler the trash action calls.
 */
function trashing(tools: PostTools, type: string): (posts: Post[]) => Promise<void> {
	const { run, unpick } = tools
	return async (posts) => {
		const moved: string[] = []
		await run(posts, (post) => trashPost(post.id).then(() => moved.push(post.id)), {
			...trashWords(),
			undo: (done) => ({
				label: __('Undo', 'gophenberg'),
				onAct: () => void run(done, (post) => restorePost(post.id), restoreWords(type)),
			}),
		})
		unpick(moved)
	}
}

/**
 * Returns the actions offered on the rows of a list outside the trash, the view first and the trash last.
 * @param mine - Whether the session may change a post.
 * @param tools - What the actions need from the screen.
 * @param type - The content type the list shows.
 * @param served - Whether the site serves the type the list shows.
 * @param open - Opens the duplicate or the rename on a post in a dialog the screen holds.
 * @returns The view, a primary action DataViews also draws on the row, then the duplicate, offered on every post not
 *   in the trash since every role may create, then the rename and the trash, offered on a post the session may change
 *   that is not in the trash yet.
 */
function listActions(
	mine: (post: Post) => boolean,
	tools: PostTools,
	type: string,
	served: boolean,
	open: (opened: OpenedPosts) => void,
): Action<Post>[] {
	const changeable = (post: Post) => post.status !== 'trash' && mine(post)
	return [
		viewAction(served),
		{
			id: 'duplicate',
			label: _x('Duplicate…', 'action label', 'gophenberg'),
			isEligible: (post) => post.status !== 'trash',
			callback: (items) => open({ action: 'duplicate', items, trigger: focusedTrigger() }),
		},
		{
			id: 'rename',
			label: __('Rename…', 'gophenberg'),
			isEligible: changeable,
			callback: (items) => open({ action: 'rename', items, trigger: focusedTrigger() }),
		},
		{
			id: 'trash',
			label: _x('Trash', 'verb', 'gophenberg'),
			icon: trashIcon,
			supportsBulk: true,
			isEligible: changeable,
			callback: trashing(tools, type),
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
 * @param open - Opens a dialog on the picked posts that the screen holds.
 * @returns The row actions.
 */
export function usePostActions(
	status: string,
	listed: ContentType,
	tools: PostTools,
	open: (opened: OpenedPosts) => void,
): Action<Post>[] {
	const session = useSession().data
	const { key, active } = listed
	return useMemo(() => {
		const mine = (post: Post) => sessionMayChange(session, post.authorId)
		if (status === 'trash') {
			return trashViewActions(mine, tools.run, key, open)
		}
		return listActions(mine, tools, key, active, open)
	}, [active, key, open, session, status, tools])
}
