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

/**
 * Makes a draft holding the title given and the content, excerpt, parent and the field values of an item the server
 * writes, the ones its conditions show less the Linked from fields it only reads, read once every write still in
 * flight to the item has answered.
 * @param client - The query client holding the writes in flight.
 * @param post - The item to copy.
 * @param title - The title of the copy.
 * @param declared - The fields the type declares, judging which values the rules show.
 * @returns The copy.
 */
async function duplicated(client: QueryClient, post: Post, title: string, declared: ContentField[]): Promise<Post> {
	const fallback = __('The item could not be duplicated.', 'gophenberg')
	await writesSettled(client, post.id)
	const source = await fetchPost(post.id, fallback)
	const draft = {
		title,
		content: source.content,
		excerpt: source.excerpt,
		parentId: source.parentId ?? undefined,
		fields: shownValues(declared, source.fields),
	}
	return createPost(post.type, draft, fallback)
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
			onSubmit={() => write([post], () => duplicated(client, post, title, fields), made)}
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
 * @returns The renamed item.
 */
async function renamed(client: QueryClient, post: Post, title: string): Promise<Post> {
	await writesSettled(client, post.id)
	return renamePost(post.id, title)
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
			onSubmit={(title) => write([post], (item) => holdWrite(client, item.id, renamed(client, item, title)), nameWords())}
			onClose={onClose}
		/>
	)
}
