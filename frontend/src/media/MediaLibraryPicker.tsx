// SPDX-License-Identifier: Apache-2.0

import { Dialog } from '@gophenberg/frontend-sdk'
import type { Field, View, ViewPickerGrid } from '@gophenberg/frontend-sdk/dataviews'
import { SlotFillProvider } from '@gophenberg/frontend-sdk/editor'
import type { MediaLibraryProps } from '@gophenberg/frontend-sdk/editor'
import { offeredSize, viewNarrows } from '@gopherium/godmin'
import { keepPreviousData, useQuery } from '@tanstack/react-query'
import { __ } from '@wordpress/i18n'
import { useMemo, useRef, useState } from 'react'

import { useListSettings } from '../settings/useListSettings'
import './media.css'
import { listMedia, mediaQueryKey } from './api'
import type { MediaItem } from './api'
import { toAttachment, typeFamily } from './editorMedia'
import { MEDIA_SORT, mediaFields, mediaListing } from './fields'
import { PickerDialog } from './PickerDialog'

const PREVIEW_SIZE = 170

const INITIAL_VIEW: ViewPickerGrid = {
	type: 'pickerGrid',
	fields: [],
	showTitle: false,
	titleField: 'title',
	mediaField: 'thumbnail',
	search: '',
	page: 1,
	filters: [],
	sort: { ...MEDIA_SORT },
	layout: { previewSize: PREVIEW_SIZE, density: 'compact', mediaFit: 'contain' },
}

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
 * Returns the fields the picker lists, whose thumbnail a reader cannot hide.
 * @returns The shared fields, the thumbnail not hideable.
 */
function pickerFields(): Field<MediaItem>[] {
	return mediaFields().map((field) => (field.id === 'thumbnail' ? { ...field, enableHiding: false } : field))
}

/**
 * Holds whether the picker is open, the view it shows and what is ticked in it.
 * @param value - The ids the block holds, ticked on each open.
 * @param onClose - Told when the picker closes.
 * @returns The state and the handlers opening and closing the picker.
 */
function usePickerState(value: MediaLibraryProps['value'], onClose: MediaLibraryProps['onClose']) {
	const [isOpen, setIsOpen] = useState(false)
	const [held, setHeld] = useState<View>(INITIAL_VIEW)
	const [selection, setSelection] = useState<string[]>([])
	/**
	 * Opens the picker on what the block holds, at the first page with no search.
	 */
	function open() {
		setHeld((view) => ({ ...view, page: 1, search: '' }))
		setSelection(selectionFrom(value))
		setIsOpen(true)
	}
	/**
	 * Closes the picker, forgetting what was ticked in it.
	 */
	function close() {
		setIsOpen(false)
		setSelection([])
		onClose?.()
	}
	return { isOpen, held, setHeld, selection, setSelection, open, close }
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
	/**
	 * Hands the block what the picker chose and closes the dialog.
	 * @param chosen - The items the picker acted on.
	 */
	function choose(chosen: MediaItem[]) {
		const attachments = chosen.map(toAttachment)
		onSelect(takesMany ? attachments : attachments[0])
		state.close()
	}
	const actions = [
		{
			id: 'select',
			label: __('Select', 'gophenberg'),
			isPrimary: true,
			supportsBulk: takesMany,
			callback: choose,
		},
	]
	return (
		<>
			{render({ open: state.open })}
			<Dialog.Root open={state.isOpen} onOpenChange={state.close}>
				<Dialog.Popup ref={frame} initialFocus={frame} size="full" className="gophenberg-media-picker">
					<SlotFillProvider>
						<PickerDialog
							title={title ?? __('Select Media', 'gophenberg')}
							media={media}
							view={view}
							onChangeView={state.setHeld}
							fields={fields}
							actions={actions}
							selection={state.selection}
							onChangeSelection={state.setSelection}
							sizes={sizes}
							narrowed={viewNarrows(view) || allowedMimes(allowedTypes).length > 0}
						/>
					</SlotFillProvider>
				</Dialog.Popup>
			</Dialog.Root>
		</>
	)
}
