// SPDX-License-Identifier: Apache-2.0

import { AlertDialog } from '@gophenberg/frontend-sdk'
import { __ } from '@wordpress/i18n'

import { emptyTrash } from './api'

/**
 * Renders the control removing every trashed post of a type for good.
 * @param props - The type whose trash to empty, and the handler reloading the listing, given the posts removed.
 * @returns The empty trash control.
 */
export function EmptyTrash({ type, onEmptied }: { type: string, onEmptied: (removed: string[]) => Promise<unknown> }) {
	return (
		<AlertDialog.Root
			onConfirm={async () => {
				const emptied = await emptyTrash(type)
				await onEmptied(emptied.removed)
				if (!emptied.finished) {
					return { close: false, error: __('Could not empty the trash.', 'gophenberg') }
				}
			}}
		>
			<AlertDialog.Trigger>{__('Empty Trash', 'gophenberg')}</AlertDialog.Trigger>
			<AlertDialog.Popup
				intent="irreversible"
				title={__('Empty Trash', 'gophenberg')}
				description={__('Every item in the trash is removed for good. This cannot be undone.', 'gophenberg')}
				confirmButtonText={__('Delete All', 'gophenberg')}
			/>
		</AlertDialog.Root>
	)
}
