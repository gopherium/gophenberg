// SPDX-License-Identifier: Apache-2.0

import { Dialog, DropZone, mediaIcon, Stack, uploadIcon, useToaster } from '@gophenberg/frontend-sdk'
import { DataViewsPicker } from '@gophenberg/frontend-sdk/dataviews'
import type { ActionButton, Field, SupportedLayouts, View } from '@gophenberg/frontend-sdk/dataviews'
import { Popover } from '@gophenberg/frontend-sdk/editor'
import { ErrorNotice, FileButton, ListEmpty, ListNoResults, paginationOf } from '@gopherium/godmin'
import type { UseQueryResult } from '@tanstack/react-query'
import { __, _x, sprintf } from '@wordpress/i18n'
import { useEffect, useRef } from 'react'
import type { ReactNode, RefObject } from 'react'

import type { MediaItem, MediaPage } from './api'
import { UploadStatus } from './UploadStatus'
import type { UploadNotice, UploadRun } from './useUploadRun'

/** The page the list draws before the first one arrives. */
const EMPTY_PAGE: MediaPage = { items: [], total: 0, perPage: 0 }

/** The one layout the picker offers, which leaves no layout to switch to. */
const LAYOUTS: SupportedLayouts = { pickerGrid: true }

/** What the head of the dialog draws, and what takes the files it is given. */
interface PickerHeaderProps {
	title: string
	accept: string | undefined
	upload: UploadRun['upload']
}

/** What a foot of the dialog draws the status from. */
interface PickerFootProps {
	uploads: UploadRun
	frame: RefObject<HTMLDivElement | null>
}

/** What the dialog draws, and what it tells the picker around it. */
interface PickerDialogProps extends PickerFootProps {
	title: string
	accept: string | undefined
	media: UseQueryResult<MediaPage>
	view: View
	onChangeView: (view: View) => void
	fields: Field<MediaItem>[]
	actions: ActionButton<MediaItem>[]
	selection: string[]
	onChangeSelection: (selection: string[]) => void
	sizes: number[] | undefined
	narrowed: boolean
	choiceNote: string
	toast: UploadNotice | null
}

/**
 * Returns the words of the box searching the picker, as the library's box reads.
 * @returns The label in the reader's language.
 */
function searchLabel(): string {
	return sprintf(__('Search %(type)s…', 'gophenberg'), {
		type: _x('Media', 'admin section', 'gophenberg').toLowerCase(),
	})
}

/**
 * Renders the head of the dialog.
 * @param props - The words of the title, what the file dialog offers and what takes the files.
 * @returns The title, then Upload media beside the close button.
 */
function PickerHeader({ title, accept, upload }: PickerHeaderProps) {
	const label = __('Upload media', 'gophenberg')
	return (
		<Dialog.Header className="gophenberg-media-picker__header">
			<Dialog.Title>{title}</Dialog.Title>
			<Stack direction="row" gap="sm" align="center">
				<FileButton
					variant="minimal"
					tone="neutral"
					size="default"
					icon={uploadIcon}
					accept={accept}
					multiple
					inputLabel={label}
					onChoose={upload}
				>
					{label}
				</FileButton>
				<Dialog.CloseIconButton />
			</Stack>
		</Dialog.Header>
	)
}

/**
 * Renders a failure inside the frame, kept off its sides.
 * @param props - The words of the failure.
 * @returns The notice in its wrapper.
 */
function PickerNotice({ children }: { children: ReactNode }) {
	return (
		<div className="gophenberg-media-picker__notice">
			<ErrorNotice>{children}</ErrorNotice>
		</div>
	)
}

/**
 * Renders what the list shows with no cards.
 * @param props - Whether a search, a filter or the block's kinds narrow the list.
 * @returns The empty state, saying nothing was found while something narrows the list.
 */
function PickerEmpty({ narrowed }: { narrowed: boolean }) {
	if (narrowed) {
		return <ListNoResults />
	}
	return <ListEmpty icon={mediaIcon} title={__('No media items yet', 'gophenberg')} />
}

/**
 * Renders the status of the uploads while any row is listed.
 * @param props - The uploads and the frame of the dialog.
 * @returns The status, nothing with no row listed.
 */
function PickerStatus({ uploads, frame }: PickerFootProps) {
	if (uploads.rows.length === 0) {
		return null
	}
	return (
		<UploadStatus
			rows={uploads.rows}
			frame={frame}
			onDismiss={uploads.dismiss}
			onPanelChange={uploads.onPanelChange}
		/>
	)
}

/**
 * Renders the foot of the list, sticking to the bottom of the dialog.
 * @param props - The uploads and the frame of the dialog.
 * @returns The status, the count, the paging and the Select button in their wrapper.
 */
function PickerFooter({ uploads, frame }: PickerFootProps) {
	const footer = 'gophenberg-media-picker__footer'
	return (
		<div className={uploads.rows.length > 0 ? `${footer} is-uploading` : footer}>
			<PickerStatus uploads={uploads} frame={frame} />
			<DataViewsPicker.Footer />
		</div>
	)
}

/**
 * Renders the foot under a list that could not be read, the one place then showing a file the check refused.
 * @param props - The uploads and the frame of the dialog.
 * @returns The status in its wrapper, nothing with no row listed.
 */
function PickerFailedFooter({ uploads, frame }: PickerFootProps) {
	if (uploads.rows.length === 0) {
		return null
	}
	return (
		<div className="gophenberg-media-picker__footer is-alone">
			<PickerStatus uploads={uploads} frame={frame} />
		</div>
	)
}

/**
 * Renders the search, the filters, the cards and the footer of the picker.
 * @param props - What the dialog draws.
 * @returns The list.
 */
function PickerList(props: PickerDialogProps) {
	const page = props.media.data ?? EMPTY_PAGE
	return (
		<DataViewsPicker
			data={page.items}
			fields={props.fields}
			actions={props.actions}
			view={props.view}
			onChangeView={props.onChangeView}
			selection={props.selection}
			onChangeSelection={props.onChangeSelection}
			isLoading={props.media.isFetching}
			getItemId={(item: MediaItem) => String(item.id)}
			itemListLabel={__('Media items', 'gophenberg')}
			config={{ perPageSizes: props.sizes ?? [], mediaFitControl: true }}
			paginationInfo={paginationOf({ total: page.total, limit: page.perPage })}
			defaultLayouts={LAYOUTS}
			empty={<PickerEmpty narrowed={props.narrowed} />}
		>
			<Stack direction="row" align="top" justify="space-between" className="dataviews__view-actions" gap="xs">
				<Stack direction="row" gap="sm" justify="start" className="dataviews__search">
					<DataViewsPicker.Search label={searchLabel()} />
					<DataViewsPicker.FiltersToggle />
				</Stack>
				<Stack direction="row" gap="xs" style={{ flexShrink: 0 }}>
					<DataViewsPicker.ViewConfig />
				</Stack>
			</Stack>
			<DataViewsPicker.FiltersToggled className="dataviews-filters__container" />
			<DataViewsPicker.Layout />
			<PickerFooter uploads={props.uploads} frame={props.frame} />
		</DataViewsPicker>
	)
}

/**
 * Renders the list with what stopped the last Select above it, or the failure of the list's read.
 * @param props - What the dialog draws.
 * @returns The body of the dialog.
 */
function PickerBody(props: PickerDialogProps) {
	if (props.media.isError) {
		return (
			<>
				<PickerNotice>{__('The media library could not be loaded.', 'gophenberg')}</PickerNotice>
				<PickerFailedFooter uploads={props.uploads} frame={props.frame} />
			</>
		)
	}
	return (
		<>
			{props.choiceNote !== '' && <PickerNotice>{props.choiceNote}</PickerNotice>}
			<PickerList {...props} />
		</>
	)
}

/**
 * Raises a waiting toast through the nearest toaster, once for each notice.
 * @param props - The notice waiting, null for none, and the handler told it was raised.
 * @returns Nothing to draw.
 */
function UploadToast({ notice, onShown }: { notice: UploadNotice | null, onShown: () => void }) {
	const toaster = useToaster()
	const raised = useRef<UploadNotice | null>(null)
	useEffect(() => {
		if (notice === null || raised.current === notice) {
			return
		}
		raised.current = notice
		toaster.show(notice.words)
		onShown()
	}, [notice, onShown, toaster])
	return null
}

/**
 * Renders what the Select Media dialog holds inside its frame.
 * @param props - What the dialog draws.
 * @returns The drop zone, the header, the body, the slot the components overlays draw in and the toast raiser.
 */
export function PickerDialog(props: PickerDialogProps) {
	return (
		<>
			<DropZone label={__('Drop files to upload', 'gophenberg')} onFilesDrop={props.uploads.upload} />
			<PickerHeader title={props.title} accept={props.accept} upload={props.uploads.upload} />
			<Dialog.Content className="gophenberg-media-picker__content">
				<PickerBody {...props} />
			</Dialog.Content>
			<Popover.Slot />
			<UploadToast notice={props.toast} onShown={props.uploads.toastShown} />
		</>
	)
}
