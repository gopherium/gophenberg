// SPDX-License-Identifier: Apache-2.0

import { InputControl } from '@gophenberg/frontend-sdk'
import { FormDialog, RenameDialog } from '@gopherium/godmin'
import type { DialogRun } from '@gopherium/godmin'
import { useQueryClient } from '@tanstack/react-query'
import type { QueryClient } from '@tanstack/react-query'
import { __, _x, sprintf } from '@wordpress/i18n'
import { useState } from 'react'

import { nameWords, oneWriteWords } from '../lists/itemWords'
import { createPost, fetchPost, renamePost } from './api'
import type { Post } from './api'
import { shownValues } from './conditions'
import { holdWrite, writesSettled } from './postWrites'
import type { ContentField } from './types'
import { useContentType } from './useContentType'

/** What the copy of an item takes from the dialog. */
interface CopyAsked {
	/** The title of the copy. */
	title: string
	/** The fields the type declares, judging which values the rules show. */
	declared: ContentField[]
}

/**
 * Makes a draft holding the title given and the content, excerpt, parent and the field values of an item the server
 * writes, the ones its conditions show less the Linked from fields it only reads, read once every write still in
 * flight to the item has answered.
 * @param client - The query client holding the writes in flight.
 * @param post - The item to copy.
 * @param asked - The title of the copy and the fields the type declares.
 * @param signal - The signal that stops the read and the create once the dialog's time limit ran out.
 * @returns The copy.
 */
async function duplicated(client: QueryClient, post: Post, asked: CopyAsked, signal: AbortSignal): Promise<Post> {
	const fallback = __('The item could not be duplicated.', 'gophenberg')
	await writesSettled(client, post.id)
	const source = await fetchPost(post.id, fallback, signal)
	const draft = {
		title: asked.title,
		content: source.content,
		excerpt: source.excerpt,
		parentId: source.parentId ?? undefined,
		fields: shownValues(asked.declared, source.fields),
	}
	return createPost(post.type, draft, fallback, signal)
}

/** What a row dialog the posts screen holds open on one item takes. */
interface PostDialogProps {
	/** The item the dialog is open on. */
	post: Post
	/** Runs the write over the item and answers the dialog, the list reloading once it ended. */
	write: DialogRun<Post>
	/** Takes the dialog away, once its card has left the page. */
	onClose: () => void
}

/**
 * Renders the dialog copying one item into a new draft under the title given, open until the copy is made.
 * @param props - The item, the write and what takes the dialog away.
 * @returns The duplicate dialog, its title starting from a copy of the item's own on each open.
 */
export function DuplicateDialog({ post, write, onClose }: PostDialogProps) {
	const client = useQueryClient()
	const { fields } = useContentType()
	const [title, setTitle] = useState<string>(() => sprintf(_x('%s (Copy)', 'post', 'gophenberg'), post.title))
	const made = oneWriteWords<Post>(sprintf(__('"%s" successfully created.', 'gophenberg'), title))
	return (
		<FormDialog
			title={_x('Duplicate', 'action label', 'gophenberg')}
			submitLabel={_x('Duplicate', 'action label', 'gophenberg')}
			cancelLabel={__('Cancel', 'gophenberg')}
			onSubmit={(signal) =>
				write([post], () => duplicated(client, post, { title, declared: fields }, signal), made, signal)
			}
			onClose={onClose}
		>
			{(busy) => (
				<InputControl
					label={__('Title', 'gophenberg')}
					value={title}
					readOnly={busy}
					onChange={(event) => setTitle(event.target.value)}
				/>
			)}
		</FormDialog>
	)
}

/**
 * Writes the title given to an item, its version read once every write still in flight to the item has answered.
 * @param client - The query client holding the writes in flight.
 * @param post - The item to rename.
 * @param title - The new title.
 * @param signal - The signal that stops the rename once the dialog's time limit ran out.
 * @returns The renamed item.
 */
async function renamed(client: QueryClient, post: Post, title: string, signal: AbortSignal): Promise<Post> {
	await writesSettled(client, post.id)
	return renamePost(post.id, title, signal)
}

/**
 * Renders the dialog writing a new title for one item, open until the write answers.
 * @param props - The item, the write and what takes the dialog away.
 * @returns The rename dialog.
 */
export function RenamePostDialog({ post, write, onClose }: PostDialogProps) {
	const client = useQueryClient()
	return (
		<RenameDialog
			title={__('Rename', 'gophenberg')}
			name={post.title}
			fieldLabel={__('Name', 'gophenberg')}
			submitLabel={__('Save', 'gophenberg')}
			cancelLabel={__('Cancel', 'gophenberg')}
			onSubmit={(title, signal) =>
				write([post], (item) => holdWrite(client, item.id, renamed(client, item, title, signal)), nameWords(), signal)
			}
			onClose={onClose}
		/>
	)
}
