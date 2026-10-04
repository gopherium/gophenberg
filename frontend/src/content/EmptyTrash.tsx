// SPDX-License-Identifier: Apache-2.0

import { AlertDialog } from '@gophenberg/frontend-sdk'
import { __, sprintf } from '@wordpress/i18n'

import { emptyTrash } from './api'

/**
 * Renders the control removing every trashed post of a type for good.
 * @param props - The type whose trash to empty, its plural label, and the handler reloading the listing.
 * @returns The empty trash control.
 */
export function EmptyTrash({
	type,
	label,
	onEmptied,
}: {
	type: string
	label: string
	onEmptied: (removed: string[]) => Promise<unknown>
}) {
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
				description={sprintf(
					__('Every item in the %(type)s trash is removed for good. This cannot be undone.', 'gophenberg'),
					{ type: label.toLowerCase() },
				)}
				confirmButtonText={__('Delete All', 'gophenberg')}
			/>
		</AlertDialog.Root>
	)
}
