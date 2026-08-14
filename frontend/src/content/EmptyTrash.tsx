// SPDX-License-Identifier: Apache-2.0

import { AlertDialog } from '@gophenberg/frontend-sdk'

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
					return { close: false, error: 'Could not empty the trash.' }
				}
				await onEmptied()
			}}
		>
			<AlertDialog.Trigger>Empty Trash</AlertDialog.Trigger>
			<AlertDialog.Popup
				intent="irreversible"
				title="Empty Trash"
				description="Every post in the trash is removed for good. This cannot be undone."
				confirmButtonText="Delete All"
			/>
		</AlertDialog.Root>
	)
}
