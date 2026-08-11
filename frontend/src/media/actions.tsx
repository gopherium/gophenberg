// SPDX-License-Identifier: Apache-2.0

import { Button, Notice, Stack, Text } from '@gophenberg/frontend-sdk'
import { DataForm } from '@gophenberg/frontend-sdk/dataviews'
import type { Action, RenderModalProps } from '@gophenberg/frontend-sdk/dataviews'
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
		return 'This item changed while you were describing it. Reload and try again.'
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
	const [refusal, setRefusal] = useState('')
	const refresh = useRefreshMedia()
	const save = useMutation({
		mutationFn: () => describeMedia(target.id, edits, target.updatedAt),
		onSuccess: async (outcome) => {
			const failure = describeFailure(outcome.kind, outcome.kind === 'rejected' ? outcome.message : '')
			if (failure !== '') {
				setRefusal(failure)
				return
			}
			setRefusal('')
			await refresh()
			closeModal?.()
		},
		onError: () => setRefusal('The server could not be reached, so nothing was saved.'),
	})
	return (
		<Stack direction="column" gap="md">
			<DataForm
				data={{ ...target, ...edits }}
				fields={mediaFields}
				form={describeForm}
				onChange={(changed) => setEdits((current) => ({ ...current, ...changed }))}
			/>
			{refusal !== '' && (
				<Notice.Root intent="error" role="alert">
					<Notice.Description>{refusal}</Notice.Description>
				</Notice.Root>
			)}
			<Stack direction="row" gap="sm" justify="flex-end">
				<Button variant="outline" onClick={closeModal}>
					Cancel
				</Button>
				<Button loading={save.isPending} onClick={() => save.mutate()}>
					Save
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
		return `Delete ${mediaName(items[0])} for good? This cannot be undone.`
	}
	return `Delete these ${items.length} items for good? This cannot be undone.`
}

/**
 * Returns the sentence reporting that a delete did not finish.
 * @param items - The items the delete was asked over.
 * @returns The sentence to show.
 */
export function deleteFailure(items: MediaItem[]): string {
	return items.length === 1 ? 'Could not delete that item.' : 'Could not delete every item.'
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
					Cancel
				</Button>
				<Button loading={remove.isPending} onClick={() => remove.mutate()}>
					Delete Permanently
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
	return useMemo(
		() => [
			{ id: 'describe', label: 'Describe', RenderModal: DescribeForm },
			{
				id: 'delete',
				label: 'Delete Permanently',
				supportsBulk: true,
				RenderModal: DeleteConfirm,
			},
		],
		[],
	)
}
