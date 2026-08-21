// SPDX-License-Identifier: Apache-2.0

import { Text } from '@gophenberg/frontend-sdk'
import type { Field } from '@gophenberg/frontend-sdk/dataviews'
import { __, _x } from '@wordpress/i18n'
import { useState } from 'react'

import { mediaSrc } from './api'
import { formatDate } from '@gopherium/gottext'
import type { MediaItem } from './api'
import { bestRendition, fileSize, imageDimensions, mediaName } from './format'

// previewWidth is the width a grid tile draws a picture at.
const previewWidth = 230

/**
 * Returns the file name of a stored file.
 * @param file - The library relative path of the file.
 * @returns The name without its directories.
 */
function fileNameOf(file: string): string {
	return file.slice(file.lastIndexOf('/') + 1)
}

/**
 * Renders the picture of a media item, or a stand in naming its file.
 * @param props - The row being rendered.
 * @returns The thumbnail cell.
 */
function ThumbnailCell({ item }: { item: MediaItem }) {
	const [broken, setBroken] = useState(false)
	if (item.type !== 'image' || broken) {
		return (
			<div className="gophenberg-media__stand-in">
				<Text variant="body-sm">{fileNameOf(item.file)}</Text>
			</div>
		)
	}
	return (
		<img
			className="gophenberg-media__thumbnail"
			src={mediaSrc(bestRendition(item, previewWidth))}
			alt={item.altText === '' ? mediaName(item) : item.altText}
			loading="lazy"
			onError={() => setBroken(true)}
		/>
	)
}

/**
 * Renders the name of a media item.
 * @param props - The row being rendered.
 * @returns The title cell.
 */
function TitleCell({ item }: { item: MediaItem }) {
	return <Text>{mediaName(item)}</Text>
}

/**
 * Renders the file a media item is stored in.
 * @param props - The row being rendered.
 * @returns The file name cell.
 */
function FileNameCell({ item }: { item: MediaItem }) {
	return <Text>{fileNameOf(item.file)}</Text>
}

/**
 * Renders the content type of a media item.
 * @param props - The row being rendered.
 * @returns The file type cell.
 */
function MimeTypeCell({ item }: { item: MediaItem }) {
	return <Text>{item.mimeType}</Text>
}

/**
 * Renders the pixel dimensions of a picture.
 * @param props - The row being rendered.
 * @returns The dimensions cell.
 */
function DimensionsCell({ item }: { item: MediaItem }) {
	return <Text>{imageDimensions(item)}</Text>
}

/**
 * Renders the size of a stored file.
 * @param props - The row being rendered.
 * @returns The file size cell.
 */
function FileSizeCell({ item }: { item: MediaItem }) {
	return <Text>{fileSize(item.filesize)}</Text>
}

/**
 * Renders the day a media item was uploaded.
 * @param props - The row being rendered.
 * @returns The date cell.
 */
function DateCell({ item }: { item: MediaItem }) {
	return <Text>{formatDate(item.createdAt)}</Text>
}

export const mediaFields: Field<MediaItem>[] = [
	{
		id: 'thumbnail',
		type: 'media',
		label: __('Thumbnail', 'gophenberg'),
		render: ThumbnailCell,
		enableSorting: false,
		enableHiding: false,
	},
	{
		id: 'title',
		label: __('Title', 'gophenberg'),
		render: TitleCell,
		Edit: { control: 'text' },
		enableSorting: false,
		enableHiding: false,
	},
	{
		id: 'type',
		label: _x('Kind', 'media family', 'gophenberg'),
		elements: [
			{ value: 'image', label: __('Images', 'gophenberg') },
			{ value: 'file', label: __('Files', 'gophenberg') },
		],
		filterBy: { operators: ['is'], isPrimary: true },
		enableSorting: false,
		readOnly: true,
	},
	{
		id: 'filename',
		label: __('File name', 'gophenberg'),
		render: FileNameCell,
		enableSorting: false,
		readOnly: true,
	},
	{
		id: 'mimeType',
		label: __('File type', 'gophenberg'),
		render: MimeTypeCell,
		enableSorting: false,
		readOnly: true,
	},
	{
		id: 'dimensions',
		label: __('Dimensions', 'gophenberg'),
		render: DimensionsCell,
		enableSorting: false,
		readOnly: true,
	},
	{
		id: 'filesize',
		label: __('File size', 'gophenberg'),
		render: FileSizeCell,
		enableSorting: false,
		readOnly: true,
	},
	{
		id: 'date',
		label: __('Date added', 'gophenberg'),
		render: DateCell,
		enableSorting: false,
		readOnly: true,
	},
	{
		id: 'altText',
		label: __('Alt text', 'gophenberg'),
		Edit: { control: 'textarea', rows: 2 },
		enableSorting: false,
	},
	{
		id: 'caption',
		label: __('Caption', 'gophenberg'),
		Edit: { control: 'textarea', rows: 2 },
		enableSorting: false,
	},
	{
		id: 'description',
		label: __('Description', 'gophenberg'),
		Edit: { control: 'textarea', rows: 5 },
		enableSorting: false,
	},
]

export const describeForm = {
	fields: ['title', 'altText', 'caption', 'description'],
}
