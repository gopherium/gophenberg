// SPDX-License-Identifier: Apache-2.0

import { Button, InputControl, Stack } from '@gophenberg/frontend-sdk'
import type { RenderModalProps } from '@gophenberg/frontend-sdk/dataviews'
import { ErrorNotice, RenameBody, useToaster } from '@gopherium/godmin'
import { useMutation } from '@tanstack/react-query'
import { __, _x, sprintf } from '@wordpress/i18n'
import { useState } from 'react'

import { useRefresh } from './actions'
import { createPost, fetchPost, renamePost } from './api'
import type { Post } from './api'

/**
 * Makes a draft holding the title given and the content, excerpt, parent and field values of an item.
 * @param post - The item to copy.
 * @param title - The title of the copy.
 * @returns The copy.
 */
async function duplicated(post: Post, title: string): Promise<Post> {
	const fallback = __('The item could not be duplicated.', 'gophenberg')
	const source = await fetchPost(post.id, fallback)
	const draft = {
		title,
		content: source.content,
		excerpt: source.excerpt,
		parentId: source.parentId ?? undefined,
		fields: source.fields,
	}
	return createPost(post.type, draft, fallback)
}

/**
 * Renders the modal copying one item into a new draft under the title given, its failure kept inside.
 * @param props - The item and the handler closing the modal.
 * @returns The duplicate form.
 */
export function DuplicateModal({ items: [post], closeModal }: RenderModalProps<Post>) {
	const refresh = useRefresh()
	const toaster = useToaster()
	const [title, setTitle] = useState<string>(sprintf(_x('%s (Copy)', 'post', 'gophenberg'), post.title))
	const duplicate = useMutation({
		mutationFn: () => duplicated(post, title),
		onSuccess: async () => {
			await refresh()
			toaster.show(sprintf(__('"%s" successfully created.', 'gophenberg'), toaster.name(title)))
			closeModal?.()
		},
	})
	return (
		<form
			onSubmit={(event) => {
				event.preventDefault()
				duplicate.mutate()
			}}
		>
			<Stack direction="column" gap="lg">
				<InputControl
					label={__('Title', 'gophenberg')}
					value={title}
					onChange={(event) => setTitle(event.target.value)}
				/>
				{duplicate.error === null ? null : <ErrorNotice>{duplicate.error.message}</ErrorNotice>}
				<Stack direction="row" gap="sm" justify="flex-end">
					<Button variant="minimal" disabled={duplicate.isPending} onClick={closeModal}>
						{__('Cancel', 'gophenberg')}
					</Button>
					<Button type="submit" loading={duplicate.isPending} disabled={duplicate.isPending}>
						{_x('Duplicate', 'action label', 'gophenberg')}
					</Button>
				</Stack>
			</Stack>
		</form>
	)
}

/**
 * Renders the modal writing a new name for one item, its failure kept inside under the field.
 * @param props - The item and the handler closing the modal.
 * @returns The rename form.
 */
export function RenameModal({ items: [post], closeModal }: RenderModalProps<Post>) {
	const refresh = useRefresh()
	const toaster = useToaster()
	const rename = useMutation({
		mutationFn: (title: string) => renamePost(post.id, title),
		onSuccess: async () => {
			await refresh([post.id])
			toaster.show(__('Name updated.', 'gophenberg'))
			closeModal?.()
		},
	})
	return (
		<RenameBody
			name={post.title}
			fieldLabel={__('Name', 'gophenberg')}
			submitLabel={__('Rename', 'gophenberg')}
			cancelLabel={__('Cancel', 'gophenberg')}
			busy={rename.isPending}
			failure={rename.error?.message}
			onSubmit={(title) => rename.mutate(title)}
			onCancel={rename.isPending ? undefined : closeModal}
		/>
	)
}
