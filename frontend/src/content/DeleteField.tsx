// SPDX-License-Identifier: Apache-2.0

import { ConfirmDialog } from '@gopherium/godmin'
import { __, sprintf } from '@wordpress/i18n'
import { useMutation } from '@tanstack/react-query'

import { unlessTimedOut } from '../dialogAnswer'
import { irreversible } from '../lists/itemWords'
import { deleteFieldInGroup, deleteSubField } from './groups'
import { DialogButton } from './fieldControls'
import type { Inside } from './fieldControls'

/**
 * Renders the control removing a field and, while it asks, the red confirm that removes it with its values.
 * @param props - The group, the field, and what to report.
 * @returns The control and its confirm, busy until the delete ends.
 */
export function DeleteField(props: Inside) {
	const opener = sprintf(__('Delete %(field)s', 'gophenberg'), { field: props.field.label })
	const warning = __(
		'Every value stored under this field goes with it, in every item the group reaches and in the revisions behind them.',
		'gophenberg',
	)
	const remove = useMutation({
		mutationFn: (signal: AbortSignal) =>
			props.path === undefined
				? deleteFieldInGroup(props.group, props.field.key, signal)
				: deleteSubField(props.group, props.path, signal),
	})
	const said = sprintf(__('%(field)s deleted.', 'gophenberg'), { field: props.field.label })
	return (
		<DialogButton
			words={__('Delete', 'gophenberg')}
			title={opener}
			dialog={(asked) => (
				<ConfirmDialog
					intent="irreversible"
					title={sprintf(__('Delete "%(field)s"?', 'gophenberg'), { field: props.field.label })}
					description={`${warning} ${irreversible()}`}
					confirmLabel={__('Delete the field', 'gophenberg')}
					cancelLabel={__('Cancel', 'gophenberg')}
					onConfirm={(signal) =>
						remove.mutateAsync(signal).then(
							() => {
								void props.onDone(said)
							},
							(cause: unknown) => unlessTimedOut(signal, cause, () => props.onRefused(cause, 'delete')),
						)
					}
					onClose={asked.onClose}
				/>
			)}
		/>
	)
}
