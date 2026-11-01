// SPDX-License-Identifier: Apache-2.0

import { sessionMayChange, trashIcon } from '@gophenberg/frontend-sdk'
import type { Action } from '@gophenberg/frontend-sdk/dataviews'
import { useBulkRun, useUnsavedText } from '@gopherium/godmin'
import type { ListRun } from '@gopherium/godmin'
import { useSession } from '@gopherium/react-auth'
import { useQueryClient } from '@tanstack/react-query'
import { __, _x } from '@wordpress/i18n'
import { useCallback, useMemo } from 'react'

import { mediaQueryKey, mediaSrc } from './api'
import type { MediaDescriptions, MediaItem } from './api'
import { displayFile } from './format'
import { DeleteMediaModal } from './MediaModals'
import type { MediaTools, OpenedDialog } from './MediaModals'

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
 * Returns the check telling whether the session may change a file.
 * @returns The check, true on its own uploads and on every file for a role that may change the work of others.
 */
export function useMayChange(): (item: MediaItem) => boolean {
	const session = useSession().data
	return useCallback((item: MediaItem) => sessionMayChange(session, item.authorId), [session])
}

/**
 * Returns what the actions of the library need from the screen, the same for as long as the screen stays.
 * @param list - What the list does around a run.
 * @returns The tools.
 */
export function useMediaTools(list: ListRun): MediaTools {
	const client = useQueryClient()
	const run = useBulkRun<MediaItem>(list, useRefreshMedia())
	const edits = useUnsavedText<MediaDescriptions>()
	const renames = useUnsavedText()
	return useMemo(() => ({ client, run, edits, renames }), [client, edits, renames, run])
}

/**
 * Opens a file in a new tab, as View does.
 * @param item - The file.
 */
export function viewFile(item: MediaItem) {
	window.open(mediaSrc(displayFile(item)), '_blank')
}

/**
 * Returns the action opening a file in a new tab, a primary one click action offered on every file.
 * @returns View.
 */
function viewAction(): Action<MediaItem> {
	return {
		id: 'view',
		label: _x('View', 'verb', 'gophenberg'),
		isPrimary: true,
		callback: ([item]) => viewFile(item),
	}
}

/**
 * Returns the actions offered on each file, View on every file and the others on a file the session may change.
 * @param tools - What the actions need from the screen.
 * @param mayChange - Whether the session may change a file.
 * @param open - Opens Edit or Rename on a file in a dialog the screen holds, which a card moving to another row of the
 *   grid never takes away.
 * @returns The row actions, the same for as long as the screen stays, so a confirm never starts over in a run.
 */
export function useMediaActions(
	tools: MediaTools,
	mayChange: (item: MediaItem) => boolean,
	open: (opened: OpenedDialog) => void,
): Action<MediaItem>[] {
	return useMemo(
		() => [
			viewAction(),
			{
				id: 'edit',
				label: __('Edit…', 'gophenberg'),
				isEligible: mayChange,
				callback: ([item]) => open({ action: 'edit', item, trigger: 'actions' }),
			},
			{
				id: 'rename',
				label: __('Rename…', 'gophenberg'),
				isEligible: mayChange,
				callback: ([item]) => open({ action: 'rename', item, trigger: 'actions' }),
			},
			{
				id: 'delete',
				label: __('Permanently delete…', 'gophenberg'),
				icon: trashIcon,
				supportsBulk: true,
				isEligible: mayChange,
				hideModalHeader: true,
				modalFocusOnMount: 'firstContentElement',
				RenderModal: (props) => <DeleteMediaModal {...props} tools={tools} />,
			},
		],
		[mayChange, open, tools],
	)
}
