// SPDX-License-Identifier: Apache-2.0

import { Button, Dialog, Stack, Text } from '@gophenberg/frontend-sdk'
import { DataViewsPicker } from '@gophenberg/frontend-sdk/dataviews'
import type { View } from '@gophenberg/frontend-sdk/dataviews'
import type { MediaLibraryProps } from '@gophenberg/frontend-sdk/editor'
import { ErrorNotice } from '@gopherium/godmin'
import { useQuery } from '@tanstack/react-query'
import { useState } from 'react'

import './media.css'
import { listMedia, mediaQueryKey } from './api'
import type { MediaItem, MediaPage } from './api'
import { toAttachment } from './editorMedia'
import { mediaFields } from './fields'

const PER_PAGE = 20

const PREVIEW_SIZE = 170

const EMPTY_PAGE: MediaPage = { items: [], total: 0 }

const INITIAL_VIEW = {
	type: 'pickerGrid',
	fields: [],
	titleField: 'title',
	mediaField: 'thumbnail',
	search: '',
	page: 1,
	perPage: PER_PAGE,
	layout: { previewSize: PREVIEW_SIZE },
} as unknown as View

/**
 * Returns the kind a block's allowed types narrow the library to.
 * @param allowedTypes - The types the block accepts, undefined for any.
 * @returns The kind to ask for, empty for every kind.
 */
export function narrowedKind(allowedTypes: string[] | undefined): string {
	if (allowedTypes === undefined || allowedTypes.length === 0) {
		return ''
	}
	return allowedTypes.every((entry) => entry === 'image' || entry.startsWith('image/'))
		? 'image'
		: ''
}

/**
 * Renders the media library the block editor opens to choose stored media.
 * @param props - The library contract the block editor passes.
 * @returns The trigger with the library behind it.
 */
export function MediaLibraryPicker({
	allowedTypes,
	multiple,
	onSelect,
	onClose,
	render,
}: MediaLibraryProps) {
	const [open, setOpen] = useState(false)
	const [view, setView] = useState<View>(INITIAL_VIEW)
	const [selection, setSelection] = useState<string[]>([])
	const takesMany = multiple === true || multiple === 'add'
	const kind = narrowedKind(allowedTypes)
	const media = useQuery({
		queryKey: [...mediaQueryKey, 'picker', kind, view.search, view.page],
		queryFn: () => listMedia({ type: kind, search: view.search, page: view.page }),
		enabled: open,
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
			label: 'Select',
			isPrimary: true,
			supportsBulk: takesMany,
			callback: choose,
		},
	]
	return (
		<>
			{render({ open: () => setOpen(true) })}
			<Dialog.Root open={open} onOpenChange={() => close()}>
				<Dialog.Popup size="large" className="gophenberg-media-picker">
					<Dialog.Header>
						<Dialog.Title>Media Library</Dialog.Title>
						<Dialog.CloseIcon />
					</Dialog.Header>
					<Dialog.Content>
						{media.isError ? (
							<ErrorNotice>The media library could not be loaded.</ErrorNotice>
						) : (
							<DataViewsPicker
								data={page.items}
								fields={mediaFields}
								actions={actions}
								view={view}
								onChangeView={setView}
								selection={selection}
								onChangeSelection={setSelection}
								isLoading={media.isPending}
								getItemId={(item: MediaItem) => String(item.id)}
								searchLabel="Search media"
								config={{ perPageSizes: [PER_PAGE] }}
								paginationInfo={{
									totalItems: page.total,
									totalPages: Math.max(1, Math.ceil(page.total / PER_PAGE)),
								}}
								defaultLayouts={
									{
										pickerGrid: { fields: [] },
										pickerTable: { fields: ['filename', 'filesize'] },
									} as unknown as Record<string, object>
								}
								empty={<Text>No media has been uploaded yet.</Text>}
							/>
						)}
					</Dialog.Content>
					<Dialog.Footer>
						<Stack direction="row" gap="sm" justify="flex-end">
							<Button variant="outline" onClick={close}>
								Cancel
							</Button>
						</Stack>
					</Dialog.Footer>
				</Dialog.Popup>
			</Dialog.Root>
		</>
	)
}
