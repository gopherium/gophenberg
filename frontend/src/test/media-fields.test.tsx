// SPDX-License-Identifier: Apache-2.0

import { audioIcon, fileIcon, Icon, imageIcon, videoIcon } from '@gophenberg/frontend-sdk'
import type { Field, View } from '@gophenberg/frontend-sdk/dataviews'
import { fireEvent, render, screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { resetLocaleData, setLocaleData } from '@wordpress/i18n'
import type { ComponentType, ReactNode } from 'react'
import { expect, onTestFinished, test, vi } from 'vitest'

import '../media/media.css'
import type { MediaItem } from '../media/api'
import { libraryFields, MEDIA_SORT, mediaFields, mediaListing } from '../media/fields'
import { bestSubSizeFile, fileSize, imageDimensions } from '../media/format'

/**
 * Returns a media item carrying the given overrides.
 * @param overrides - The fields to set on the item.
 * @returns The item to describe.
 */
function item(overrides: Partial<MediaItem> = {}): MediaItem {
	return {
		id: 1,
		type: 'image',
		file: '2026/08/harbor.jpg',
		title: 'Harbor at dawn',
		altText: '',
		caption: '',
		description: '',
		mimeType: 'image/jpeg',
		width: 1600,
		height: 1000,
		filesize: 254000,
		sizes: {},
		authorId: '',
		authorName: '',
		authorAvatar: '',
		createdAt: '',
		updatedAt: '',
		...overrides,
	}
}

/** A picture stored at three widths besides its own. */
const SIZED = item({
	sizes: {
		thumbnail: { file: 'a-150.jpg', width: 150, height: 150, mimeType: 'image/jpeg', filesize: 1 },
		medium: { file: 'a-300.jpg', width: 300, height: 200, mimeType: 'image/jpeg', filesize: 2 },
		large: { file: 'a-1024.jpg', width: 1024, height: 640, mimeType: 'image/jpeg', filesize: 3 },
	},
})

/**
 * Returns the field of the shared list with the given id.
 * @param id - The id of the field.
 * @returns The field.
 */
function fieldOf(id: string): Field<MediaItem> {
	return mediaFields().find((field) => field.id === id) as Field<MediaItem>
}

/**
 * Renders the cell a field draws for an item, as a DataViews layout hands it the size it draws pictures at.
 * @param id - The id of the field.
 * @param held - The item the cell draws.
 * @param sizes - The width the layout draws pictures at, none outside a layout.
 * @returns The element the cell was drawn into.
 */
function renderCell(id: string, held: MediaItem, sizes?: string): HTMLElement {
	const Cell = fieldOf(id).render as ComponentType<{ item: MediaItem, config?: { sizes: string } }>
	return render(<Cell item={held} config={sizes === undefined ? undefined : { sizes }} />).container
}

/**
 * Returns the shapes an icon draws.
 * @param icon - The icon.
 * @returns The markup inside its svg.
 */
function drawnIcon(icon: typeof fileIcon): string {
	const { container, unmount } = render(<Icon icon={icon} />)
	const markup = container.querySelector('svg')?.innerHTML
	unmount()
	return markup ?? ''
}

/**
 * Asserts that a cell draws the kind icon at the 24px WordPress draws it.
 * @param cell - The cell.
 * @param icon - The icon it should draw.
 */
function expectKindIcon(cell: HTMLElement, icon: typeof fileIcon) {
	const drawn = cell.querySelector('svg.gophenberg-media__icon')

	expect(drawn?.innerHTML).toBe(drawnIcon(icon))
	expect(drawn).toHaveAttribute('width', '24')
}

/**
 * Loads one catalogue for the admin's own words until the test ends.
 * @param messages - The translations, keyed by context and message.
 */
function speaking(messages: Record<string, string[]>) {
	setLocaleData({ '': { 'plural-forms': 'nplurals=2; plural=(n != 1);' }, ...messages }, 'gophenberg')
	onTestFinished(() => resetLocaleData({}, 'gophenberg'))
}

/** The id and label of every WordPress attachment field, in the order its media picker lists them. */
const SHARED = [
	['thumbnail', 'Thumbnail'],
	['title', 'Title'],
	['altText', 'Alt text'],
	['caption', 'Caption'],
	['description', 'Description'],
	['date', 'Date added'],
	['modified', 'Date modified'],
	['author', 'Author'],
	['filename', 'File name'],
	['filesize', 'File size'],
	['dimensions', 'Dimensions'],
	['mimeType', 'File type'],
]

test('holds the WordPress attachment fields in the order its media picker lists them', () => {
	expect(mediaFields().map((field) => [field.id, field.label])).toEqual(SHARED)
})

test('adds the Type filter to the library fields alone, under the id WordPress filters media by', () => {
	expect(libraryFields().map((field) => [field.id, field.label])).toEqual([...SHARED, ['media_type', 'Type']])
	expect(mediaFields().some((field) => field.id === 'media_type')).toBe(false)
})

test('lets a reader change only the title, the alt text, the caption and the description', () => {
	const editable = libraryFields().filter((field) => field.readOnly !== true && field.type !== 'media')

	expect(editable.map((field) => field.id)).toEqual(['title', 'altText', 'caption', 'description'])
})

test('declares every field a reader writes in or reads about the stored file as text, the title alone searched', () => {
	const texts = ['title', 'altText', 'caption', 'description', 'filename', 'filesize', 'dimensions', 'mimeType']

	expect(texts.map((id) => fieldOf(id).type)).toEqual(texts.map(() => 'text'))
	expect(mediaFields().filter((field) => field.enableGlobalSearch === true).map((field) => field.id)).toEqual(['title'])
})

test('filters by the dates before or after a moment, by the author and by the kind alone', () => {
	const filtered = libraryFields().filter((field) => field.filterBy !== false)

	expect(filtered.map((field) => [field.id, field.filterBy])).toEqual([
		['date', { operators: ['before', 'after'] }],
		['modified', { operators: ['before', 'after'] }],
		['author', { operators: ['isAny', 'isNone'] }],
		['media_type', { operators: ['is'] }],
	])
})

test('reads an untitled item as an empty title under the No title placeholder, so the Edit box starts empty', () => {
	const title = fieldOf('title')

	expect(title.getValue?.({ item: item({ title: '' }) })).toBe('')
	expect(title.placeholder).toBe('No title')
})

test('draws a title as text marked with the file, (no title) when untitled, so the bold title rule applies', () => {
	expect(renderCell('title', item()).innerHTML).toBe('<span data-item-id="1">Harbor at dawn</span>')
	expect(renderCell('title', item({ title: '' })).innerHTML).toBe('<span data-item-id="1">(no title)</span>')
})

test.each([
	{ shape: 'a picture', held: item({ id: 7 }) },
	{ shape: 'a file that is no picture', held: item({ id: 7, type: 'file', mimeType: 'application/pdf' }) },
])('marks the thumbnail of $shape with the file, so the focus finds its card again', ({ held }) => {
	const cell = renderCell('thumbnail', held, '900px')

	expect(cell.querySelectorAll('[data-item-id="7"]')).toHaveLength(1)
})

test('offers alt text on a picture alone', () => {
	const alt = fieldOf('altText')

	expect(alt.isVisible?.(item({ mimeType: 'image/png' }))).toBe(true)
	expect(alt.isVisible?.(item({ type: 'file', mimeType: 'application/pdf' }))).toBe(false)
	expect(alt.isVisible?.(item({ type: 'file', mimeType: 'video/mp4' }))).toBe(false)
})

test('helps with alt text in WordPress words, the WAI decision tree opening in a new tab', () => {
	const { container } = render(<>{fieldOf('altText').description as ReactNode}</>)

	const link = screen.getByRole('link', { name: /Describe the purpose of the image\./ })
	expect(link).toHaveAttribute('href', 'https://www.w3.org/WAI/tutorials/images/decision-tree/')
	expect(link).toHaveAttribute('target', '_blank')
	expect(container).toHaveTextContent(/Leave empty if decorative\.$/)
})

test.each(['altText', 'caption', 'description', 'mimeType'])('draws a dash for an empty %s', (id) => {
	const empty = item({ altText: '', caption: '', description: '', mimeType: '' })

	expect(renderCell(id, empty)).toHaveTextContent(/^-$/)
})

test.each([
	['altText', 'Boats at sunrise'],
	['caption', 'Before the market opens'],
	['description', 'From the eastern pier'],
	['mimeType', 'image/jpeg'],
])('draws the %s a file holds', (id, held) => {
	const filled = item({
		altText: 'Boats at sunrise',
		caption: 'Before the market opens',
		description: 'From the eastern pier',
	})

	expect(renderCell(id, filled)).toHaveTextContent(held)
})

test('draws the description as bare text, as the caption, so a card cuts it with the ellipsis of every value', () => {
	const pier = 'From the eastern pier'
	expect(renderCell('description', item({ description: pier })).innerHTML).toBe(pier)
	expect(renderCell('description', item()).innerHTML).toBe('-')
	expect(renderCell('caption', item({ caption: 'Before the market opens' })).innerHTML).toBe('Before the market opens')
})

test('writes a file size in the WordPress words of a size and its unit', () => {
	speaking({ 'file size\u0004%1$s %2$s': ['%2$s: %1$s'], 'unit symbol\u0004KB': ['kio'] })

	expect(fileSize(1536)).toBe('kio: 1,5')
})

test('writes every other binary step in the words of its unit symbol', () => {
	speaking({
		'unit symbol\u0004B': ['byte'],
		'unit symbol\u0004MB': ['MiB'],
		'unit symbol\u0004GB': ['GiB'],
		'unit symbol\u0004TB': ['TiB'],
		'unit symbol\u0004PB': ['PiB'],
		'unit symbol\u0004EB': ['EiB'],
		'unit symbol\u0004ZB': ['ZiB'],
		'unit symbol\u0004YB': ['YiB'],
	})

	expect([0, 2, 3, 4, 5, 6, 7, 8].map((step) => fileSize(1024 ** step))).toEqual([
		'1 byte',
		'1 MiB',
		'1 GiB',
		'1 TiB',
		'1 PiB',
		'1 EiB',
		'1 ZiB',
		'1 YiB',
	])
})

test.each([
	{ bytes: 512, written: '512 B' },
	{ bytes: 51200, written: '50 KB' },
	{ bytes: 5 * 1024 ** 2, written: '5 MB' },
	{ bytes: 2.5 * 1024 ** 3, written: '2,5 GB' },
	{ bytes: 1024 ** 4, written: '1 TB' },
	{ bytes: 3 * 1024 ** 5, written: '3 PB' },
	{ bytes: 1024 ** 6, written: '1 EB' },
	{ bytes: 1024 ** 7, written: '1 ZB' },
	{ bytes: 2 * 1024 ** 8, written: '2 YB' },
	{ bytes: 3.14159 * 1024 ** 2, written: '3,14 MB' },
	{ bytes: 1000, written: '1.000 B' },
])('writes $written in the binary steps WordPress reports, through the site number format', ({ bytes, written }) => {
	expect(fileSize(bytes)).toBe(written)
})

test('says nothing about a size it was never told', () => {
	expect(fileSize(0)).toBe('')
	expect(fileSize(0.5)).toBe('')
})

test('shows the size of a file only once it is known', () => {
	const size = fieldOf('filesize')

	expect(size.getValue?.({ item: item() })).toBe('248,05 KB')
	expect(size.isVisible?.(item())).toBe(true)
	expect(size.getValue?.({ item: item({ filesize: 0 }) })).toBe('')
	expect(size.isVisible?.(item({ filesize: 0 }))).toBe(false)
})

test('writes the dimensions of a picture in the WordPress words of image dimensions', () => {
	speaking({ 'image dimensions\u0004%1$s × %2$s': ['%1$s by %2$s'] })

	expect(imageDimensions(item())).toBe('1600 by 1000')
})

test('shows the dimensions of a file only when it carries both sides', () => {
	const dimensions = fieldOf('dimensions')

	expect(dimensions.getValue?.({ item: item() })).toBe('1600 × 1000')
	expect(dimensions.isVisible?.(item())).toBe(true)
	expect(dimensions.getValue?.({ item: item({ height: 0 }) })).toBe('')
	expect(dimensions.isVisible?.(item({ height: 0 }))).toBe(false)
	expect(dimensions.isVisible?.(item({ width: 0 }))).toBe(false)
})

test('reads the file name of a stored file without its folders', () => {
	expect(fieldOf('filename').getValue?.({ item: item() })).toBe('harbor.jpg')
})

test('writes a file name of 15 characters whole, with no tooltip to hover', () => {
	const cell = renderCell('filename', item({ file: '2026/08/harbor-dawn.jpg' }))

	expect(cell.innerHTML).toBe('<span class="gophenberg-media__filename">harbor-dawn.jpg</span>')
})

test('keeps a longer file name whole in the page and shows it in a tooltip on hover', async () => {
	renderCell('filename', item({ file: '2026/08/harbor-at-dawn.jpg' }))
	const name = screen.getByText('harbor-at-dawn.jpg')

	await userEvent.hover(name)

	expect(name).toHaveClass('gophenberg-media__filename')
	await waitFor(() => expect(name).toHaveAttribute('data-popup-open'))
	const popup = document.querySelector('[data-base-ui-portal]') as HTMLElement
	expect(within(popup).getByText('harbor-at-dawn.jpg')).toBeInTheDocument()
})

/**
 * Returns the width the browser reads 15 characters of the page font as.
 * @returns The computed width.
 */
function fifteenCharacters(): string {
	const measure = document.createElement('span')
	measure.style.maxInlineSize = '15ch'
	document.body.append(measure)
	const width = getComputedStyle(measure).maxInlineSize
	measure.remove()
	return width
}

test('cuts a file name at 15 characters on one line inside a DataViews table, grid or list alone', () => {
	for (const layout of ['dataviews-view-table', 'dataviews-view-grid', 'dataviews-view-list']) {
		const { container } = render(
			<div className={layout}>
				<span className="gophenberg-media__filename">harbor-at-dawn.jpg</span>
			</div>,
		)
		const cut = getComputedStyle(container.querySelector('span') as HTMLElement)

		expect([cut.maxInlineSize, cut.overflow, cut.textOverflow, cut.whiteSpace]).toEqual([
			fifteenCharacters(),
			'hidden',
			'ellipsis',
			'nowrap',
		])
	}
	const { container } = render(<span className="gophenberg-media__filename">harbor-at-dawn.jpg</span>)
	expect(getComputedStyle(container.querySelector('span') as HTMLElement).whiteSpace).not.toBe('nowrap')
})

test('draws a picture from the smallest stored size at least as wide as the layout draws it', () => {
	expect(renderCell('thumbnail', SIZED, '900px').querySelector('img')).toHaveAttribute('src', '/media/a-1024.jpg')
	expect(renderCell('thumbnail', SIZED, '52px').querySelector('img')).toHaveAttribute('src', '/media/a-150.jpg')
	expect(renderCell('thumbnail', SIZED, '4000px').querySelector('img')).toHaveAttribute('src', '/media/a-1024.jpg')
})

test('draws the stored file itself when no layout names a size', () => {
	expect(renderCell('thumbnail', SIZED).querySelector('img')).toHaveAttribute('src', '/media/2026/08/harbor.jpg')
})

test('loads a picture lazily, covering its box', () => {
	const picture = renderCell('thumbnail', SIZED, '900px').querySelector('img') as HTMLImageElement

	expect(picture).toHaveAttribute('loading', 'lazy')
	expect(getComputedStyle(picture).objectFit).toBe('cover')
})

test('fades a picture in only while it loads', () => {
	const box = renderCell('thumbnail', SIZED, '900px').firstElementChild as HTMLElement

	expect(box).toHaveClass('is-loading')
	fireEvent.load(box.querySelector('img') as HTMLImageElement)

	expect(box).toHaveClass('is-loaded')
	expect(box).not.toHaveClass('is-loading')
})

test('shows a picture the browser already holds at once, with no fade', () => {
	vi.spyOn(HTMLImageElement.prototype, 'complete', 'get').mockReturnValue(true)

	const box = renderCell('thumbnail', SIZED, '900px').firstElementChild as HTMLElement
	fireEvent.load(box.querySelector('img') as HTMLImageElement)

	expect([...box.classList]).toEqual(['gophenberg-media__thumbnail'])
})

test('names a picture by its alt text, else by its title', () => {
	expect(renderCell('thumbnail', item({ altText: 'Boats at sunrise' }), '900px').querySelector('img')).toHaveAttribute(
		'alt',
		'Boats at sunrise',
	)
	expect(renderCell('thumbnail', item(), '900px').querySelector('img')).toHaveAttribute('alt', 'Harbor at dawn')
})

test.each([
	['application/pdf', 'manual.pdf', fileIcon],
	['video/mp4', 'harbor-tour.mp4', videoIcon],
	['audio/mpeg', 'interview.mp3', audioIcon],
	['application/octet-stream', 'notes.bin', fileIcon],
])('draws the kind icon of a %s file and its whole file name, no picture', (mimeType, name, icon) => {
	const cell = renderCell('thumbnail', item({ type: 'file', mimeType, file: `2026/08/${name}` }), '900px')

	expect(cell.querySelector('img')).toBeNull()
	expectKindIcon(cell, icon)
	expect(cell.querySelector('.gophenberg-media__filename-text')).toHaveTextContent(name)
})

test('draws the picture icon and the file name for a picture whose file will not load', () => {
	const cell = renderCell('thumbnail', SIZED, '900px')

	fireEvent.error(cell.querySelector('img') as HTMLImageElement)

	expect(cell.querySelector('img')).toBeNull()
	expectKindIcon(cell, imageIcon)
	expect(cell.querySelector('.gophenberg-media__filename-text')).toHaveTextContent('harbor.jpg')
})

test.each([
	['image/png', 'image'],
	['video/webm', 'video'],
	['audio/flac', 'audio'],
	['application/pdf', 'document'],
	['application/zip', 'archive'],
	['text/plain', 'file'],
])('files a %s file under the %s kind', (mimeType, kind) => {
	const field = libraryFields().find((held) => held.id === 'media_type') as Field<MediaItem>

	expect(field.getValue?.({ item: item({ mimeType }) })).toBe(kind)
})

test('offers the WordPress file type groups as the kinds, Images, Audio, Video, Documents and Archives', () => {
	const field = libraryFields().find((held) => held.id === 'media_type') as Field<MediaItem>

	expect(field.elements).toEqual([
		{ value: 'image', label: 'Images' },
		{ value: 'audio', label: 'Audio' },
		{ value: 'video', label: 'Video' },
		{ value: 'document', label: 'Documents' },
		{ value: 'archive', label: 'Archives' },
	])
})

test('reads Type in the words of the catalogue, and its Audio, Video and Archives groups in their contexts', () => {
	speaking({
		Type: ['Family'],
		'file type group\u0004Audio': ['Sound'],
		'file type group\u0004Video': ['Film'],
		'file type group\u0004Archives': ['Bundles'],
	})

	const field = libraryFields().find((held) => held.id === 'media_type') as Field<MediaItem>

	expect(field.label).toBe('Family')
	expect(field.elements?.map((element) => element.label)).toEqual(['Images', 'Sound', 'Film', 'Documents', 'Bundles'])
})

test('picks the sub-size closest above the width a grid asks for', () => {
	expect(bestSubSizeFile(SIZED, 200)).toBe('a-300.jpg')
	expect(bestSubSizeFile(SIZED, 150)).toBe('a-150.jpg')
})

test('falls back to the largest sub-size when every one is too small', () => {
	expect(bestSubSizeFile(SIZED, 4000)).toBe('a-1024.jpg')
})

test('falls back to the stored file when there are no sub-sizes', () => {
	expect(bestSubSizeFile(item(), 200)).toBe('2026/08/harbor.jpg')
})

test('asks for the search and the sort alone when a view holds no filters', () => {
	const view: View = { type: 'grid', search: 'harbor', sort: { ...MEDIA_SORT } }

	expect(mediaListing(view)).toEqual({ search: 'harbor', orderBy: 'date', order: 'desc' })
})
