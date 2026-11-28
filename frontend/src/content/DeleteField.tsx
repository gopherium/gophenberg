// SPDX-License-Identifier: Apache-2.0

import { ConfirmDialog } from '@gopherium/godmin'
import { __, sprintf } from '@wordpress/i18n'
import { useMutation } from '@tanstack/react-query'

import { unlessTimedOut } from '../dialogAnswer'
import { irreversible } from '../lists/itemWords'
import { deleteFieldInGroup, deleteSubField } from './groups'
import type { Asked, Inside } from './fieldControls'
import type { ContentField } from './types'

/**
 * Returns the name of the control removing a field.
 * @param field - The field the control removes.
 * @returns The name the control reads out.
 */
export function deleteName(field: ContentField): string {
	return sprintf(__('Delete %(field)s', 'gophenberg'), { field: field.label })
}

/**
 * Returns the words of the red confirm removing a field, saying the unsaved changes inside it go too while any is held.
 * @param field - The field the confirm removes.
 * @param held - Whether the field or a field inside it holds unsaved changes.
 * @returns The words.
 */
function deleteWords(field: ContentField, held: boolean): string {
	const warning = __(
		'Every value stored under this field goes with it, in every item the group reaches and in the revisions behind them.',
		'gophenberg',
	)
	if (!held) {
		return `${warning} ${irreversible()}`
	}
	const lost = sprintf(__('Unsaved changes inside %(field)s are lost too.', 'gophenberg'), { field: field.label })
	return `${warning} ${lost} ${irreversible()}`
}

/** What the red confirm removing a field takes beyond the field. */
interface DeleteAsked extends Pick<Asked, 'onClose'> {
	/** Whether the field or a field inside it holds unsaved changes. */
	held: boolean
	/** Runs once the delete landed, before the toast saying so. */
	onDeleted: () => void
}

/**
 * Renders the red confirm that removes a field with its values.
 * @param props - The group, the field, whether a draft is held inside it, what to report, and what takes the confirm
 * away.
 * @returns The confirm, busy until the delete ends.
 */
export function DeleteConfirm(props: Inside & DeleteAsked) {
	const remove = useMutation({
		mutationFn: (signal: AbortSignal) =>
			props.path === undefined
				? deleteFieldInGroup(props.group, props.field.key, signal)
				: deleteSubField(props.group, props.path, signal),
	})
	const said = sprintf(__('%(field)s deleted.', 'gophenberg'), { field: props.field.label })
	return (
		<ConfirmDialog
			intent="irreversible"
			title={sprintf(__('Delete "%(field)s"?', 'gophenberg'), { field: props.field.label })}
			description={deleteWords(props.field, props.held)}
			confirmLabel={__('Delete the field', 'gophenberg')}
			cancelLabel={__('Cancel', 'gophenberg')}
			onConfirm={(signal) =>
				remove.mutateAsync(signal).then(
					() => {
						props.onDeleted()
						void props.onDone(said)
					},
					(cause: unknown) => unlessTimedOut(signal, cause, () => props.onRefused(cause, 'delete')),
				)
			}
			onClose={props.onClose}
		/>
	)
}
