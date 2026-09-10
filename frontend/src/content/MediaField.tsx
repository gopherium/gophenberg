// SPDX-License-Identifier: Apache-2.0

import { Button, Stack, Text } from '@gophenberg/frontend-sdk'
import { useQuery } from '@tanstack/react-query'
import { __, sprintf } from '@wordpress/i18n'

import { listMediaByIDs, mediaSrc } from '../media/api'
import type { MediaItem } from '../media/api'
import { bestRendition } from '../media/format'
import { MediaLibraryPicker } from '../media/MediaLibraryPicker'
import { FieldLabel } from './FieldLabel'
import type { ContentField } from './types'

/** The width a gallery draws each file at. */
const previewWidth = 150

/**
 * Returns the media identity a stored value holds.
 * @param value - The value the buffer holds under the field key.
 * @returns The identity, or nothing when the field holds none.
 */
export function mediaHeld(value: unknown): number | undefined {
	return typeof value === 'number' ? value : undefined
}

/**
 * Returns the identities a gallery value holds.
 * @param value - The value the buffer holds under the field key.
 * @returns The identities, empty when the field holds none.
 */
export function galleryHeld(value: unknown): number[] {
	if (!Array.isArray(value)) {
		return []
	}
	return value.filter((held): held is number => typeof held === 'number')
}

/**
 * Returns the identity a picked attachment carries.
 * @param picked - What the library reported.
 * @returns The identity, or nothing when it reported none.
 */
export function pickedMedia(picked: unknown): number | null {
	if (Array.isArray(picked)) {
		return pickedMedia(picked[0])
	}
	if (typeof picked === 'object' && picked !== null && 'id' in picked) {
		const held = (picked as { id: unknown }).id
		return typeof held === 'number' ? held : null
	}
	return null
}

/**
 * Returns the identities a picked set of attachments carries, each once.
 * @param picked - What the library reported.
 * @returns The identities, none when it reported none.
 */
export function pickedMediaList(picked: unknown): number[] {
	const listed = Array.isArray(picked) ? picked : [picked]
	const ids: number[] = []
	for (const one of listed) {
		const id = pickedMedia(one)
		if (id !== null && !ids.includes(id)) {
			ids.push(id)
		}
	}
	return ids
}

/**
 * Returns the identities with one moved the given number of places, unchanged when it cannot move.
 * @param ids - The identities in their order.
 * @param id - The identity to move.
 * @param by - How many places to move it, negative towards the front.
 * @returns The identities in their new order.
 */
export function moved(ids: number[], id: number, by: number): number[] {
	const at = ids.indexOf(id)
	const to = at + by
	if (at < 0 || to < 0 || to >= ids.length) {
		return ids
	}
	const next = [...ids]
	next.splice(at, 1)
	next.splice(to, 0, id)
	return next
}

/**
 * Returns the name a gallery shows a file under, its identity when the library no longer names it.
 * @param id - The identity the gallery holds.
 * @param item - The stored file, when the library holds it.
 * @returns The name.
 */
function fileName(id: number, item: MediaItem | undefined): string {
	return item === undefined ? sprintf(__('Media %(id)d', 'gophenberg'), { id }) : item.title
}

/**
 * Renders one file a gallery holds, with its picture when it is one.
 * @param props - The identity, the stored file, and the name it is shown under.
 * @returns The preview element.
 */
function GalleryPreview(props: { id: number; item: MediaItem | undefined; name: string }) {
	if (props.item === undefined || props.item.type !== 'image') {
		return <Text>{props.name}</Text>
	}
	return (
		<>
			<img
				className="gophenberg-editor__gallery-preview"
				src={mediaSrc(bestRendition(props.item, previewWidth))}
				alt={props.item.altText === '' ? props.name : props.item.altText}
				loading="lazy"
			/>
			<Text>{props.name}</Text>
		</>
	)
}

/**
 * Renders the control filling a media field that holds many items.
 * @param props - The field, the identities held, and what to do with a change.
 * @returns The control element.
 */
export function GalleryField(props: {
	field: ContentField
	value: number[]
	onChange: (value: number[] | null) => void
}) {
	const held = useQuery({
		queryKey: ['media', 'held', props.value.join(',')],
		queryFn: () => listMediaByIDs(props.value),
	})
	const byID = new Map((held.data ?? []).map((item) => [item.id, item]))
	/**
	 * Carries the identities in a new order, or nothing when none is left.
	 * @param ids - The identities to hold.
	 */
	function carry(ids: number[]) {
		props.onChange(ids.length > 0 ? ids : null)
	}
	return (
		<Stack direction="column" gap="xs">
			<FieldLabel field={props.field} />
			<ul className="gophenberg-editor__gallery" aria-label={props.field.label}>
				{props.value.map((id, at) => {
					const name = fileName(id, byID.get(id))
					return (
						<li key={id}>
							<Stack direction="row" gap="sm">
								<GalleryPreview id={id} item={byID.get(id)} name={name} />
								<Button
									variant="outline"
									size="compact"
									disabled={at === 0}
									onClick={() => carry(moved(props.value, id, -1))}
								>
									{sprintf(__('Move %(name)s up', 'gophenberg'), { name })}
								</Button>
								<Button
									variant="outline"
									size="compact"
									disabled={at === props.value.length - 1}
									onClick={() => carry(moved(props.value, id, 1))}
								>
									{sprintf(__('Move %(name)s down', 'gophenberg'), { name })}
								</Button>
								<Button
									variant="outline"
									size="compact"
									onClick={() => carry(props.value.filter((one) => one !== id))}
								>
									{sprintf(__('Remove %(name)s', 'gophenberg'), { name })}
								</Button>
							</Stack>
						</li>
					)
				})}
			</ul>
			<MediaLibraryPicker
				multiple="add"
				onSelect={(picked) => {
					const chosen = pickedMediaList(picked).filter((id) => !props.value.includes(id))
					if (chosen.length > 0) {
						props.onChange([...props.value, ...chosen])
					}
				}}
				onClose={() => {}}
				render={({ open }) => (
					<Button variant="outline" onClick={open}>
						{sprintf(__('Add to %(field)s', 'gophenberg'), { field: props.field.label })}
					</Button>
				)}
			/>
		</Stack>
	)
}

/**
 * Renders the control pointing a media field at a stored upload.
 * @param props - The field, the identity held, and what to do with a choice.
 * @returns The control element.
 */
export function MediaField(props: {
	field: ContentField
	value: number | undefined
	onChange: (value: number | null) => void
}) {
	return (
		<Stack direction="column" gap="xs">
			<FieldLabel field={props.field} />
			<Text>
				{props.value === undefined
					? __('No media chosen', 'gophenberg')
					: sprintf(__('Media %(id)d', 'gophenberg'), { id: props.value })}
			</Text>
			<MediaLibraryPicker
				value={props.value}
				onSelect={(picked) => props.onChange(pickedMedia(picked))}
				onClose={() => {}}
				render={({ open }) => (
					<Button variant="outline" onClick={open}>
						{sprintf(__('Choose %(field)s', 'gophenberg'), { field: props.field.label })}
					</Button>
				)}
			/>
			{props.value !== undefined && (
				<Button variant="outline" onClick={() => props.onChange(null)}>
					{sprintf(__('Clear %(field)s', 'gophenberg'), { field: props.field.label })}
				</Button>
			)}
		</Stack>
	)
}
