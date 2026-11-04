// SPDX-License-Identifier: Apache-2.0

import { Dialog } from '@gophenberg/frontend-sdk'
import type { Field, View } from '@gophenberg/frontend-sdk/dataviews'
import { SlotFillProvider } from '@gophenberg/frontend-sdk/editor'
import type { MediaLibraryProps } from '@gophenberg/frontend-sdk/editor'
import { offeredSize, viewNarrows } from '@gopherium/godmin'
import { keepPreviousData, useQuery } from '@tanstack/react-query'
import { __ } from '@wordpress/i18n'
import { useMemo, useRef, useState } from 'react'

import { useListSettings } from '../settings/useListSettings'
import { AdminToaster } from '../toasts'
import './media.css'
import { listMedia, listMediaByIDs, mediaQueryKey } from './api'
import type { MediaItem } from './api'
import { toAttachment, typeFamily } from './editorMedia'
import { mediaFields, mediaListing } from './fields'
import { PickerDialog } from './PickerDialog'
import { usePickerView } from './pickerView'
import { useUploadRun } from './useUploadRun'

/**
 * Returns the content type prefixes a block's allowed types narrow the library to.
 * @param allowedTypes - The types the block accepts, undefined for any.
 * @returns One prefix per accepted type, empty for every kind.
 */
export function allowedMimes(allowedTypes: string[] = []): string[] {
	if (allowedTypes.includes('*')) {
		return []
	}
	const prefixes = allowedTypes.map(typeFamily).map((entry) => (entry.includes('/') ? entry : `${entry}/`))
	return [...new Set(prefixes)]
}

/**
 * Returns what the file dialog offers for a block's allowed types.
 * @param allowedTypes - The types the block accepts, undefined for any.
 * @returns The types as an input's accept attribute takes them, undefined to offer every file.
 */
function acceptOf(allowedTypes: string[] = ['*']): string | undefined {
	if (allowedTypes.includes('*')) {
		return undefined
	}
	return allowedTypes.map(typeFamily).map((entry) => (entry.includes('/') ? entry : `${entry}/*`)).join(',')
}

/**
 * Returns the selection a block's current value stands for.
 * @param value - The ids the block holds, undefined for none.
 * @returns One selected id string per held item.
 */
export function selectionFrom(value: number | number[] | undefined): string[] {
	if (value === undefined) {
		return []
	}
	const held = Array.isArray(value) ? value : [value]
	return held.filter((id) => typeof id === 'number').map(String)
}

/**
 * Returns the selection once files landed.
 * @param held - What is ticked.
 * @param landed - The new files the block's types take, in the order picked.
 * @param takesMany - Whether the block takes many files.
 * @returns The new files joined without repeats, or the first in place of what was ticked, which stays if none landed.
 */
function selectionAfter(held: string[], landed: string[], takesMany: boolean): string[] {
	if (landed.length === 0) {
		return held
	}
	return takesMany ? [...new Set([...held, ...landed])] : landed.slice(0, 1)
}

/**
 * Returns the fields the picker lists, whose thumbnail a reader cannot hide.
 * @returns The shared fields, the thumbnail not hideable.
 */
function pickerFields(): Field<MediaItem>[] {
	return mediaFields().map((field) => (field.id === 'thumbnail' ? { ...field, enableHiding: false } : field))
}

/**
 * Reads the ticked files, whatever page of the library they sit on.
 * @param ids - The ticked ids, in the order they were ticked.
 * @param sizes - The page sizes the settings name, the largest being one call's worth.
 * @returns The attachments, and the words saying why there is nothing to hand over, empty when there is.
 */
async function readChosen(ids: string[], sizes: readonly number[] | undefined) {
	try {
		const items = await listMediaByIDs(ids.map(Number), sizes)
		const note = items.length > 0 ? '' : __('The selected media is no longer in the library.', 'gophenberg')
		return { attachments: items.map(toAttachment), note }
	} catch {
		return { attachments: [], note: __('The media library could not be loaded.', 'gophenberg') }
	}
}

/**
 * Holds whether the picker is open, the view it shows, what is ticked in it and what stopped its last Select.
 * @param value - The ids the block holds, ticked on each open.
 * @param onClose - Told when the picker closes.
 * @returns The state and the handlers opening and closing the picker.
 */
function usePickerState(value: MediaLibraryProps['value'], onClose: MediaLibraryProps['onClose']) {
	const [isOpen, setIsOpen] = useState(false)
	const [held, setHeld] = usePickerView()
	const [selection, setSelection] = useState<string[]>([])
	const [choiceNote, setChoiceNote] = useState('')
	const sessions = useRef(0)
	/**
	 * Opens the picker on what the block holds, at the first page with no search.
	 */
	function open() {
		setHeld((view) => ({ ...view, page: 1, search: '' }))
		setSelection(selectionFrom(value))
		setIsOpen(true)
	}
	/**
	 * Closes the picker, forgetting what was ticked in it and any read still owed to it.
	 */
	function close() {
		sessions.current += 1
		setChoiceNote('')
		setIsOpen(false)
		setSelection([])
		onClose?.()
	}
	return { isOpen, held, setHeld, selection, setSelection, choiceNote, setChoiceNote, sessions, open, close }
}

/**
 * Reads the page of the library a view shows, asking nothing while the picker is closed.
 * @param view - The view the picker shows.
 * @param allowedTypes - The types the block accepts.
 * @param isOpen - Whether the picker is open.
 * @returns The page query.
 */
function usePickerPage(view: View, allowedTypes: string[] | undefined, isOpen: boolean) {
	const asked = { mimes: allowedMimes(allowedTypes), ...mediaListing(view), page: view.page, perPage: view.perPage }
	return useQuery({
		queryKey: [...mediaQueryKey, 'picker', asked],
		queryFn: () => listMedia(asked),
		enabled: isOpen,
		placeholderData: keepPreviousData,
	})
}

/**
 * Returns the Select action, handing the block every ticked file, read again, unless a close outran the read.
 * @param state - The state of the picker.
 * @param sizes - The page sizes the settings name.
 * @param props - Whether the block takes many files, and the handler taking what it chose.
 * @returns The action.
 */
function selectAction(
	state: ReturnType<typeof usePickerState>,
	sizes: readonly number[] | undefined,
	{ takesMany, onSelect }: { takesMany: boolean, onSelect: MediaLibraryProps['onSelect'] },
) {
	return {
		id: 'select',
		label: __('Select', 'gophenberg'),
		isPrimary: true,
		supportsBulk: takesMany,
		callback: async () => {
			const session = state.sessions.current
			state.setChoiceNote('')
			const chosen = await readChosen(state.selection, sizes)
			if (session !== state.sessions.current) {
				return
			}
			state.setChoiceNote(chosen.note)
			if (chosen.note === '') {
				onSelect(takesMany ? chosen.attachments : chosen.attachments[0])
				state.close()
			}
		},
	}
}

/**
 * Renders the Select Media dialog the block editor opens to choose stored media.
 * @param props - The library contract the block editor passes.
 * @returns The trigger with the dialog behind it.
 */
export function MediaLibraryPicker({
	allowedTypes,
	multiple,
	value,
	title,
	onSelect,
	onClose,
	render,
}: MediaLibraryProps) {
	const settings = useListSettings().data
	const sizes = settings?.list_page_sizes
	const fields = useMemo(() => pickerFields(), [])
	const frame = useRef<HTMLDivElement>(null)
	const state = usePickerState(value, onClose)
	const view = { ...state.held, perPage: offeredSize(state.held.perPage, sizes, settings?.picker_page_size) }
	const takesMany = Boolean(multiple)
	const media = usePickerPage(view, allowedTypes, state.isOpen)
	const uploads = useUploadRun({
		allowedTypes,
		onLanded: (landed) => state.setSelection((held) => selectionAfter(held, landed, takesMany)),
	})
	return (
		<>
			{render({ open: state.open })}
			<Dialog.Root open={state.isOpen} onOpenChange={state.close}>
				<Dialog.Popup ref={frame} initialFocus={frame} size="full" className="gophenberg-media-picker">
					<AdminToaster>
						<SlotFillProvider>
							<PickerDialog
								title={title ?? __('Select Media', 'gophenberg')}
								accept={acceptOf(allowedTypes)}
								media={media}
								view={view}
								onChangeView={state.setHeld}
								fields={fields}
								actions={[selectAction(state, sizes, { takesMany, onSelect })]}
								selection={state.selection}
								onChangeSelection={state.setSelection}
								sizes={sizes}
								narrowed={viewNarrows(view) || allowedMimes(allowedTypes).length > 0}
								choiceNote={state.choiceNote}
								uploads={uploads}
								toast={state.isOpen ? uploads.toast : null}
								frame={frame}
							/>
						</SlotFillProvider>
					</AdminToaster>
				</Dialog.Popup>
			</Dialog.Root>
		</>
	)
}
