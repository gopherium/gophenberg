// SPDX-License-Identifier: Apache-2.0

import {
	audio,
	file,
	Icon,
	image,
	Link,
	Stack,
	Text,
	Tooltip,
	video,
} from '@gophenberg/frontend-sdk'
import type { Field, View } from '@gophenberg/frontend-sdk/dataviews'
import type { ViewGuard } from '@gopherium/godmin'
import { getSettings } from '@wordpress/date'
import { __, _x } from '@wordpress/i18n'
import { useCallback, useState } from 'react'

import { authorField, authorNarrowing, namesAccounts } from '../lists/author'
import { dateNarrowing, isMoment, narrowedBy } from '../lists/narrowing'
import type { Narrowing } from '../lists/narrowing'
import { mediaSrc } from './api'
import type { MediaItem, MediaQuery } from './api'
import { bestSubSizeFile, fileSize, imageDimensions, isPicture, kindOf, kindPrefix, titleOf } from './format'

/** The longest file name a cell shows with no tooltip, as WordPress cuts one. */
const FILE_NAME_CUT = 15

/** The mark a cell shows for a value a file leaves empty, as WordPress shows one. */
const EMPTY_MARK = '-'

/** The order the library opens in, the newest file first, and the one it keeps for a sort it never offers. */
export const MEDIA_SORT = { field: 'date', direction: 'desc' } as const

/** The order the library opens in, and how its author and date filters check the value they hold. */
export const MEDIA_VIEW_GUARD: ViewGuard = {
	opening: MEDIA_SORT,
	checks: { author: namesAccounts, date: isMoment, modified: isMoment },
}

/** The icon a card draws for each WordPress file type group that has its own, keyed by the group. */
const KIND_ICONS: Record<string, typeof file | undefined> = { image, video, audio }

/** The class of a picture's box at each step of its load, a fade only while it loads. */
const LOADING_CLASSES = {
	instant: 'gophenberg-media__thumbnail',
	loading: 'gophenberg-media__thumbnail is-loading',
	loaded: 'gophenberg-media__thumbnail is-loaded',
}

/** The part of a listing request a filter of each field narrows it by, keyed by the field. */
const narrowings: Record<string, Narrowing<MediaQuery> | undefined> = {
	author: authorNarrowing,
	date: dateNarrowing('before', 'after'),
	modified: dateNarrowing('modifiedBefore', 'modifiedAfter'),
	media_type: (filter) => ({ mimes: [kindPrefix(String(filter.value))] }),
}

/**
 * Returns the file name of a stored file.
 * @param path - The library relative path of the file.
 * @returns The name without its directories.
 */
function fileNameOf(path: string): string {
	return path.slice(path.lastIndexOf('/') + 1)
}

/**
 * Returns the file a picture loads for the width a layout draws it at, as WordPress picks one.
 * @param item - The picture.
 * @param sizes - The width the layout draws pictures at, such as 900px, none outside a layout.
 * @returns The smallest stored size at least that wide, the stored file itself when no width is named.
 */
function pictureFile(item: MediaItem, sizes: string | undefined): string {
	const target = Number.parseInt(sizes ?? '', 10)
	return Number.isNaN(target) ? item.file : bestSubSizeFile(item, target)
}

/**
 * Renders a picture filling its card, faded in while it loads unless the browser already holds it.
 * @param props - The picture, the width its layout draws it at and the handler told when its file will not load.
 * @returns The picture in its box.
 */
function PictureCell({ item, sizes, onError }: { item: MediaItem, sizes: string | undefined, onError: () => void }) {
	const [loading, setLoading] = useState<keyof typeof LOADING_CLASSES>('loading')
	const held = useCallback((picture: HTMLImageElement | null) => {
		if (picture?.complete) {
			setLoading('instant')
		}
	}, [])
	return (
		<div className={LOADING_CLASSES[loading]} data-item-id={item.id}>
			<img
				ref={held}
				className="gophenberg-media__image"
				src={mediaSrc(pictureFile(item, sizes))}
				alt={item.altText === '' ? item.title : item.altText}
				loading="lazy"
				onLoad={() => setLoading((step) => (step === 'loading' ? 'loaded' : step))}
				onError={onError}
			/>
		</div>
	)
}

/**
 * Renders a file that shows no picture as the icon of its kind above its file name.
 * @param props - The file.
 * @returns The kind icon and the file name, cut on one line.
 */
function KindCell({ item }: { item: MediaItem }) {
	return (
		<div className="gophenberg-media__thumbnail" data-item-id={item.id}>
			<Stack direction="column" justify="center" align="center" className="gophenberg-media__stack">
				<Icon
					className="gophenberg-media__icon"
					icon={KIND_ICONS[kindOf(item.mimeType)] ?? file}
					size={24}
				/>
				<div className="gophenberg-media__name-box">
					<Text variant="body-sm" className="gophenberg-media__filename-text">
						{fileNameOf(item.file)}
					</Text>
				</div>
			</Stack>
		</div>
	)
}

/**
 * Renders the picture of a media item, or the icon of its kind for a file that is no picture or will not load.
 * @param props - The item and the width its layout draws pictures at.
 * @returns The thumbnail cell.
 */
function ThumbnailCell({ item, config }: { item: MediaItem, config?: { sizes: string } }) {
	const [broken, setBroken] = useState(false)
	if (broken || !isPicture(item)) {
		return <KindCell item={item} />
	}
	return <PictureCell item={item} sizes={config?.sizes} onError={() => setBroken(true)} />
}

/**
 * Renders the title of a media item as text marked with the item, so the title rule of a layout draws it and the
 * focus finds its card again.
 * @param props - The row being rendered.
 * @returns The title, or (no title).
 */
function TitleCell({ item }: { item: MediaItem }) {
	return <span data-item-id={item.id}>{titleOf(item)}</span>
}

/**
 * Renders the alt text of a picture, a dash when it holds none.
 * @param props - The row being rendered.
 * @returns The alt text cell.
 */
function AltTextCell({ item }: { item: MediaItem }) {
	return item.altText || EMPTY_MARK
}

/**
 * Renders the caption of a media item, a dash when it holds none.
 * @param props - The row being rendered.
 * @returns The caption cell.
 */
function CaptionCell({ item }: { item: MediaItem }) {
	return item.caption || EMPTY_MARK
}

/**
 * Renders the description of a media item as bare text, a dash when it holds none, as WordPress renders the caption.
 * @param props - The row being rendered.
 * @returns The description cell.
 */
function DescriptionCell({ item }: { item: MediaItem }) {
	return item.description || EMPTY_MARK
}

/**
 * Renders the content type of a media item, a dash when it carries none.
 * @param props - The row being rendered.
 * @returns The file type cell.
 */
function MimeTypeCell({ item }: { item: MediaItem }) {
	return item.mimeType || EMPTY_MARK
}

/**
 * Renders the whole file name of a media item, held in a tooltip once a layout may cut it.
 * @param props - The row being rendered.
 * @returns The file name cell.
 */
function FileNameCell({ item }: { item: MediaItem }) {
	const name = fileNameOf(item.file)
	const shown = <span className="gophenberg-media__filename">{name}</span>
	if (name.length <= FILE_NAME_CUT) {
		return shown
	}
	return (
		<Tooltip.Root>
			<Tooltip.Trigger render={shown} />
			<Tooltip.Popup>{name}</Tooltip.Popup>
		</Tooltip.Root>
	)
}

/**
 * Returns the help WordPress gives under the alt text box.
 * @returns The link to the WAI decision tree, opening in a new tab, and the note about decorative pictures.
 */
function altTextHelp() {
	return (
		<>
			<Link href={__('https://www.w3.org/WAI/tutorials/images/decision-tree/', 'gophenberg')} openInNewTab>
				{__('Describe the purpose of the image.', 'gophenberg')}
			</Link>
			<br />
			{__('Leave empty if decorative.', 'gophenberg')}
		</>
	)
}

/**
 * Returns the fields a reader writes about a file, as WordPress lists them.
 * @returns The title, the alt text, the caption and the description.
 */
function describingFields(): Field<MediaItem>[] {
	return [
		{
			id: 'title',
			type: 'text',
			label: __('Title', 'gophenberg'),
			placeholder: __('No title', 'gophenberg'),
			getValue: ({ item }) => item.title,
			render: TitleCell,
			enableGlobalSearch: true,
			filterBy: false,
		},
		{
			id: 'altText',
			type: 'text',
			label: __('Alt text', 'gophenberg'),
			description: altTextHelp(),
			Edit: { control: 'textarea', rows: 2 },
			isVisible: isPicture,
			render: AltTextCell,
			enableSorting: false,
			filterBy: false,
		},
		{
			id: 'caption',
			type: 'text',
			label: __('Caption', 'gophenberg'),
			Edit: { control: 'textarea', rows: 2 },
			render: CaptionCell,
			enableSorting: false,
			filterBy: false,
		},
		{
			id: 'description',
			type: 'text',
			label: __('Description', 'gophenberg'),
			Edit: { control: 'textarea', rows: 5 },
			render: DescriptionCell,
			enableSorting: false,
			filterBy: false,
		},
	]
}

/**
 * Returns a moment of a media item as a field, written in the full date and time format the admin booted with.
 * @param id - The id of the field.
 * @param label - The label of the field.
 * @param moment - Reads the moment from an item.
 * @returns The field, sorted and filtered before or after a moment.
 */
function momentField(id: string, label: string, moment: (item: MediaItem) => string): Field<MediaItem> {
	return {
		id,
		type: 'datetime',
		label,
		getValue: ({ item }) => moment(item),
		format: { datetime: getSettings().formats.datetime, weekStartsOn: getSettings().l10n.startOfWeek },
		filterBy: { operators: ['before', 'after'] },
		readOnly: true,
	}
}

/**
 * Returns the fields describing the stored file itself, which no reader changes.
 * @returns The file name, the file size, the dimensions and the file type.
 */
function storedFileFields(): Field<MediaItem>[] {
	return [
		{
			id: 'filename',
			type: 'text',
			label: __('File name', 'gophenberg'),
			getValue: ({ item }) => fileNameOf(item.file),
			render: FileNameCell,
			enableSorting: false,
			filterBy: false,
			readOnly: true,
		},
		{
			id: 'filesize',
			type: 'text',
			label: __('File size', 'gophenberg'),
			getValue: ({ item }) => fileSize(item.filesize),
			isVisible: (item) => item.filesize > 0,
			enableSorting: false,
			filterBy: false,
			readOnly: true,
		},
		{
			id: 'dimensions',
			type: 'text',
			label: __('Dimensions', 'gophenberg'),
			getValue: ({ item }) => imageDimensions(item),
			isVisible: (item) => item.width > 0 && item.height > 0,
			enableSorting: false,
			filterBy: false,
			readOnly: true,
		},
		{
			id: 'mimeType',
			type: 'text',
			label: __('File type', 'gophenberg'),
			render: MimeTypeCell,
			enableSorting: false,
			filterBy: false,
			readOnly: true,
		},
	]
}

/**
 * Returns the WordPress attachment fields the library and the editor's media picker list, in the picker's order.
 * @returns The thumbnail, the describing fields, both dates, the author and the stored file's fields.
 */
export function mediaFields(): Field<MediaItem>[] {
	return [
		{
			id: 'thumbnail',
			type: 'media',
			label: __('Thumbnail', 'gophenberg'),
			render: ThumbnailCell,
			enableSorting: false,
			filterBy: false,
		},
		...describingFields(),
		momentField('date', __('Date added', 'gophenberg'), (item) => item.createdAt),
		momentField('modified', __('Date modified', 'gophenberg'), (item) => item.updatedAt),
		{ ...authorField<MediaItem>(), readOnly: true },
		...storedFileFields(),
	]
}

/**
 * Returns the fields of the media library, the shared ones and the type filter WordPress's file type groups pick from.
 * @returns The shared fields, then Type.
 */
export function libraryFields(): Field<MediaItem>[] {
	return [
		...mediaFields(),
		{
			id: 'media_type',
			label: __('Type', 'gophenberg'),
			getValue: ({ item }) => kindOf(item.mimeType),
			elements: [
				{ value: 'image', label: __('Images', 'gophenberg') },
				{ value: 'audio', label: _x('Audio', 'file type group', 'gophenberg') },
				{ value: 'video', label: _x('Video', 'file type group', 'gophenberg') },
				{ value: 'document', label: __('Documents', 'gophenberg') },
				{ value: 'archive', label: _x('Archives', 'file type group', 'gophenberg') },
			],
			filterBy: { operators: ['is'] },
			enableSorting: false,
			enableHiding: false,
			readOnly: true,
		},
	]
}

/**
 * Returns the search, the sort and the filters of a media listing a view stands for, leaving its page to the caller.
 * @param view - The view the list shows.
 * @returns The part of the listing request the view decides.
 */
export function mediaListing(view: View): MediaQuery {
	return {
		search: view.search,
		orderBy: view.sort?.field,
		order: view.sort?.direction,
		...narrowedBy(view.filters ?? [], narrowings),
	}
}
