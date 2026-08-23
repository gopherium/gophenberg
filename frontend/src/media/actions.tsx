// SPDX-License-Identifier: Apache-2.0

import { Button, Notice, Stack, Text, sessionMayChange } from '@gophenberg/frontend-sdk'
import { DataForm } from '@gophenberg/frontend-sdk/dataviews'
import type { Action, RenderModalProps } from '@gophenberg/frontend-sdk/dataviews'
import { useSession } from '@gopherium/react-auth'
import { __, _n, sprintf } from '@wordpress/i18n'
import { useMutation, useQueryClient } from '@tanstack/react-query'
import { useCallback, useMemo, useState } from 'react'

import { deleteMedia, describeMedia, mediaQueryKey } from './api'
import type { MediaDescriptions, MediaItem } from './api'
import { describeForm, mediaFields } from './fields'
import { mediaName } from './format'

/**
 * Returns the handler that reloads the library.
 * @returns The reload handler.
 */
export function useRefreshMedia(): () => Promise<unknown> {
	const client = useQueryClient()
	return useCallback(
		() => client.invalidateQueries({ queryKey: mediaQueryKey }),
		[client],
	)
}

/**
 * Returns the sentence reporting how a save ended.
 * @param kind - The outcome the server answered with.
 * @param message - The reason the server gave, when it gave one.
 * @returns The sentence to show, empty when the save succeeded.
 */
export function describeFailure(kind: string, message: string): string {
	if (kind === 'stale') {
		return __('This item changed while you were describing it. Reload and try again.', 'gophenberg')
	}
	return kind === 'rejected' ? message : ''
}

/**
 * Renders the form describing one media item.
 * @param props - The items acted on and the handler closing the modal.
 * @returns The describe form.
 */
function DescribeForm({ items, closeModal }: RenderModalProps<MediaItem>) {
	const target = items[0]
	const [edits, setEdits] = useState<MediaDescriptions>({})
	const [notice, setNotice] = useState('')
	const refresh = useRefreshMedia()
	const save = useMutation({
		mutationFn: () => describeMedia(target.id, edits, target.updatedAt),
		onSuccess: async (outcome) => {
			const failure = describeFailure(outcome.kind, outcome.kind === 'rejected' ? outcome.message : '')
			if (failure !== '') {
				setNotice(failure)
				return
			}
			setNotice('')
			await refresh()
			closeModal?.()
		},
		onError: () => setNotice(__('The server could not be reached, so nothing was saved.', 'gophenberg')),
	})
	return (
		<Stack direction="column" gap="md">
			<DataForm
				data={{ ...target, ...edits }}
				fields={mediaFields}
				form={describeForm}
				onChange={(changed) => setEdits((current) => ({ ...current, ...changed }))}
			/>
			{notice !== '' && (
				<Notice.Root intent="error" role="alert">
					<Notice.Description>{notice}</Notice.Description>
				</Notice.Root>
			)}
			<Stack direction="row" gap="sm" justify="flex-end">
				<Button variant="outline" onClick={closeModal}>
					{__('Cancel', 'gophenberg')}
				</Button>
				<Button loading={save.isPending} onClick={() => save.mutate()}>
					{__('Save', 'gophenberg')}
				</Button>
			</Stack>
		</Stack>
	)
}

/**
 * Returns the question asked before media is deleted for good.
 * @param items - The items about to be deleted.
 * @returns The question to show.
 */
export function deleteQuestion(items: MediaItem[]): string {
	if (items.length === 1) {
		return sprintf(__('Delete %(title)s for good? This cannot be undone.', 'gophenberg'), {
			title: mediaName(items[0]),
		})
	}
	return sprintf(
		_n(
			'Delete this %(count)d item for good? This cannot be undone.',
			'Delete these %(count)d items for good? This cannot be undone.',
			items.length,
			'gophenberg',
		),
		{ count: items.length },
	)
}

/**
 * Returns the sentence reporting that a delete did not finish.
 * @param items - The items the delete was asked over.
 * @returns The sentence to show.
 */
export function deleteFailure(items: MediaItem[]): string {
	return _n('Could not delete that item.', 'Could not delete every item.', items.length, 'gophenberg')
}

/**
 * Renders the confirmation asked for before media is deleted for good.
 * @param props - The items acted on and the handler closing the modal.
 * @returns The confirmation body.
 */
function DeleteConfirm({ items, closeModal }: RenderModalProps<MediaItem>) {
	const refresh = useRefreshMedia()
	const remove = useMutation({
		mutationFn: async () => {
			const settled = await Promise.allSettled(items.map((item) => deleteMedia(item.id)))
			if (settled.some((outcome) => outcome.status === 'rejected')) {
				throw new Error('deleting did not finish')
			}
		},
		onSuccess: async () => {
			await refresh()
			closeModal?.()
		},
		onError: () => {
			void refresh()
		},
	})
	return (
		<Stack direction="column" gap="md">
			<Text>{deleteQuestion(items)}</Text>
			{remove.isError && (
				<Notice.Root intent="error" role="alert">
					<Notice.Description>{deleteFailure(items)}</Notice.Description>
				</Notice.Root>
			)}
			<Stack direction="row" gap="sm" justify="flex-end">
				<Button variant="outline" onClick={closeModal}>
					{__('Cancel', 'gophenberg')}
				</Button>
				<Button loading={remove.isPending} onClick={() => remove.mutate()}>
					{__('Delete Permanently', 'gophenberg')}
				</Button>
			</Stack>
		</Stack>
	)
}

/**
 * Returns the actions offered on each media item.
 * @returns The row actions.
 */
export function useMediaActions(): Action<MediaItem>[] {
	const session = useSession().data
	return useMemo(() => {
		const mine = (item: MediaItem) => sessionMayChange(session, item.authorId)
		return [
			{
				id: 'describe',
				label: __('Describe', 'gophenberg'),
				isEligible: mine,
				RenderModal: DescribeForm,
			},
			{
				id: 'delete',
				label: __('Delete Permanently', 'gophenberg'),
				supportsBulk: true,
				isEligible: mine,
				RenderModal: DeleteConfirm,
			},
		]
	}, [session])
}
