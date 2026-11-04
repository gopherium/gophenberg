// SPDX-License-Identifier: Apache-2.0

import { Dialog, mediaIcon, Stack } from '@gophenberg/frontend-sdk'
import { DataViewsPicker } from '@gophenberg/frontend-sdk/dataviews'
import type { ActionButton, Field, SupportedLayouts, View } from '@gophenberg/frontend-sdk/dataviews'
import { Popover } from '@gophenberg/frontend-sdk/editor'
import { ErrorNotice, ListEmpty, ListNoResults, paginationOf } from '@gopherium/godmin'
import type { UseQueryResult } from '@tanstack/react-query'
import { __, _x, sprintf } from '@wordpress/i18n'
import type { ReactNode } from 'react'

import type { MediaItem, MediaPage } from './api'

/** The page the list draws before the first one arrives. */
const EMPTY_PAGE: MediaPage = { items: [], total: 0, perPage: 0 }

/** The one layout the picker offers, which leaves no layout to switch to. */
const LAYOUTS: SupportedLayouts = { pickerGrid: true }

/** What the dialog draws, and what it tells the picker around it. */
interface PickerDialogProps {
	title: string
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
 * @param props - The words of the title.
 * @returns The title and the close button.
 */
function PickerHeader({ title }: { title: string }) {
	return (
		<Dialog.Header className="gophenberg-media-picker__header">
			<Dialog.Title>{title}</Dialog.Title>
			<Dialog.CloseIconButton />
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
 * Renders the foot of the list, sticking to the bottom of the dialog.
 * @returns The count, the paging and the Select button in their wrapper.
 */
function PickerFooter() {
	return (
		<div className="gophenberg-media-picker__footer">
			<DataViewsPicker.Footer />
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
			<PickerFooter />
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
		return <PickerNotice>{__('The media library could not be loaded.', 'gophenberg')}</PickerNotice>
	}
	return (
		<>
			{props.choiceNote !== '' && <PickerNotice>{props.choiceNote}</PickerNotice>}
			<PickerList {...props} />
		</>
	)
}

/**
 * Renders what the Select Media dialog holds inside its frame.
 * @param props - What the dialog draws.
 * @returns The header, the body and the slot the components overlays draw in.
 */
export function PickerDialog(props: PickerDialogProps) {
	return (
		<>
			<PickerHeader title={props.title} />
			<Dialog.Content className="gophenberg-media-picker__content">
				<PickerBody {...props} />
			</Dialog.Content>
			<Popover.Slot />
		</>
	)
}
