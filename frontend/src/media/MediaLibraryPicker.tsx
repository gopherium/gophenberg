// SPDX-License-Identifier: Apache-2.0

import { Button, Dialog, Stack, Text } from '@gophenberg/frontend-sdk'
import { DataViewsPicker } from '@gophenberg/frontend-sdk/dataviews'
import type { View } from '@gophenberg/frontend-sdk/dataviews'
import { Popover, SlotFillProvider } from '@gophenberg/frontend-sdk/editor'
import type { MediaLibraryProps } from '@gophenberg/frontend-sdk/editor'
import { ErrorNotice, paginationOf } from '@gopherium/godmin'
import { keepPreviousData, useQuery } from '@tanstack/react-query'
import { __ } from '@wordpress/i18n'
import { useMemo, useState } from 'react'

import { useListSettings } from '../settings/useListSettings'
import './media.css'
import { listMedia, mediaQueryKey } from './api'
import type { MediaItem, MediaPage } from './api'
import { toAttachment } from './editorMedia'
import { mediaFields, mediaListing } from './fields'

const PREVIEW_SIZE = 170

const EMPTY_PAGE: MediaPage = { items: [], total: 0, perPage: 0 }

const INITIAL_VIEW = {
	type: 'pickerGrid',
	fields: [],
	titleField: 'title',
	mediaField: 'thumbnail',
	search: '',
	page: 1,
	layout: { previewSize: PREVIEW_SIZE },
} as unknown as View

/**
 * Returns the content type prefixes a block's allowed types narrow the library to.
 * @param allowedTypes - The types the block accepts, undefined for any.
 * @returns One prefix per accepted type, empty for every kind.
 */
export function allowedMimes(allowedTypes: string[] | undefined): string[] {
	const prefixes = (allowedTypes ?? []).map((entry) => (entry.includes('/') ? entry : `${entry}/`))
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
 * Renders the media library the block editor opens to choose stored media.
 * @param props - The library contract the block editor passes.
 * @returns The trigger with the library behind it.
 */
export function MediaLibraryPicker({
	allowedTypes,
	multiple,
	value,
	onSelect,
	onClose,
	render,
}: MediaLibraryProps) {
	const settings = useListSettings().data
	const fields = useMemo(() => mediaFields(), [])
	const [open, setOpen] = useState(false)
	const [held, setHeld] = useState<View>(INITIAL_VIEW)
	const view = { ...held, perPage: held.perPage ?? settings?.picker_page_size }
	const [selection, setSelection] = useState<string[]>([])
	const takesMany = multiple === true || multiple === 'add'
	const asked = { mimes: allowedMimes(allowedTypes), ...mediaListing(view), page: view.page, perPage: view.perPage }
	const media = useQuery({
		queryKey: [...mediaQueryKey, 'picker', asked],
		queryFn: () => listMedia(asked),
		enabled: open,
		placeholderData: keepPreviousData,
	})
	const page = media.data ?? EMPTY_PAGE
	/**
	 * Closes the library, forgetting what was selected in it.
	 */
	function close() {
		setOpen(false)
		setSelection([])
		onClose?.()
	}
	/**
	 * Hands the block what the picker chose and closes the library.
	 * @param chosen - The items the picker acted on.
	 */
	function choose(chosen: MediaItem[]) {
		const attachments = chosen.map(toAttachment)
		onSelect(takesMany ? attachments : attachments[0])
		close()
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
			{render({
				open: () => {
					setSelection(selectionFrom(value))
					setOpen(true)
				},
			})}
			<Dialog.Root open={open} onOpenChange={() => close()}>
				<Dialog.Popup size="large" className="gophenberg-media-picker">
					<SlotFillProvider>
						<Dialog.Header>
							<Dialog.Title>{__('Media Library', 'gophenberg')}</Dialog.Title>
							<Dialog.CloseIconButton />
						</Dialog.Header>
						<Dialog.Content>
							{media.isError ? (
								<ErrorNotice>{__('The media library could not be loaded.', 'gophenberg')}</ErrorNotice>
							) : (
								<DataViewsPicker
									data={page.items}
									fields={fields}
									actions={actions}
									view={view}
									onChangeView={setHeld}
									selection={selection}
									onChangeSelection={setSelection}
									isLoading={media.isFetching}
									getItemId={(item: MediaItem) => String(item.id)}
									searchLabel={__('Search media', 'gophenberg')}
									config={{ perPageSizes: [] }}
									paginationInfo={paginationOf({ total: page.total, limit: page.perPage })}
									defaultLayouts={
										{
											pickerGrid: { fields: [] },
											pickerTable: { fields: ['filename', 'filesize'] },
										} as unknown as Record<string, object>
									}
									empty={<Text>{__('No media has been uploaded yet.', 'gophenberg')}</Text>}
								/>
							)}
						</Dialog.Content>
						<Dialog.Footer>
							<Stack direction="row" gap="sm" justify="flex-end">
								<Button variant="outline" onClick={close}>
									{__('Cancel', 'gophenberg')}
								</Button>
							</Stack>
						</Dialog.Footer>
						<Popover.Slot />
					</SlotFillProvider>
				</Dialog.Popup>
			</Dialog.Root>
		</>
	)
}
