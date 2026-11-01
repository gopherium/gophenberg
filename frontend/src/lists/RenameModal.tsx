// SPDX-License-Identifier: Apache-2.0

import type { RenderModalProps } from '@gophenberg/frontend-sdk/dataviews'
import { RenameBody, markLatest } from '@gopherium/godmin'
import type { BulkRun, UnsavedText } from '@gopherium/godmin'
import { __ } from '@wordpress/i18n'

import { oneWriteWords } from './itemWords'

/** What the send of a new name for one item needs. */
export interface RenameSend<Item> {
	/** Returns the id an item is known by. */
	idOf: (item: Item) => string
	/** Writes the new name of an item. */
	rename: (item: Item, name: string) => Promise<unknown>
	/** Runs the write over the item, the list reloading once it ended. */
	run: BulkRun<Item>
	/** The names failed writes left unsaved, offered again when the modal opens for the same item. */
	unsaved: UnsavedText
}

/** What the rename modal of a list takes beside the props DataViews hands a modal. */
interface RenameModalProps<Item> extends RenderModalProps<Item>, RenameSend<Item> {
	/** Returns the name an item carries now. */
	nameOf: (item: Item) => string
}

/**
 * Returns the send of a new name for one item, which closes the dialog at once and toasts how the write ended.
 * @param item - The item.
 * @param close - Takes the dialog away.
 * @param send - The id reader, the write, the run and the unsaved names.
 * @returns The send, given the name as typed, settling once the toast shows.
 */
export function useRenameSend<Item>(item: Item, close: (() => void) | undefined, send: RenameSend<Item>) {
	const { idOf, rename, run, unsaved } = send
	const id = idOf(item)
	return (name: string) => {
		close?.()
		unsaved.delete(id)
		const stillLatest = markLatest(unsaved, id)
		const write = () =>
			rename(item, name).catch((failure: unknown) => {
				if (stillLatest()) {
					unsaved.set(id, name)
				}
				throw failure
			})
		return run([item], write, oneWriteWords(__('Name updated.', 'gophenberg')))
	}
}

/**
 * Renders the modal writing a new name for one item, closed the moment Save is pressed, its failure in a toast.
 * @param props - The item, the handler closing the modal, the name and id readers, the write, the run and the unsaved
 *   names.
 * @returns The rename form.
 */
export function RenameModal<Item>(props: RenameModalProps<Item>) {
	const { items: [item], closeModal, nameOf } = props
	const save = useRenameSend(item, closeModal, props)
	return (
		<RenameBody
			name={props.unsaved.get(props.idOf(item)) ?? nameOf(item)}
			fieldLabel={__('Name', 'gophenberg')}
			submitLabel={__('Save', 'gophenberg')}
			cancelLabel={__('Cancel', 'gophenberg')}
			onSubmit={(name) => void save(name)}
			onCancel={closeModal}
		/>
	)
}
