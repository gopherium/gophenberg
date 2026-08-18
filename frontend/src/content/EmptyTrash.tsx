// SPDX-License-Identifier: Apache-2.0

import { AlertDialog } from '@gophenberg/frontend-sdk'
import { __ } from '@wordpress/i18n'

import { emptyTrash } from './api'

/**
 * Renders the control removing every trashed post for good.
 * @param props - The handler reloading the listing once the trash is empty.
 * @returns The empty trash control.
 */
export function EmptyTrash({ onEmptied }: { onEmptied: () => Promise<unknown> }) {
	return (
		<AlertDialog.Root
			onConfirm={async () => {
				try {
					await emptyTrash()
				} catch {
					return { close: false, error: __('Could not empty the trash.', 'gophenberg') }
				}
				await onEmptied()
			}}
		>
			<AlertDialog.Trigger>{__('Empty Trash', 'gophenberg')}</AlertDialog.Trigger>
			<AlertDialog.Popup
				intent="irreversible"
				title={__('Empty Trash', 'gophenberg')}
				description={__('Every post in the trash is removed for good. This cannot be undone.', 'gophenberg')}
				confirmButtonText={__('Delete All', 'gophenberg')}
			/>
		</AlertDialog.Root>
	)
}
